package handlers

import (
	"context"
	"strconv"
	"time"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/yi-nology/zentao-max/backend/pkg/errors"
	"github.com/yi-nology/zentao-max/backend/internal/service"
	"github.com/yi-nology/zentao-max/backend/internal/infra/zentao"
)

// DailyNoteHandler daily note HTTP 处理器。
//
// 路由：
//   GET  /api/v1/effort/notes?user={account}&year={YYYY}&month={MM}
//        拉指定用户某月所有 note；user 留空取当前用户；year/month 留空返全部。
//   GET  /api/v1/effort/notes/{user}/{date}
//        拉单条 note；不存在返 200 + 空 content。
//   PUT  /api/v1/effort/notes/{user}/{date}
//        upsert 单条 note。body: {"content": "..."}。
//   DELETE /api/v1/effort/notes/{user}/{date}
//        删一条 note。
//
// 业务定位：跟禅道 effort 互补 — effort 是 task 维度 ("X 任务 Y 小时")，
// note 是日维度总结 ("今天整体感受 / 会议 / 明日计划")。
//
// 写时优先同步到禅道：
//   - 给当前 user 自动建一个 special "📓 Daily Notes" task（产品/项目/执行=0）
//   - daily note 作为该 task 的 effort.work 字段
//   - zentao web 端"我的工时"页 / task 详情页能直接看到
// 失败降级：zentao 不可达 / 限流时仅写本地 SQLite（不丢数据）。
type DailyNoteHandler struct {
	store   *zentao.DailyNoteStore
	service *service.DailyNoteService
}

// NewDailyNoteHandler 构造 handler。store 可为 nil（构造失败时），路由层会判 nil。
// service 可选：传 nil 时 handler 退化到直接走 store（不走 zentao 同步）。
func NewDailyNoteHandler(store *zentao.DailyNoteStore, svc *service.DailyNoteService) *DailyNoteHandler {
	return &DailyNoteHandler{store: store, service: svc}
}

// ListNotes GET /api/v1/effort/notes?user=&year=&month=
func (h *DailyNoteHandler) ListNotes(ctx context.Context, c *app.RequestContext) {
	if h.store == nil {
		errors.InternalError(c, "daily note store not initialized")
		return
	}
	user := string(c.QueryArgs().Peek("user"))
	yearStr := string(c.QueryArgs().Peek("year"))
	monthStr := string(c.QueryArgs().Peek("month"))
	year, _ := strconv.Atoi(yearStr)
	month, _ := strconv.Atoi(monthStr)

	// 优先 service（可结合 zentao + 本地），降级到 store
	var notes []*zentao.DailyNote
	var err error
	if h.service != nil {
		notes, err = h.service.ListNotes(ctx, user, year, month)
	} else {
		notes, err = h.store.ListNotesByMonth(ctx, user, year, month)
	}
	if err != nil {
		errors.Error(c, errors.ExternalError("daily note", err))
		return
	}
	if notes == nil {
		notes = []*zentao.DailyNote{}
	}
	errors.Success(c, map[string]interface{}{
		"notes": notes,
		"user":  user,
		"year":  year,
		"month": month,
	})
}

// GetNote GET /api/v1/effort/notes/{user}/{date}
func (h *DailyNoteHandler) GetNote(ctx context.Context, c *app.RequestContext) {
	if h.store == nil {
		errors.InternalError(c, "daily note store not initialized")
		return
	}
	user := c.Param("user")
	date := c.Param("date")
	if !isValidNoteDate(date) {
		errors.BadRequest(c, "date 格式错误，应为 YYYY-MM-DD")
		return
	}
	var note *zentao.DailyNote
	var err error
	if h.service != nil {
		note, err = h.service.GetNote(ctx, user, date)
	} else {
		note, err = h.store.GetNote(ctx, user, date)
	}
	if err != nil {
		errors.Error(c, errors.ExternalError("daily note", err))
		return
	}
	if note == nil {
		errors.Success(c, map[string]interface{}{
			"user":    user,
			"date":    date,
			"content": "",
			"exists":  false,
		})
		return
	}
	errors.Success(c, map[string]interface{}{
		"user":      note.User,
		"date":      note.Date,
		"content":   note.Content,
		"exists":    true,
		"createdAt": note.CreatedAt.Format(time.RFC3339),
		"updatedAt": note.UpdatedAt.Format(time.RFC3339),
	})
}

// PutNote PUT /api/v1/effort/notes/{user}/{date}
func (h *DailyNoteHandler) PutNote(ctx context.Context, c *app.RequestContext) {
	if h.store == nil {
		errors.InternalError(c, "daily note store not initialized")
		return
	}
	user := c.Param("user")
	date := c.Param("date")
	if !isValidNoteDate(date) {
		errors.BadRequest(c, "date 格式错误，应为 YYYY-MM-DD")
		return
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := c.BindAndValidate(&body); err != nil {
		errors.BadRequest(c, "body 格式错误，应为 {\"content\": \"...\"}")
		return
	}

	// service 路径：写禅道 + 写本地（zentao 失败时降级）
	if h.service != nil {
		res, err := h.service.PutNote(ctx, user, date, body.Content)
		if err != nil {
			errors.Error(c, errors.ExternalError("daily note", err))
			return
		}
		errors.Success(c, map[string]interface{}{
			"user":         res.Note.User,
			"date":         res.Note.Date,
			"content":      res.Note.Content,
			"zentaoSynced": res.ZentaoSynced,
			"taskId":       res.TaskID,
			"message":      res.Message,
			"createdAt":    res.Note.CreatedAt.Format(time.RFC3339),
			"updatedAt":    res.Note.UpdatedAt.Format(time.RFC3339),
		})
		return
	}

	// 降级路径：纯本地
	note, err := h.store.SetNote(ctx, user, date, body.Content, 0)
	if err != nil {
		errors.Error(c, errors.ExternalError("daily note", err))
		return
	}
	errors.Success(c, map[string]interface{}{
		"user":         note.User,
		"date":         note.Date,
		"content":      note.Content,
		"zentaoSynced": false,
		"createdAt":    note.CreatedAt.Format(time.RFC3339),
		"updatedAt":    note.UpdatedAt.Format(time.RFC3339),
	})
}

// DeleteNote DELETE /api/v1/effort/notes/{user}/{date}
func (h *DailyNoteHandler) DeleteNote(ctx context.Context, c *app.RequestContext) {
	if h.store == nil {
		errors.InternalError(c, "daily note store not initialized")
		return
	}
	user := c.Param("user")
	date := c.Param("date")
	if !isValidNoteDate(date) {
		errors.BadRequest(c, "date 格式错误，应为 YYYY-MM-DD")
		return
	}
	var err error
	if h.service != nil {
		err = h.service.DeleteNote(ctx, user, date)
	} else {
		err = h.store.DeleteNote(ctx, user, date)
	}
	if err != nil {
		errors.Error(c, errors.ExternalError("daily note", err))
		return
	}
	errors.Success(c, map[string]interface{}{
		"user":    user,
		"date":    date,
		"deleted": true,
	})
}

// isValidNoteDate 简单校验 YYYY-MM-DD 格式（不严格到合法日期，只校验 10 字符 + 两位 dash）。
func isValidNoteDate(s string) bool {
	if len(s) != 10 {
		return false
	}
	if s[4] != '-' || s[7] != '-' {
		return false
	}
	for i, c := range s {
		if i == 4 || i == 7 {
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

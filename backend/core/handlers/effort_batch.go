package handlers

import (
	"context"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/yi-nology/zentao-max/backend/core/errors"
	"github.com/yi-nology/zentao-max/backend/core/zentao"
)

// EffortBatchHandler 工时批量创建 HTTP 处理器。
//
// 路由：POST /api/v1/efforts/batch
// body: {"date": "2026-07-16", "items": [{"work": "...", "objectType": "task_617525",
//                                          "execution": 9151, "consumed": 1, "left": 10}, ...]}
//
// 后端走 /effort-batchCreate-{date}.html?onlybody=yes 端点（受限 session mode），
// service.BatchCreateEffort 负责拼 form 格式（id[]/work[]/objectType[]/execution[N]/consumed[]/left[N]）。
// 缓存：写成功后通过 doSessionPost 的 writeOpToEntity 自动失效 sessionEntityEfforts，
// 下次 effort-calendar 拉新数据。
type EffortBatchHandler struct {
	client *zentao.Client
}

// NewEffortBatchHandler 构造 handler。
func NewEffortBatchHandler(client *zentao.Client) *EffortBatchHandler {
	return &EffortBatchHandler{client: client}
}

// EffortBatchItem 前端提交的 batch 单条记录。
type EffortBatchItem struct {
	Work       string  `json:"work"`
	ObjectType string  `json:"objectType"` // "task_617525" 或 "bug_xxx" 或空（占位行）
	Execution  int     `json:"execution"`
	Consumed   float64 `json:"consumed"`
	Left       float64 `json:"left"`
}

// EffortBatchRequest 批量创建请求 body。
type EffortBatchRequest struct {
	Date  string            `json:"date"`  // YYYY-MM-DD
	Items []EffortBatchItem `json:"items"` // 多条
}

// Create POST /api/v1/efforts/batch
func (h *EffortBatchHandler) Create(ctx context.Context, c *app.RequestContext) {
	if h.client == nil {
		errors.InternalError(c, "zentao client not initialized")
		return
	}

	var req EffortBatchRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "body 格式错误，应为 {\"date\":\"2026-07-16\",\"items\":[...]}")
		return
	}
	if req.Date == "" {
		errors.BadRequest(c, "date 不能为空，应为 YYYY-MM-DD")
		return
	}
	if len(req.Items) == 0 {
		errors.BadRequest(c, "items 不能为空")
		return
	}

	// 过滤空行 + 转换为 client.BatchCreateEffortItem
	clientItems := make([]zentao.BatchCreateEffortItem, 0, len(req.Items))
	skipped := 0
	for _, item := range req.Items {
		// 空行（work/objectType/execution 全空）跳过
		if strings.TrimSpace(item.Work) == "" && item.ObjectType == "" && item.Execution == 0 && item.Consumed == 0 {
			skipped++
			continue
		}
		// objectType 必须有值（受限实例需要 task_xxx 或 bug_xxx 才能定位）
		if item.ObjectType == "" {
			errors.BadRequest(c, "objectType 不能为空（应为 task_xxx 或 bug_xxx 形式）")
			return
		}
		if item.Execution == 0 {
			errors.BadRequest(c, "execution 不能为空")
			return
		}
		if item.Consumed < 0 || item.Left < 0 {
			errors.BadRequest(c, "consumed 和 left 不能为负数")
			return
		}
		clientItems = append(clientItems, zentao.BatchCreateEffortItem{
			Work:       item.Work,
			ObjectType: item.ObjectType,
			Execution:  item.Execution,
			Consumed:   item.Consumed,
			Left:       item.Left,
		})
	}
	if len(clientItems) == 0 {
		errors.BadRequest(c, "至少需要一条有效记录")
		return
	}

	if err := h.client.BatchCreateEffort(ctx, req.Date, clientItems); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}

	errors.Success(c, map[string]interface{}{
		"date":    req.Date,
		"created": len(clientItems),
		"skipped": skipped,
	})
}

package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	myzentao "github.com/yi-nology/zentao-max/backend/core/zentao"
)

// DailyNoteService 业务层包装：daily note 写禅道（effort 表）+ 本地 SQLite 兜底。
//
// 设计动机：daily note 是用户私人的"今天我干了什么"，归用户自己所有。
// 写到禅道 → 用户在 zentao web 端"我的工时"页能直接看到 work 字段。
// 写本地 SQLite → 禅道不可达时 fallback + 离线可用。
//
// 同步模型：
//   - 写：1) 找/建 special "📓 Daily Notes" task;2) RecordEffort(work=content);3) 本地存
//   - 读：本地 SQLite（zentao effort 拉取需要查 task + 翻页，复杂；本服务读直接走本地）
//   - 删：本地删 + zentao 端不删 effort（zentao effort 没有 delete API；后续可加"置空"逻辑）
//
// thread-safe：Service 自身有 mu 锁 special task_id 的查找/创建；DailyNoteStore 内部也有锁。
type DailyNoteService struct {
	store *myzentao.DailyNoteStore
	client *myzentao.Client

	// 进程内 special task_id 缓存，避免每个请求都查 zentao。
	taskMu    sync.Mutex
	taskCache map[string]int // account → zentao task_id
}

// NewDailyNoteService 构造 service。store 可为 nil（仅纯本地模式）；client 可为 nil（zentao 同步禁用）。
func NewDailyNoteService(store *myzentao.DailyNoteStore, client *myzentao.Client) *DailyNoteService {
	return &DailyNoteService{
		store:     store,
		client:    client,
		taskCache: make(map[string]int),
	}
}

// DailyNoteResult 是 PutNote 的返回（包含 zentao 同步状态）。
type DailyNoteResult struct {
	Note     *myzentao.DailyNote
	ZentaoSynced bool   // true = 同步到禅道 effort 成功；false = 仅本地
	TaskID   int    // zentao special task_id（0 = 未同步到禅道）
	Message  string // 给前端的提示文本
}

// PutNote 写一条 daily note：先尝试禅道 RecordEffort，再写本地 SQLite。
//
// 失败降级：zentao 写失败时（限流/网络/权限），仍写本地 SQLite，ZentaoSynced=false。
// 这样用户写的内容不丢，离线 / zentao 不可达时仍能用。
func (s *DailyNoteService) PutNote(ctx context.Context, user, date, content string) (*DailyNoteResult, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("daily note service not initialized")
	}
	if user == "" || date == "" {
		return nil, errors.New("user and date are required")
	}

	// 1) 找/建 special task（zentao 端）。负缓存（-1）表示该用户此前查找/创建
	//    失败（受限实例常不支持 execution=0 建任务），本进程内快速跳过，
	//    避免每次保存备注都慢速重试失败。
	var zentaoTaskID int
	var zentaoErr error
	zentaoSynced := false
	if s.client != nil {
		if cached, ok := func() (int, bool) {
			s.taskMu.Lock()
			defer s.taskMu.Unlock()
			v, ok := s.taskCache[user]
			return v, ok
		}(); ok && cached == -1 {
			zentaoErr = errSpecialTaskUnavailable
		} else {
			zentaoTaskID, zentaoErr = s.findOrCreateDailyNoteTask(ctx, user)
		}
		if zentaoErr == nil && zentaoTaskID > 0 {
			// 2) RecordEffort 到这个 task
			// 禅道 work 字段存 daily note 内容，consumed=0.01 让 zentao 接受（禅道拒绝 0）
			if err := s.client.RecordEffort(zentaoTaskID, date, 0.01, 0.01, content); err != nil {
				log.Printf("[DailyNoteService] zentao RecordEffort 失败: user=%s date=%s task=%d err=%v (本地仍写入)",
					user, date, zentaoTaskID, err)
			} else {
				zentaoSynced = true
			}
		} else {
			log.Printf("[DailyNoteService] zentao 找/建 special task 失败: user=%s err=%v (本地仍写入)", user, zentaoErr)
		}
	}

	// 3) 本地存（必成功，否则返错给前端）
	note, err := s.store.SetNote(ctx, user, date, content, 0)
	if err != nil {
		return nil, fmt.Errorf("本地存储失败: %w", err)
	}

	msg := "已保存"
	if !zentaoSynced && s.client != nil {
		msg = "已保存到本地（禅道同步失败，详情看 server log）"
	}
	return &DailyNoteResult{
		Note:        note,
		ZentaoSynced: zentaoSynced,
		TaskID:      zentaoTaskID,
		Message:     msg,
	}, nil
}

// DeleteNote 删一条 daily note：本地删 + 通知 zentao 不动（effort 没 delete API）。
func (s *DailyNoteService) DeleteNote(ctx context.Context, user, date string) error {
	if s == nil || s.store == nil {
		return errors.New("daily note service not initialized")
	}
	return s.store.DeleteNote(ctx, user, date)
}

// ListNotes 拉某月所有 daily note。
func (s *DailyNoteService) ListNotes(ctx context.Context, user string, year, month int) ([]*myzentao.DailyNote, error) {
	if s == nil || s.store == nil {
		return nil, nil
	}
	return s.store.ListNotesByMonth(ctx, user, year, month)
}

// GetNote 拉单条 daily note。
func (s *DailyNoteService) GetNote(ctx context.Context, user, date string) (*myzentao.DailyNote, error) {
	if s == nil || s.store == nil {
		return nil, nil
	}
	return s.store.GetNote(ctx, user, date)
}

// errSpecialTaskUnavailable 表示 special task 在当前实例不可用（已负缓存）。
var errSpecialTaskUnavailable = errors.New("special task 在当前实例不可用（已跳过重试）")

// findOrCreateDailyNoteTask 找当前 user 的 special "📓 Daily Notes" task。
// 查找顺序：进程内 cache → 本地 SQLite → zentao ListMyTasks 按 name 匹配 → 都没则创建。
func (s *DailyNoteService) findOrCreateDailyNoteTask(ctx context.Context, user string) (int, error) {
	const specialName = "📓 Daily Notes"

	// 1) 进程内 cache
	s.taskMu.Lock()
	if id, ok := s.taskCache[user]; ok && id > 0 {
		s.taskMu.Unlock()
		return id, nil
	}
	s.taskMu.Unlock()

	// 2) 本地 SQLite
	if id, name, err := s.store.GetDailyNoteTaskID(ctx, user); err == nil && id > 0 {
		s.taskMu.Lock()
		s.taskCache[user] = id
		s.taskMu.Unlock()
		return id, nil
	} else if name != "" {
		// 本地有记录但 task_id=0（极端情况），落到下一步查 zentao
		_ = name
	}

	// 3) zentao 查找（biz8.x 受限实例：/my-task-{status}.json 返回空，
	//    改用 /my-work-task.json（禅道首页"待处理"数据源，实测可用）；
	//    特殊任务若已关闭则不在列表中，走下一步创建）
	tasks, err := s.client.GetMyWorkTasks(ctx)
	if err == nil {
		for _, t := range tasks {
			if t.Name == specialName {
				// 找到，存本地 + 缓存
				_ = s.store.SaveDailyNoteTask(ctx, user, specialName, t.ID)
				s.taskMu.Lock()
				s.taskCache[user] = t.ID
				s.taskMu.Unlock()
				return t.ID, nil
			}
		}
	} else {
		log.Printf("[DailyNoteService] GetMyWorkTasks err: user=%s err=%v", user, err)
	}

	// 4) 都没就创建（execution=0；受限实例若拒绝创建会在此报错，
	//    调用方降级为仅本地保存，前端可显式把日志挂到真实任务）
	if err := s.client.CreateTaskNoExecution(specialName, "misc", user, "系统为 daily note 同步自动创建，请勿删除"); err != nil {
		// 负缓存：本进程内不再对该用户重试（每次保存都慢速失败影响体验）。
		// 显式挂真实任务的路径（前端"同步为工时"）不受影响。
		s.taskMu.Lock()
		s.taskCache[user] = -1
		s.taskMu.Unlock()
		return 0, fmt.Errorf("创建 special task 失败（实例可能要求任务必须挂执行）: %w", err)
	}
	// 创完再查
	tasks, err = s.client.GetMyWorkTasks(ctx)
	if err != nil {
		return 0, fmt.Errorf("创建后查询失败: %w", err)
	}
	for _, t := range tasks {
		if t.Name == specialName {
			_ = s.store.SaveDailyNoteTask(ctx, user, specialName, t.ID)
			s.taskMu.Lock()
			s.taskCache[user] = t.ID
			s.taskMu.Unlock()
			return t.ID, nil
		}
	}
	return 0, errors.New("创建 special task 后仍找不到")
}

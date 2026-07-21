package handlers

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/yi-nology/zentao-max/backend/core/errors"
	"github.com/yi-nology/zentao-max/backend/core/logger"
)

// LogHandler 日志查看接口（供前端日志页消费）
type LogHandler struct{}

func NewLogHandler() *LogHandler {
	return &LogHandler{}
}

type logsQuery struct {
	Level   string `form:"level"`
	Keyword string `form:"q"`
	Limit   int    `form:"limit"`
}

// GetLogs GET /api/logs?level=info&q=keyword&limit=200
// 返回最近的日志（最新在前），支持按级别和关键字过滤
func (h *LogHandler) GetLogs(ctx context.Context, c *app.RequestContext) {
	q := logsQuery{Limit: 200}
	if err := c.BindAndValidate(&q); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if q.Limit <= 0 || q.Limit > 2000 {
		q.Limit = 200
	}

	entries := logger.GetRingBuffer().Query(q.Level, q.Keyword, q.Limit)
	if entries == nil {
		entries = []*logger.LogEntry{}
	}
	errors.Success(c, map[string]interface{}{
		"entries":     entries,
		"total":       len(entries),
		"buffer_size": logger.GetRingBuffer().Size(),
	})
}

// ClearLogs DELETE /api/logs - 清空日志缓冲
func (h *LogHandler) ClearLogs(ctx context.Context, c *app.RequestContext) {
	logger.GetRingBuffer().Clear()
	errors.Success(c, map[string]interface{}{
		"cleared": true,
	})
}

// LogsStatus GET /api/logs/status - 返回日志缓冲大小
func (h *LogHandler) LogsStatus(ctx context.Context, c *app.RequestContext) {
	errors.Success(c, map[string]interface{}{
		"buffer_size": logger.GetRingBuffer().Size(),
		"max_size":    1000,
	})
}

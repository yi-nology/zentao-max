package handlers

import (
	"context"
	"strconv"
	"time"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/yi-nology/zentao-max/backend/core/errors"
	"github.com/yi-nology/zentao-max/backend/core/service"
)

// EffortCalendarHandler 工时日历 HTTP 处理器。
//
// 路由：GET /api/v1/effort-calendar?user={account}&year={YYYY}
// 路由：GET /api/effort-calendar?user={account}&year={YYYY}
//
// 业务行为：
//   - 不传 user 时取当前登录用户（前端最常用）。
//   - 不传 year 时默认当前年。
//   - Session 模式：直接走 service.EffortCalendarService（已带 sessionCache）。
//   - Token 模式（标准禅道）：service 内部走 501 + 友好错误（本期未实现）。
type EffortCalendarHandler struct {
	servicer service.EffortCalendarServicer
}

// NewEffortCalendarHandler 构造 handler。
func NewEffortCalendarHandler(s service.EffortCalendarServicer) *EffortCalendarHandler {
	return &EffortCalendarHandler{servicer: s}
}

// GetEffortCalendar GET /api/v1/effort-calendar?user=&year=
func (h *EffortCalendarHandler) GetEffortCalendar(ctx context.Context, c *app.RequestContext) {
	user := string(c.QueryArgs().Peek("user"))
	yearStr := string(c.QueryArgs().Peek("year"))

	year := time.Now().Year()
	if yearStr != "" {
		v, err := strconv.Atoi(yearStr)
		if err != nil || v < 2000 || v > 2100 {
			errors.BadRequest(c, "year 格式错误，应为 2000-2100 之间的整数")
			return
		}
		year = v
	}

	result, err := h.servicer.GetEffortCalendar(user, year)
	if err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.Success(c, result)
}

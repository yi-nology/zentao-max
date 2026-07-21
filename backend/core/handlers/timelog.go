package handlers

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/yi-nology/zentao-max/backend/core/dto"
	"github.com/yi-nology/zentao-max/backend/core/errors"
)

type TimelogHandler struct {
	timelogService TimelogServicer
}

func NewTimelogHandler(timelogService TimelogServicer) *TimelogHandler {
	return &TimelogHandler{timelogService: timelogService}
}

func (h *TimelogHandler) GetTimelogAnalysis(ctx context.Context, c *app.RequestContext) {
	var query dto.TimelogQueryDTO
	if err := c.BindAndValidate(&query); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}

	// productId 改为可选：受限 session 模式下 user 工时不一定关联 product，
	// product 过滤会吞掉数据。空 productId 走 user 维度 fallback（service 层）
	if query.DateFrom == "" || query.DateTo == "" {
		errors.BadRequest(c, "dateFrom and dateTo are required")
		return
	}

	result, err := h.timelogService.GetTimelogAnalysis(&query)
	if err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}

	errors.Success(c, result)
}

func (h *TimelogHandler) GetTimelogDashboard(ctx context.Context, c *app.RequestContext) {
	var query dto.TimelogQueryDTO
	if err := c.BindAndValidate(&query); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}

	// productId 可选（受限 session mode fallback）


	if query.DateFrom == "" || query.DateTo == "" {
		errors.BadRequest(c, "dateFrom and dateTo are required")
		return
	}

	result, err := h.timelogService.GetTimelogDashboard(&query)
	if err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}

	errors.Success(c, result)
}

func (h *TimelogHandler) GetTimelogEfforts(ctx context.Context, c *app.RequestContext) {
	var query dto.TimelogQueryDTO
	if err := c.BindAndValidate(&query); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}

	// productId 可选（受限 session mode fallback）

	if query.DateFrom == "" || query.DateTo == "" {
		errors.BadRequest(c, "dateFrom and dateTo are required")
		return
	}

	result, err := h.timelogService.GetTimelogEfforts(&query)
	if err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}

	errors.Success(c, result)
}

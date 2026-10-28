package handlers

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/yi-nology/zentao-max/backend/internal/dto"
	"github.com/yi-nology/zentao-max/backend/pkg/errors"
)

type DashboardHandler struct {
	dashboardService DashboardServicer
}

func NewDashboardHandler(ds DashboardServicer) *DashboardHandler {
	return &DashboardHandler{dashboardService: ds}
}

func (h *DashboardHandler) GetDashboard(ctx context.Context, c *app.RequestContext) {
	var query dto.DashboardQuery
	if err := c.BindAndValidate(&query); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if err := query.Validate(); err != nil {
		errors.BadRequest(c, err.Error())
		return
	}
	result, err := h.dashboardService.GetDashboardContext(ctx, query.ProductID, query.StartDate, query.EndDate)
	if err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	// 如果数据来自缓存（离线或命中），通过响应头告知前端
	if result != nil && result.FromCache {
		c.Response.Header.Set("X-Cache", "HIT")
		if result.Stale {
			c.Response.Header.Set("X-Cache-Stale", "1")
		}
	} else {
		c.Response.Header.Set("X-Cache", "MISS")
	}
	errors.Success(c, result)
}

func (h *DashboardHandler) GetProjectOverview(ctx context.Context, c *app.RequestContext) {
	var query dto.ProjectOverviewQuery
	if err := c.BindAndValidate(&query); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	result, err := h.dashboardService.GetProjectOverviewContext(ctx, query.ProjectID)
	if err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.Success(c, result)
}

func (h *DashboardHandler) GetPersonalTimelog(ctx context.Context, c *app.RequestContext) {
	var query dto.PersonalTimelogQuery
	if err := c.BindAndValidate(&query); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	result, err := h.dashboardService.GetPersonalTimelogContext(ctx, query.Account, query.ProductID, query.DateFrom, query.DateTo, query.GroupBy)
	if err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.Success(c, result)
}

func (h *DashboardHandler) Search(ctx context.Context, c *app.RequestContext) {
	var query dto.SearchQuery
	if err := c.BindAndValidate(&query); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if err := query.Validate(); err != nil {
		errors.Error(c, err)
		return
	}
	result, err := h.dashboardService.SearchContext(ctx, query.Keyword, query.ProductID, query.Page, query.PageSize)
	if err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.Success(c, result)
}

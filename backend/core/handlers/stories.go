package handlers

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/yi-nology/zentao-max/backend/core/dto"
	"github.com/yi-nology/zentao-max/backend/core/errors"
	"github.com/yi-nology/zentao-max/backend/core/service"
)

type StoryHandler struct {
	storyService StoryServicer
}

func NewStoryHandler(storyService StoryServicer) *StoryHandler {
	return &StoryHandler{storyService: storyService}
}

func (h *StoryHandler) GetStories(ctx context.Context, c *app.RequestContext) {
	var query dto.StoryQueryDTO
	if err := c.BindAndValidate(&query); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}

	if err := query.Validate(); err != nil {
		errors.Error(c, err)
		return
	}

	result, err := h.storyService.GetStories(&query)
	if err != nil {
		if _, ok := err.(*service.ValidationError); ok {
			errors.BadRequest(c, err.Error())
			return
		}
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}

	// 统计卡需要全量（分页前）的各状态数量，随列表一并返回
	statusCounts, err := h.storyService.GetStoryStatusCounts(&query)
	if err != nil {
		if _, ok := err.(*service.ValidationError); ok {
			errors.BadRequest(c, err.Error())
			return
		}
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}

	errors.Success(c, map[string]interface{}{
		"list":         result.List,
		"total":        result.Total,
		"page":         result.Page,
		"pageSize":     result.PageSize,
		"statusCounts": statusCounts,
	})
}

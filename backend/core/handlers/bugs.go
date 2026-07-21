package handlers

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/yi-nology/zentao-max/backend/core/dto"
	"github.com/yi-nology/zentao-max/backend/core/errors"
)

type BugHandler struct {
	bugService BugServicer
}

func NewBugHandler(bugService BugServicer) *BugHandler {
	return &BugHandler{bugService: bugService}
}

func (h *BugHandler) GetBugs(ctx context.Context, c *app.RequestContext) {
	var query dto.BugQueryDTO
	if err := c.BindAndValidate(&query); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}

	if err := query.Validate(); err != nil {
		errors.Error(c, err)
		return
	}

	result, err := h.bugService.GetBugs(&query)
	if err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}

	errors.Success(c, result)
}

package handlers

import (
	"context"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/yi-nology/zentao-max/backend/pkg/errors"
)

type BuildHandler struct {
	buildService BuildServicer
}

func NewBuildHandler(buildService BuildServicer) *BuildHandler {
	return &BuildHandler{buildService: buildService}
}

func (h *BuildHandler) GetBuildsByProject(ctx context.Context, c *app.RequestContext) {
	projectID, err := strconv.Atoi(c.Query("projectId"))
	if err != nil || projectID <= 0 {
		errors.BadRequest(c, "请提供有效的项目ID")
		return
	}

	result, err := h.buildService.GetBuildsByProject(projectID)
	if err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}

	errors.Success(c, result)
}

func (h *BuildHandler) GetBuildsByExecution(ctx context.Context, c *app.RequestContext) {
	executionID, err := strconv.Atoi(c.Query("executionId"))
	if err != nil || executionID <= 0 {
		errors.BadRequest(c, "请提供有效的执行ID")
		return
	}

	result, err := h.buildService.GetBuildsByExecution(executionID)
	if err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}

	errors.Success(c, result)
}

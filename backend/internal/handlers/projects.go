package handlers

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/yi-nology/zentao-max/backend/internal/dto"
	"github.com/yi-nology/zentao-max/backend/pkg/errors"
)

type ProjectHandler struct {
	projectService ProjectServicer
}

func NewProjectHandler(projectService ProjectServicer) *ProjectHandler {
	return &ProjectHandler{projectService: projectService}
}

func (h *ProjectHandler) GetProjects(ctx context.Context, c *app.RequestContext) {
	var query dto.ProjectQueryDTO
	if err := c.BindAndValidate(&query); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}

	_ = query.Validate()

	result, err := h.projectService.GetProjects(&query)
	if err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}

	errors.Success(c, result)
}

type ExecutionHandler struct {
	executionService ExecutionServicer
}

func NewExecutionHandler(executionService ExecutionServicer) *ExecutionHandler {
	return &ExecutionHandler{executionService: executionService}
}

func (h *ExecutionHandler) GetExecutions(ctx context.Context, c *app.RequestContext) {
	var query dto.ExecutionQueryDTO
	if err := c.BindAndValidate(&query); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}

	_ = query.Validate()

	result, err := h.executionService.GetExecutions(&query)
	if err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}

	errors.Success(c, result)
}

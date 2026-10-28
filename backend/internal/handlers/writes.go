package handlers

import (
	"context"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"

	zentaosdk "github.com/yi-nology/zentao-max/backend/internal/infra/zentao/sdk"
	"github.com/yi-nology/zentao-max/backend/pkg/errors"
	myzentao "github.com/yi-nology/zentao-max/backend/internal/infra/zentao"
)

// WriteHandler 暴露 bug/task/story 的写操作（Phase2c）。
// 之前 wrapper 是纯只读的，这里为前端提供 resolve/close/assign/start/finish 等入口。
// 所有方法同时支持 token 和 session 两种认证模式（分发在 wrapper 层完成）。
type WriteHandler struct{ client *myzentao.Client }

func NewWriteHandler(client *myzentao.Client) *WriteHandler {
	return &WriteHandler{client: client}
}

// pathID 从路径参数 :id 提取整数 ID。
func pathID(c *app.RequestContext, key string) (int, error) {
	v := c.Param(key)
	n, err := strconv.Atoi(string(v))
	if err != nil {
		return 0, errors.New(errors.CodeInvalidParam, "无效的 "+key)
	}
	return n, nil
}

// ---- Bug ----

func (h *WriteHandler) ResolveBug(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	var req zentaosdk.BugResolveRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if err := h.client.ResolveBug(id, req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "Bug 已解决", nil)
}

func (h *WriteHandler) CloseBug(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	var req zentaosdk.BugCloseRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if err := h.client.CloseBug(id, req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "Bug 已关闭", nil)
}

func (h *WriteHandler) AssignBug(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	var req zentaosdk.BugAssignRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if err := h.client.AssignBug(id, req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "Bug 已指派", nil)
}

func (h *WriteHandler) ConfirmBug(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	var req zentaosdk.BugConfirmRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if err := h.client.ConfirmBug(id, req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "Bug 已确认", nil)
}

func (h *WriteHandler) ActivateBug(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	var req zentaosdk.BugActivateRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if err := h.client.ActivateBug(id, req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "Bug 已激活", nil)
}

// ---- Task ----

func (h *WriteHandler) StartTask(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	var req zentaosdk.TaskStartRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if err := h.client.StartTask(id, req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "任务已开始", nil)
}

func (h *WriteHandler) FinishTask(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	var req zentaosdk.TaskFinishRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if err := h.client.FinishTask(id, req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "任务已完成", nil)
}

func (h *WriteHandler) PauseTask(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	var req zentaosdk.TaskPauseRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if err := h.client.PauseTask(id, req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "任务已暂停", nil)
}

func (h *WriteHandler) AssignTask(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	var req zentaosdk.TaskAssignRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if err := h.client.AssignTask(id, req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "任务已指派", nil)
}

// ---- Story ----

func (h *WriteHandler) ChangeStory(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	var req struct {
		Spec   string `json:"spec"`
		Verify string `json:"verify"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if err := h.client.ChangeStory(id, req.Spec, req.Verify); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "需求已变更", nil)
}

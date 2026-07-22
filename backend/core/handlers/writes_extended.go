package handlers

import (
	"context"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"

	zentaosdk "github.com/yi-nology/zentao-max/backend/core/zentao/sdk"
	"github.com/yi-nology/zentao-max/backend/core/errors"
)

// 本文件为 WriteHandler 补齐剩余写操作端点（Phase2c 扩展）。
// 覆盖 bug/task/story 的 create/update/delete、task activate、effort record、
// plan create/update/delete/link、case/product/project/program/
// execution/build/user 的 create/update/delete。
//
// 设计：所有 create/update 用 JSON body 绑参到 SDK request 类型，
// delete 只需路径 :id。session 模式下 wrapper 不返回新建对象，handler 统一返回
// {code:0, message:"...成功"}（不返回 data 对象）。

// queryInt 从 query 取整数（productID/executionID 等父ID）。
func queryInt(c *app.RequestContext, key string) int {
	v := c.Query(key)
	n, _ := strconv.Atoi(string(v))
	return n
}

// ---- Bug create/update/delete ----

func (h *WriteHandler) CreateBug(ctx context.Context, c *app.RequestContext) {
	productID := queryInt(c, "productId")
	if productID == 0 {
		errors.BadRequest(c, "缺少 productId")
		return
	}
	var req zentaosdk.BugCreateRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if _, err := h.client.CreateBug(productID, req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "Bug 已创建", nil)
}

func (h *WriteHandler) UpdateBug(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	var req zentaosdk.BugUpdateRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if _, err := h.client.UpdateBug(id, req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "Bug 已更新", nil)
}

func (h *WriteHandler) DeleteBug(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	if err := h.client.DeleteBug(id); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "Bug 已删除", nil)
}

// ---- Task create/update/delete/activate ----

func (h *WriteHandler) CreateTask(ctx context.Context, c *app.RequestContext) {
	executionID := queryInt(c, "executionId")
	if executionID == 0 {
		errors.BadRequest(c, "缺少 executionId")
		return
	}
	var req zentaosdk.TaskCreateRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if _, err := h.client.CreateTask(executionID, req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "任务已创建", nil)
}

func (h *WriteHandler) UpdateTask(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	var req zentaosdk.TaskUpdateRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if _, err := h.client.UpdateTask(id, req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "任务已更新", nil)
}

func (h *WriteHandler) DeleteTask(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	if err := h.client.DeleteTask(id); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "任务已删除", nil)
}

func (h *WriteHandler) ActivateTask(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	var req struct {
		Consumed float64 `json:"consumed"`
		Left     float64 `json:"left"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if err := h.client.ActivateTask(id, req.Consumed, req.Left); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "任务已激活", nil)
}

// ---- Story create/update/delete ----

func (h *WriteHandler) CreateStory(ctx context.Context, c *app.RequestContext) {
	var req zentaosdk.StoryCreateRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if _, err := h.client.CreateStory(req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "需求已创建", nil)
}

func (h *WriteHandler) UpdateStory(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	var req zentaosdk.StoryUpdateRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if _, err := h.client.UpdateStory(id, req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "需求已更新", nil)
}

func (h *WriteHandler) DeleteStory(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	if err := h.client.DeleteStory(id); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "需求已删除", nil)
}

// ---- Effort ----

func (h *WriteHandler) RecordEffort(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	var req struct {
		Date     string  `json:"date"`
		Consumed float64 `json:"consumed"`
		Left     float64 `json:"left"`
		Work     string  `json:"work"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if err := h.client.RecordEffort(id, req.Date, req.Consumed, req.Left, req.Work); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "工时已记录", nil)
}

// ---- Plan create/update/delete/link ----

func (h *WriteHandler) CreatePlan(ctx context.Context, c *app.RequestContext) {
	productID := queryInt(c, "productId")
	if productID == 0 {
		errors.BadRequest(c, "缺少 productId")
		return
	}
	var req zentaosdk.PlanCreateRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if _, err := h.client.CreatePlan(productID, req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "计划已创建", nil)
}

func (h *WriteHandler) UpdatePlan(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	var req zentaosdk.PlanCreateRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if _, err := h.client.UpdatePlan(id, req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "计划已更新", nil)
}

func (h *WriteHandler) DeletePlan(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	if err := h.client.DeletePlan(id); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "计划已删除", nil)
}

func (h *WriteHandler) LinkStoriesToPlan(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	var req struct {
		StoryIDs []int `json:"storyIds"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if err := h.client.LinkStoriesToPlan(id, req.StoryIDs); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "需求已关联到计划", nil)
}

func (h *WriteHandler) UnlinkStoriesFromPlan(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	var req struct {
		StoryIDs []int `json:"storyIds"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if err := h.client.UnlinkStoriesFromPlan(id, req.StoryIDs); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "需求已从计划移除", nil)
}

func (h *WriteHandler) LinkBugsToPlan(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	var req struct {
		BugIDs []int `json:"bugIds"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if err := h.client.LinkBugsToPlan(id, req.BugIDs); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "Bug 已关联到计划", nil)
}

func (h *WriteHandler) UnlinkBugsFromPlan(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	var req struct {
		BugIDs []int `json:"bugIds"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if err := h.client.UnlinkBugsFromPlan(id, req.BugIDs); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "Bug 已从计划移除", nil)
}

// ---- Case create/update/delete ----

func (h *WriteHandler) CreateCase(ctx context.Context, c *app.RequestContext) {
	productID := queryInt(c, "productId")
	if productID == 0 {
		errors.BadRequest(c, "缺少 productId")
		return
	}
	var req zentaosdk.CaseCreateRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if _, err := h.client.CreateCase(productID, req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "用例已创建", nil)
}

func (h *WriteHandler) UpdateCase(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	var req zentaosdk.CaseUpdateRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if _, err := h.client.UpdateCase(id, req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "用例已更新", nil)
}

func (h *WriteHandler) DeleteCase(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	if err := h.client.DeleteCase(id); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "用例已删除", nil)
}

// ---- Product/Project/Program/Execution/Build/User CRUD ----
// 这些是基础实体管理端点。为简洁，每个实体三个端点（create/update/delete）。

func (h *WriteHandler) CreateProduct(ctx context.Context, c *app.RequestContext) {
	var req zentaosdk.ProductCreateRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if _, err := h.client.CreateProduct(req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "产品已创建", nil)
}

func (h *WriteHandler) UpdateProduct(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	var req zentaosdk.ProductCreateRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if _, err := h.client.UpdateProduct(id, req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "产品已更新", nil)
}

func (h *WriteHandler) DeleteProduct(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	if err := h.client.DeleteProduct(id); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "产品已删除", nil)
}

func (h *WriteHandler) CreateProject(ctx context.Context, c *app.RequestContext) {
	var req zentaosdk.ProjectCreateRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if _, err := h.client.CreateProject(req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "项目已创建", nil)
}

func (h *WriteHandler) UpdateProject(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	var req zentaosdk.ProjectCreateRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if _, err := h.client.UpdateProject(id, req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "项目已更新", nil)
}

func (h *WriteHandler) DeleteProject(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	if err := h.client.DeleteProject(id); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "项目已删除", nil)
}

func (h *WriteHandler) CreateProgram(ctx context.Context, c *app.RequestContext) {
	var req zentaosdk.ProgramCreateRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if _, err := h.client.CreateProgram(req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "项目集已创建", nil)
}

func (h *WriteHandler) UpdateProgram(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	var req zentaosdk.ProgramCreateRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if _, err := h.client.UpdateProgram(id, req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "项目集已更新", nil)
}

func (h *WriteHandler) DeleteProgram(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	if err := h.client.DeleteProgram(id); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "项目集已删除", nil)
}

func (h *WriteHandler) CreateExecution(ctx context.Context, c *app.RequestContext) {
	projectID := queryInt(c, "projectId")
	if projectID == 0 {
		errors.BadRequest(c, "缺少 projectId")
		return
	}
	var req zentaosdk.ExecutionCreateRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if _, err := h.client.CreateExecution(projectID, req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "执行已创建", nil)
}

func (h *WriteHandler) UpdateExecution(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	var req zentaosdk.ExecutionCreateRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if _, err := h.client.UpdateExecution(id, req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "执行已更新", nil)
}

func (h *WriteHandler) DeleteExecution(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	if err := h.client.DeleteExecution(id); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "执行已删除", nil)
}

func (h *WriteHandler) CreateBuild(ctx context.Context, c *app.RequestContext) {
	projectID := queryInt(c, "projectId")
	if projectID == 0 {
		errors.BadRequest(c, "缺少 projectId")
		return
	}
	var req zentaosdk.BuildCreateRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if _, err := h.client.CreateBuild(projectID, req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "版本已创建", nil)
}

func (h *WriteHandler) UpdateBuild(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	var req zentaosdk.BuildCreateRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if _, err := h.client.UpdateBuild(id, req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "版本已更新", nil)
}

func (h *WriteHandler) DeleteBuild(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	if err := h.client.DeleteBuild(id); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "版本已删除", nil)
}

func (h *WriteHandler) CreateUser(ctx context.Context, c *app.RequestContext) {
	var req zentaosdk.UserCreateRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if _, err := h.client.CreateUser(req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "用户已创建", nil)
}

func (h *WriteHandler) UpdateUser(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	var req zentaosdk.UserUpdateRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "参数格式错误")
		return
	}
	if _, err := h.client.UpdateUser(id, req); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "用户已更新", nil)
}

func (h *WriteHandler) DeleteUser(ctx context.Context, c *app.RequestContext) {
	id, err := pathID(c, "id")
	if err != nil {
		errors.Error(c, err)
		return
	}
	if err := h.client.DeleteUser(id); err != nil {
		errors.Error(c, errors.ExternalError("禅道", err))
		return
	}
	errors.SuccessWithMessage(c, "用户已删除", nil)
}

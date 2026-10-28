package handlers

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/yi-nology/zentao-max/backend/pkg/errors"
	"github.com/yi-nology/zentao-max/backend/internal/models"
	"github.com/yi-nology/zentao-max/backend/internal/service"
)

type SchedulerHandler struct {
	schedulerService *service.SchedulerService
	webhookService   *service.WebhookService
	reportService    *service.ReportService
}

func NewSchedulerHandler(schedulerService *service.SchedulerService, webhookService *service.WebhookService, reportService *service.ReportService) *SchedulerHandler {
	return &SchedulerHandler{
		schedulerService: schedulerService,
		webhookService:   webhookService,
		reportService:    reportService,
	}
}

func (h *SchedulerHandler) ListTasks(ctx context.Context, c *app.RequestContext) {
	tasks, err := h.schedulerService.ListTasks()
	if err != nil {
		errors.Error(c, errors.ExternalError("调度器", err))
		return
	}
	errors.Success(c, tasks)
}

func (h *SchedulerHandler) CreateTask(ctx context.Context, c *app.RequestContext) {
	var task models.SchedulerTask
	if err := c.BindAndValidate(&task); err != nil {
		errors.BadRequest(c, "请求参数格式错误")
		return
	}
	if task.Name == "" {
		errors.MissingParam(c, "name")
		return
	}
	if task.CronExpr == "" {
		errors.MissingParam(c, "cronExpr")
		return
	}
	if task.ProjectID == 0 && task.ProductID == 0 {
		errors.BadRequest(c, "请提供产品ID或项目ID")
		return
	}
	if err := h.schedulerService.CreateTask(&task); err != nil {
		errors.Error(c, errors.NewInternalError("创建定时任务失败", err))
		return
	}
	errors.Success(c, task)
}

func (h *SchedulerHandler) UpdateTask(ctx context.Context, c *app.RequestContext) {
	id := c.Param("id")
	if id == "" {
		errors.MissingParam(c, "id")
		return
	}
	var task models.SchedulerTask
	if err := c.BindAndValidate(&task); err != nil {
		errors.BadRequest(c, "请求参数格式错误")
		return
	}
	task.ID = id
	if err := h.schedulerService.UpdateTask(&task); err != nil {
		errors.Error(c, errors.NewInternalError("更新定时任务失败", err))
		return
	}
	errors.Success(c, task)
}

func (h *SchedulerHandler) DeleteTask(ctx context.Context, c *app.RequestContext) {
	id := c.Param("id")
	if id == "" {
		errors.MissingParam(c, "id")
		return
	}
	if err := h.schedulerService.DeleteTask(id); err != nil {
		errors.Error(c, errors.NewInternalError("删除定时任务失败", err))
		return
	}
	errors.Success(c, nil)
}

func (h *SchedulerHandler) ToggleTask(ctx context.Context, c *app.RequestContext) {
	id := c.Param("id")
	if id == "" {
		errors.MissingParam(c, "id")
		return
	}
	task, err := h.schedulerService.ToggleTask(id)
	if err != nil {
		errors.Error(c, errors.NewInternalError("切换定时任务状态失败", err))
		return
	}
	errors.Success(c, task)
}

func (h *SchedulerHandler) RunTaskNow(ctx context.Context, c *app.RequestContext) {
	id := c.Param("id")
	if id == "" {
		errors.MissingParam(c, "id")
		return
	}
	logEntry, err := h.schedulerService.RunTaskNow(id)
	if err != nil {
		errors.Error(c, errors.NewInternalError("手动执行任务失败", err))
		return
	}
	errors.Success(c, logEntry)
}

func (h *SchedulerHandler) TestWebhook(ctx context.Context, c *app.RequestContext) {
	var req struct {
		URL string `json:"url"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "请求参数格式错误")
		return
	}
	if req.URL == "" {
		errors.MissingParam(c, "url")
		return
	}
	result, err := h.webhookService.TestWebhook(req.URL)
	if err != nil {
		errors.Error(c, errors.NewInternalError("测试Webhook失败", err))
		return
	}
	errors.Success(c, result)
}

func (h *SchedulerHandler) GetTaskLogs(ctx context.Context, c *app.RequestContext) {
	id := c.Param("id")
	if id == "" {
		errors.MissingParam(c, "id")
		return
	}
	logs, err := h.schedulerService.ListLogs(id, 50)
	if err != nil {
		errors.Error(c, errors.NewInternalError("获取执行日志失败", err))
		return
	}
	errors.Success(c, logs)
}

func (h *SchedulerHandler) GetAllLogs(ctx context.Context, c *app.RequestContext) {
	logs, err := h.schedulerService.ListLogs("", 100)
	if err != nil {
		errors.Error(c, errors.NewInternalError("获取执行日志失败", err))
		return
	}
	errors.Success(c, logs)
}

func (h *SchedulerHandler) PreviewReport(ctx context.Context, c *app.RequestContext) {
	var req struct {
		ReportType        string   `json:"reportType"`
		ProductID         int      `json:"productId"`
		ProjectID         int      `json:"projectId"`
		ProjectName       string   `json:"projectName"`
		ProductName       string   `json:"productName"`
		StatusFilter      string   `json:"statusFilter"`
		AgingDays         int      `json:"agingDays"`
		Keyword           string   `json:"keyword"`
		ExternalInfo      string   `json:"externalInfo"`
		MessageHeader     string   `json:"messageHeader"`
		PriorityAssignees []string `json:"priorityAssignees"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "请求参数格式错误")
		return
	}
	if req.ProjectID == 0 && req.ProductID == 0 {
		errors.BadRequest(c, "请提供产品ID或项目ID")
		return
	}

	reportType := req.ReportType
	if reportType == "" {
		reportType = "bug"
	}

	switch reportType {
	case "requirement":
		report, err := h.reportService.GenerateRequirementReport(req.ProductID, req.ProjectID, req.ProjectName, req.ProductName, req.Keyword, req.ExternalInfo, req.MessageHeader, req.PriorityAssignees)
		if err != nil {
			errors.Error(c, errors.ExternalError("禅道API", err))
			return
		}
		errors.Success(c, report)
	case "task":
		report, err := h.reportService.GenerateTaskReport(req.ProductID, req.ProjectID, req.ProjectName, req.ProductName, req.Keyword, req.ExternalInfo, req.MessageHeader, req.PriorityAssignees)
		if err != nil {
			errors.Error(c, errors.ExternalError("禅道API", err))
			return
		}
		errors.Success(c, report)
	case "bug-aging":
		agingDays := req.AgingDays
		if agingDays <= 0 {
			agingDays = 7
		}
		report, err := h.reportService.GenerateBugAgingReport(req.ProductID, req.ProjectID, req.ProjectName, req.StatusFilter, agingDays, req.Keyword, req.ExternalInfo, req.PriorityAssignees, req.MessageHeader)
		if err != nil {
			errors.Error(c, errors.ExternalError("禅道API", err))
			return
		}
		errors.Success(c, report)
	default:
		report, err := h.reportService.GenerateBugReport(req.ProductID, req.ProjectID, req.ProjectName, req.StatusFilter, req.Keyword, req.ExternalInfo, req.MessageHeader, req.PriorityAssignees)
		if err != nil {
			errors.Error(c, errors.ExternalError("禅道API", err))
			return
		}
		errors.Success(c, report)
	}
}

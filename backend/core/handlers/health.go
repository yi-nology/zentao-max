package handlers

import (
	"context"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/yi-nology/zentao-max/backend/core/dto"
	"github.com/yi-nology/zentao-max/backend/core/errors"
	"github.com/yi-nology/zentao-max/backend/core/service"
	"github.com/yi-nology/zentao-max/backend/core/zentao"
)

type HealthHandler struct {
	zentaoClient     *zentao.Client
	productService   ProductServicer
	projectService   ProjectServicer
	bugService       BugServicer
	storyService     StoryServicer
	taskService      TaskServicer
	userService      UserServicer
	schedulerService *service.SchedulerService
}

func NewHealthHandler(
	zentaoClient *zentao.Client,
	productService ProductServicer,
	projectService ProjectServicer,
	bugService BugServicer,
	storyService StoryServicer,
	taskService TaskServicer,
	userService UserServicer,
	schedulerService *service.SchedulerService,
) *HealthHandler {
	return &HealthHandler{
		zentaoClient:     zentaoClient,
		productService:   productService,
		projectService:   projectService,
		bugService:       bugService,
		storyService:     storyService,
		taskService:      taskService,
		userService:      userService,
		schedulerService: schedulerService,
	}
}

// SetSchedulerService 设置调度器服务（延迟初始化）
func (h *HealthHandler) SetSchedulerService(s *service.SchedulerService) {
	h.schedulerService = s
}

type CheckItem struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Count     int    `json:"count,omitempty"`
	Message   string `json:"message,omitempty"`
	LatencyMs int64  `json:"latencyMs,omitempty"`
}

type HealthCheckResponse struct {
	Timestamp string      `json:"timestamp"`
	Zentao    *CheckItem  `json:"zentao"`
	Checks    []CheckItem `json:"checks"`
	Summary   SummaryInfo `json:"summary"`
}

type SummaryInfo struct {
	Total   int  `json:"total"`
	Ok      int  `json:"ok"`
	Fail    int  `json:"fail"`
	Healthy bool `json:"healthy"`
}

func (h *HealthHandler) Check(ctx context.Context, c *app.RequestContext) {
	start := time.Now()
	resp := HealthCheckResponse{
		Timestamp: start.Format(time.RFC3339),
	}

	zentaoCheck := h.checkZentao()
	resp.Zentao = &zentaoCheck

	if zentaoCheck.Status != "ok" {
		resp.Checks = []CheckItem{}
		resp.Summary = SummaryInfo{Total: 1, Fail: 1, Healthy: false}
		errors.Success(c, resp)
		return
	}

	type checkFn struct {
		name string
		fn   func() CheckItem
	}
	checks := []checkFn{
		{"products", h.checkProducts},
		{"projects", h.checkProjects},
		{"bugs", h.checkBugs},
		{"stories", h.checkStories},
		{"tasks", h.checkTasks},
		{"users", h.checkUsers},
		{"scheduler", h.checkScheduler},
	}

	var wg sync.WaitGroup
	results := make([]CheckItem, len(checks))
	for i, check := range checks {
		wg.Add(1)
		go func(idx int, c checkFn) {
			defer wg.Done()
			item := c.fn()
			item.Name = c.name
			results[idx] = item
		}(i, check)
	}
	wg.Wait()

	resp.Checks = results

	okCount := 0
	failCount := 0
	if zentaoCheck.Status == "ok" {
		okCount++
	} else {
		failCount++
	}
	for _, r := range results {
		if r.Status == "ok" {
			okCount++
		} else {
			failCount++
		}
	}

	resp.Summary = SummaryInfo{
		Total:   1 + len(results),
		Ok:      okCount,
		Fail:    failCount,
		Healthy: failCount == 0,
	}

	errors.Success(c, resp)
}

func (h *HealthHandler) checkZentao() CheckItem {
	start := time.Now()
	if !h.zentaoClient.IsConnected() {
		return CheckItem{
			Status:    "fail",
			Message:   "未连接到禅道服务器，请检查配置",
			LatencyMs: time.Since(start).Milliseconds(),
		}
	}
	server := h.zentaoClient.GetServer()
	account := h.zentaoClient.GetAccount()
	return CheckItem{
		Status:    "ok",
		Message:   "已连接 " + server + " (账号: " + account + ")",
		LatencyMs: time.Since(start).Milliseconds(),
	}
}

func (h *HealthHandler) checkProducts() CheckItem {
	start := time.Now()
	products, err := h.productService.GetProducts()
	if err != nil {
		return CheckItem{
			Status:    "fail",
			Message:   err.Error(),
			LatencyMs: time.Since(start).Milliseconds(),
		}
	}
	return CheckItem{
		Status:    "ok",
		Count:     len(products),
		Message:   "正常",
		LatencyMs: time.Since(start).Milliseconds(),
	}
}

func (h *HealthHandler) checkProjects() CheckItem {
	start := time.Now()
	projects, err := h.projectService.GetProjects(&dto.ProjectQueryDTO{})
	if err != nil {
		return CheckItem{
			Status:    "fail",
			Message:   err.Error(),
			LatencyMs: time.Since(start).Milliseconds(),
		}
	}
	return CheckItem{
		Status:    "ok",
		Count:     len(projects),
		Message:   "正常",
		LatencyMs: time.Since(start).Milliseconds(),
	}
}

func (h *HealthHandler) checkBugs() CheckItem {
	start := time.Now()
	result, err := h.bugService.GetBugs(&dto.BugQueryDTO{PageSize: 1})
	if err != nil {
		return CheckItem{
			Status:    "fail",
			Message:   err.Error(),
			LatencyMs: time.Since(start).Milliseconds(),
		}
	}
	return CheckItem{
		Status:    "ok",
		Count:     result.Total,
		Message:   "正常",
		LatencyMs: time.Since(start).Milliseconds(),
	}
}

func (h *HealthHandler) checkStories() CheckItem {
	start := time.Now()
	result, err := h.storyService.GetStories(&dto.StoryQueryDTO{PageSize: 1})
	if err != nil {
		return CheckItem{
			Status:    "fail",
			Message:   err.Error(),
			LatencyMs: time.Since(start).Milliseconds(),
		}
	}
	return CheckItem{
		Status:    "ok",
		Count:     result.Total,
		Message:   "正常",
		LatencyMs: time.Since(start).Milliseconds(),
	}
}

func (h *HealthHandler) checkTasks() CheckItem {
	start := time.Now()
	result, err := h.taskService.GetTasks(&dto.TaskQueryDTO{PageSize: 1})
	if err != nil {
		return CheckItem{
			Status:    "fail",
			Message:   err.Error(),
			LatencyMs: time.Since(start).Milliseconds(),
		}
	}
	return CheckItem{
		Status:    "ok",
		Count:     result.Total,
		Message:   "正常",
		LatencyMs: time.Since(start).Milliseconds(),
	}
}

func (h *HealthHandler) checkUsers() CheckItem {
	start := time.Now()
	users, err := h.userService.GetUsersAll()
	if err != nil {
		return CheckItem{
			Status:    "fail",
			Message:   err.Error(),
			LatencyMs: time.Since(start).Milliseconds(),
		}
	}
	return CheckItem{
		Status:    "ok",
		Count:     len(users),
		Message:   "正常",
		LatencyMs: time.Since(start).Milliseconds(),
	}
}

func (h *HealthHandler) checkScheduler() CheckItem {
	start := time.Now()
	if h.schedulerService == nil {
		return CheckItem{
			Status:    "ok",
			Message:   "调度器未初始化",
			LatencyMs: time.Since(start).Milliseconds(),
		}
	}
	tasks, err := h.schedulerService.ListTasks()
	if err != nil {
		return CheckItem{
			Status:    "fail",
			Message:   err.Error(),
			LatencyMs: time.Since(start).Milliseconds(),
		}
	}
	return CheckItem{
		Status:    "ok",
		Count:     len(tasks),
		Message:   "正常",
		LatencyMs: time.Since(start).Milliseconds(),
	}
}

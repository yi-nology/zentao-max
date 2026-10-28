package handlers

import (
	"context"
	"time"

	"github.com/oekit/concurrency/group"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/yi-nology/zentao-max/backend/internal/dto"
	"github.com/yi-nology/zentao-max/backend/pkg/errors"
	"github.com/yi-nology/zentao-max/backend/internal/service"
	"github.com/yi-nology/zentao-max/backend/internal/infra/zentao"
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

	// 检查限时：story/task 等在大实例上会全量遍历执行（数千个），
	// 无超时会拖死整个 healthz（前端心跳页长期转圈）。
	// oekit group.NewWaitGroup：并发 + 超时编排一体，panic 已由 Wrap 内置
	// GoSafe 收编；超时后未完成的检查项标记 fail
	results := make([]CheckItem, len(checks))
	wg := group.NewWaitGroup(context.Background(), group.WithTimeout(20*time.Second))
	for i, check := range checks {
		wg.Wrap(func(_ context.Context) error {
			item := check.fn()
			item.Name = check.name
			results[i] = item
			return nil
		})
	}
	_ = wg.Wait()
	for idx := range results {
		if results[idx].Name == "" && results[idx].Status == "" {
			results[idx] = CheckItem{
				Name:    checks[idx].name,
				Status:  "fail",
				Message: "检测超时（数据量大，请稍后重试）",
			}
		}
	}

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
	// connected 标志由后台 ticker 异步同步，可能滞后于真实连通性
	// （会话每请求自动重登）。心跳以实时探测为准：能拉到当前用户即视为连接。
	if !h.zentaoClient.IsConnected() {
		if user, err := h.zentaoClient.GetCurrentUser(); err == nil && user != nil && user.ID > 0 {
			h.zentaoClient.MarkConnected()
			return CheckItem{
				Status:    "ok",
				Message:   "已连接 " + h.zentaoClient.GetServer() + " (账号: " + user.Account + ")",
				LatencyMs: time.Since(start).Milliseconds(),
			}
		}
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

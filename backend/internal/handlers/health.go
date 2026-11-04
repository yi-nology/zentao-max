package handlers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/oekit/concurrency/singleflight"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/yi-nology/zentao-max/backend/internal/dto"
	"github.com/yi-nology/zentao-max/backend/internal/infra/zentao"
	"github.com/yi-nology/zentao-max/backend/internal/service"
	"github.com/yi-nology/zentao-max/backend/pkg/errors"
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
	// healthz 聚合结果短缓存（90s）：心跳页重复访问秒开；
	// 未命中时经 oekit singleflight 收敛并发重算（只有一个真算，其余共享）
	cacheMu     sync.Mutex
	cachedResp  *HealthCheckResponse
	cachedAt    time.Time
	recomputeFl *singleflight.Flight[*HealthCheckResponse]
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
	// Cached 标记本次结果来自 90s 内存缓存（前端可据此弱化刷新频率）
	Cached bool `json:"cached,omitempty"`
}

type SummaryInfo struct {
	Total   int  `json:"total"`
	Ok      int  `json:"ok"`
	Fail    int  `json:"fail"`
	Timeout int  `json:"timeout"` // 大实例遍历超时项，非故障，不拉低 Healthy
	Healthy bool `json:"healthy"`
}

func (h *HealthHandler) Check(ctx context.Context, c *app.RequestContext) {
	// 90s 内存缓存：healthz 聚合 7 项检查（单次 20s 上限），心跳页每次访问
	// 都全量重跑会让页面反复等 20s——短缓存让重复访问秒开，到期后下一次
	// 访问触发重算；未命中经 oekit singleflight 收敛并发重算（2026-11-04）。
	h.cacheMu.Lock()
	if h.cachedResp != nil && time.Since(h.cachedAt) < 90*time.Second {
		cached := *h.cachedResp
		cached.Cached = true
		cached.Timestamp = time.Now().Format(time.RFC3339)
		h.cacheMu.Unlock()
		errors.Success(c, cached)
		return
	}
	h.cacheMu.Unlock()

	// 背景化 ctx：重算与请求生命周期解耦（共享结果不随首个请求断开而中断），
	// 并发未命中只有一个真算，其余共享结果
	resp, sfErr := h.recomputeFl.Do(context.Background(), "healthz", func(ctx context.Context) (*HealthCheckResponse, error) {
		return h.recomputeHealth(ctx)
	})
	if sfErr != nil {
		errors.InternalError(c, sfErr.Error())
		return
	}
	h.cacheMu.Lock()
	resp.Cached = false
	h.cacheMu.Unlock()
	errors.Success(c, *resp)
}

// recomputeHealth 全量重算 7 项检查（singleflight 回调；只算不写 HTTP）。
func (h *HealthHandler) recomputeHealth(ctx context.Context) (*HealthCheckResponse, error) {
	start := time.Now()
	resp := HealthCheckResponse{
		Timestamp: start.Format(time.RFC3339),
	}

	// checkZentao 里的 GetCurrentUser 实时探测无内部超时——会话建立挂住时
	// （VPN 抖动/底层 dial 105s）曾把整个 healthz 拖到 150s+。这里手工 20s
	// 兜底，与下方检查组的限时语义一致。
	zentaoCh := make(chan CheckItem, 1)
	go func() { zentaoCh <- h.checkZentao() }()
	var zentaoCheck CheckItem
	select {
	case zentaoCheck = <-zentaoCh:
	case <-time.After(20 * time.Second):
		zentaoCheck = CheckItem{
			Status:    "fail",
			Message:   "禅道探测超时（20s 内无响应，请检查网络/VPN）",
			LatencyMs: 20000,
		}
	}
	resp.Zentao = &zentaoCheck

	if zentaoCheck.Status != "ok" {
		resp.Checks = []CheckItem{}
		resp.Summary = SummaryInfo{Total: 1, Fail: 1, Healthy: false}
		h.storeCache(&resp)
		return &resp, nil
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
	// ⚠️ 不能用 oekit group.NewWaitGroup(WithTimeout)——其超时靠 cb 感知 ctx，
	// 而这些 check.fn() 无参不接收 ctx，Wait() 会死等所有 fn 完成（实测挂 170s+）。
	// 这里用 channel 收集 + 硬超时：到期即返回，未完成项标记 fail，
	// 迟到的 goroutine 结果被丢弃（buffered channel 不阻塞写，无泄漏无竞争）。
	results := make([]CheckItem, len(checks))
	type checkResult struct {
		idx  int
		item CheckItem
	}
	ch := make(chan checkResult, len(checks))
	for i, check := range checks {
		go func(idx int, name string, fn func() CheckItem) {
			// check.fn 内部已无 panic 传播需求（原来由 oekit GoSafe 收编）；
			// 兜底 recover 转 fail，防止单检查崩溃拖垮 healthz
			defer func() {
				if r := recover(); r != nil {
					ch <- checkResult{idx, CheckItem{Name: name, Status: "fail", Message: fmt.Sprintf("检查异常: %v", r)}}
				}
			}()
			item := fn()
			item.Name = name
			ch <- checkResult{idx, item}
		}(i, check.name, check.fn)
	}
	deadline := time.After(20 * time.Second)
collect:
	for collected := 0; collected < len(checks); collected++ {
		select {
		case r := <-ch:
			results[r.idx] = r.item
		case <-deadline:
			break collect
		}
	}
	for idx := range results {
		if results[idx].Name == "" && results[idx].Status == "" {
			// 语义区分：timeout ≠ fail。大实例上 stories/tasks 全量遍历几乎必然
			// 超 20s，若计为 fail 会让 healthy 长期 False 误导用户以为系统故障；
			// timeout 仅表示"数据量大没跑完"，不影响健康判定
			results[idx] = CheckItem{
				Name:    checks[idx].name,
				Status:  "timeout",
				Message: "检测超时（数据量大，非故障；缓存预热后可恢复）",
			}
		}
	}

	resp.Checks = results

	okCount := 0
	failCount := 0
	timeoutCount := 0
	if zentaoCheck.Status == "ok" {
		okCount++
	} else {
		failCount++
	}
	for _, r := range results {
		switch r.Status {
		case "ok":
			okCount++
		case "timeout":
			timeoutCount++
		default:
			failCount++
		}
	}

	resp.Summary = SummaryInfo{
		Total:   1 + len(results),
		Ok:      okCount,
		Fail:    failCount,
		Timeout: timeoutCount,
		Healthy: failCount == 0,
	}

	h.storeCache(&resp)
	return &resp, nil
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

// storeCache 写入 healthz 聚合结果缓存（90s TTL，锁内快照拷贝）。
func (h *HealthHandler) storeCache(resp *HealthCheckResponse) {
	snap := *resp
	h.cacheMu.Lock()
	h.cachedResp = &snap
	h.cachedAt = time.Now()
	h.cacheMu.Unlock()
}

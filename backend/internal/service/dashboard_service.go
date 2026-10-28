package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/oekit/concurrency/async"
	"github.com/oekit/concurrency/fanout"

	"github.com/yi-nology/zentao-max/backend/pkg/logger"
	"github.com/yi-nology/zentao-max/backend/internal/repository/sqlite"
	"github.com/yi-nology/zentao-max/backend/internal/vo"
	myzentao "github.com/yi-nology/zentao-max/backend/internal/infra/zentao"
	zentaosdk "github.com/yi-nology/zentao-max/backend/internal/infra/zentao/sdk"
)

// DashboardService 仪表盘服务
type DashboardService struct {
	client *myzentao.Client
	cache  *CacheService // 可选，nil 表示不启用离线缓存
}

// NewDashboardService 创建仪表盘服务
func NewDashboardService(client *myzentao.Client) *DashboardService {
	return &DashboardService{client: client}
}

// SetCacheService 注入缓存服务（启用离线模式后由 registry 调用）
func (s *DashboardService) SetCacheService(cs *CacheService) {
	s.cache = cs
}

// GetDashboard 获取仪表盘数据
func (s *DashboardService) GetDashboard(productID int) (*vo.DashboardVO, error) {
	return s.GetDashboardContext(context.Background(), productID, "", "")
}

// GetProjectOverview 获取项目概览
func (s *DashboardService) GetProjectOverview(projectID int) (*vo.ProjectOverviewVO, error) {
	return s.GetProjectOverviewContext(context.Background(), projectID)
}

// GetPersonalTimelog 获取个人工时报表
func (s *DashboardService) GetPersonalTimelog(account string, productID int, dateFrom string, dateTo string, groupBy string) (*vo.PersonalTimelogVO, error) {
	return s.GetPersonalTimelogContext(context.Background(), account, productID, dateFrom, dateTo, groupBy)
}

// Search 全局搜索
func (s *DashboardService) Search(keyword string, productID int, page int, pageSize int) (*vo.SearchVO, error) {
	return s.SearchContext(context.Background(), keyword, productID, page, pageSize)
}

// ========== 统计辅助 ==========

func calcBugStats(bugs []zentaosdk.Bug) vo.BugStatsVO {
	stats := vo.BugStatsVO{
		BySeverity: make(map[string]int),
		ByType:     make(map[string]int),
		Total:      len(bugs),
	}
	for _, b := range bugs {
		switch b.Status {
		case "active":
			stats.Active++
		case "resolved":
			stats.Resolved++
		case "closed":
			stats.Closed++
		}
		stats.BySeverity[fmt.Sprintf("%v", b.Severity)]++
		bugType := b.Type
		if bugType == "" {
			bugType = "未分类"
		}
		stats.ByType[bugType]++
	}
	return stats
}

func calcStoryStats(stories []zentaosdk.Story) vo.StoryStatsVO {
	stats := vo.StoryStatsVO{
		ByStage: make(map[string]int),
		Total:   len(stories),
	}
	for _, st := range stories {
		switch st.Status {
		case "draft":
			stats.Draft++
		case "closed":
			stats.Closed++
		default:
			stats.Active++
		}
		stats.ByStage[fmt.Sprintf("%v", st.Stage)]++
	}
	return stats
}

func calcTaskStats(tasks []zentaosdk.Task) vo.TaskStatsVO {
	stats := vo.TaskStatsVO{Total: len(tasks)}
	for _, t := range tasks {
		switch t.Status {
		case "wait":
			stats.Wait++
		case "doing":
			stats.Doing++
		case "done":
			stats.Done++
		case "closed":
			stats.Closed++
		}
	}
	return stats
}

func convertBugs(bugs []zentaosdk.Bug) []vo.BugVO {
	result := make([]vo.BugVO, 0, len(bugs))
	for _, b := range bugs {
		result = append(result, vo.BugVO{
			ID:       b.ID,
			Project:  b.Project,
			Product:  b.Product,
			Title:    b.Title,
			Severity: b.Severity,
			Pri:      b.Pri,
			Type:     b.Type,
			Status:   b.Status,
		})
	}
	return result
}

func convertTasks(tasks []zentaosdk.Task) []vo.TaskVO {
	result := make([]vo.TaskVO, 0, len(tasks))
	for _, t := range tasks {
		result = append(result, vo.TaskVO{
			ID:        t.ID,
			Project:   t.Project,
			Execution: t.Execution,
			Name:      t.Name,
			Type:      t.Type,
			Pri:       t.Pri,
			Status:    t.Status,
		})
	}
	return result
}

func toFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case json.Number:
		f, _ := val.Float64()
		return f
	case string:
		f, _ := time.ParseDuration(val)
		return f.Hours()
	}
	return 0
}

func toInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	case json.Number:
		n, _ := val.Int64()
		return int(n)
	}
	return 0
}

func toWeekKey(dateStr string) string {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return dateStr
	}
	year, week := t.ISOWeek()
	return fmt.Sprintf("%d-W%02d", year, week)
}

// ========== Context-aware Dashboard 方法 ==========

// GetDashboardContext 获取仪表盘数据（支持 context 取消和日期范围过滤）
// startDate/endDate 为空表示不过滤；格式 YYYY-MM-DD
// 若注入了 CacheService，优先走缓存：缓存未命中回源后写入；回源失败有过期缓存时 fallback
//
// 缓存 key 必须把 (startDate, endDate) 编进 entityType，否则不同日期范围的请求
// 会共享同一份 cache，前端切日期看到的"过滤后数据"实际是第一次请求时的日期
// 过滤过的旧数据（V040+ 教训：date filter 在 cache 命中后被静默跳过）。
func (s *DashboardService) GetDashboardContext(ctx context.Context, productID int, startDate, endDate string) (*vo.DashboardVO, error) {
	if s.cache != nil && productID > 0 {
		// 缓存键含日期范围（不同时间范围不共享缓存）
		cacheKey := fmt.Sprintf("%s:%s:%s", storage.EntityDashboard, startDate, endDate)
		result, err := s.cache.GetOrLoad(ctx, cacheKey, productID, DefaultCacheTTL,
			func(ctx context.Context) ([]byte, error) {
				d, err := s.fetchDashboard(ctx, productID, startDate, endDate)
				if err != nil {
					return nil, err
				}
				return json.Marshal(d)
			})
		if err != nil {
			return nil, err
		}
		var dashboard vo.DashboardVO
		if err := json.Unmarshal(result.Data, &dashboard); err != nil {
			return nil, err
		}
		// 标记数据来源（前端据此显示离线徽标）
		dashboard.FromCache = result.FromCache
		dashboard.Stale = result.Stale
		return &dashboard, nil
	}
	return s.fetchDashboard(ctx, productID, startDate, endDate)
}

// fetchDashboard 真正从禅道拉数据并聚合（原 GetDashboardContext 主体）
//
// 三个并发 goroutine（bugs / stories / executions）通过 s.client 间接走
// SessionWorkerPool 限流：每个 goroutine 内的 doSessionJSON 自动受 pool
// 调度（4 read worker, 8 req/s 默认限流），不需要上层再手动同步。
// collectTasksContext 的 N 个 goroutine 同理。
//
// Session 模式专属：cache 命中（L1/L2 fresh）直接返；cache miss 走 pool
// 调 session.DoJSONRaw 拿原始字节；写后 doSessionPost 触发 cache.InvalidateEntity
// 强制下次读回源。详见 backend/internal/infra/zentao/session_cache.go / session_pool.go。
func (s *DashboardService) fetchDashboard(ctx context.Context, productID int, startDate, endDate string) (*vo.DashboardVO, error) {
	dashboard := &vo.DashboardVO{}

	var (
		bugs     []zentaosdk.Bug
		stories  []zentaosdk.Story
		execCtxs []myzentao.ExecutionContext
		wg       sync.WaitGroup
		bgCtx    context.Context
		cancel   context.CancelFunc
	)

	// 总超时保护：任务统计靠遍历执行收集（受限于禅道接口无产品级任务端点），
	// 大实例（数千执行）全遍历需要数分钟，无超时会拖死 dashboard。
	// 超时后返回已收集的部分数据（前端可见 FromCache/Stale 标志）。
	bgCtx, cancel = context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	wg.Add(3)
	async.GoSafe(func() {
		defer wg.Done()
		select {
		case <-bgCtx.Done():
			return
		default:
		}
		var err error
		bugs, err = s.client.GetAllBugsContext(bgCtx, productID)
		if err != nil {
			logger.Error("Failed to fetch bugs for dashboard", "error", err)
		}
	})
	async.GoSafe(func() {
		defer wg.Done()
		select {
		case <-bgCtx.Done():
			return
		default:
		}
		var err error
		stories, err = s.client.GetAllStoriesContext(bgCtx, productID)
		if err != nil {
			logger.Error("Failed to fetch stories for dashboard", "error", err)
		}
	})
	async.GoSafe(func() {
		defer wg.Done()
		select {
		case <-bgCtx.Done():
			return
		default:
		}
		var err error
		execCtxs, err = s.client.GetExecutionsByProductContext(bgCtx, productID)
		if err != nil {
			logger.Error("Failed to fetch executions for dashboard", "error", err)
		}
	})
	wg.Wait()

	// 检查 context 是否已取消
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// 按日期范围过滤（基于 OpenedDate）
	bugs = filterBugsByDateRange(bugs, startDate, endDate)
	stories = filterStoriesByDateRange(stories, startDate, endDate)

	if bugs != nil {
		dashboard.Bugs = calcBugStats(bugs)
		n := len(bugs)
		if n > 5 {
			n = 5
		}
		dashboard.RecentBugs = convertBugs(bugs[:n])
	}

	if stories != nil {
		dashboard.Stories = calcStoryStats(stories)
	}

	if execCtxs != nil {
		allTasks := collectTasksContext(bgCtx, s.client, execCtxs)
		allTasks = filterTasksByDateRange(allTasks, startDate, endDate)
		dashboard.Tasks = calcTaskStats(allTasks)
		n := len(allTasks)
		if n > 5 {
			n = 5
		}
		dashboard.RecentTasks = convertTasks(allTasks[:n])
	}

	return dashboard, nil
}

// filterBugsByDateRange 按 OpenedDate 过滤 Bug 列表
// startDate/endDate 为空表示不过滤；end 包含当天（截到 23:59:59）
func filterBugsByDateRange(bugs []zentaosdk.Bug, startDate, endDate string) []zentaosdk.Bug {
	if startDate == "" && endDate == "" || len(bugs) == 0 {
		return bugs
	}
	result := make([]zentaosdk.Bug, 0, len(bugs))
	for _, b := range bugs {
		// Bug.OpenedDate 是 interface{}，需要类型断言
		rawDate := ""
		switch v := b.OpenedDate.(type) {
		case string:
			rawDate = v
		default:
			if b.OpenedDate != nil {
				rawDate = fmt.Sprintf("%v", v)
			}
		}
		if isDateInRange(rawDate, startDate, endDate) {
			result = append(result, b)
		}
	}
	return result
}

func filterStoriesByDateRange(stories []zentaosdk.Story, startDate, endDate string) []zentaosdk.Story {
	if startDate == "" && endDate == "" || len(stories) == 0 {
		return stories
	}
	result := make([]zentaosdk.Story, 0, len(stories))
	for _, s := range stories {
		if isDateInRange(s.OpenedDate, startDate, endDate) {
			result = append(result, s)
		}
	}
	return result
}

func filterTasksByDateRange(tasks []zentaosdk.Task, startDate, endDate string) []zentaosdk.Task {
	if startDate == "" && endDate == "" || len(tasks) == 0 {
		return tasks
	}
	result := make([]zentaosdk.Task, 0, len(tasks))
	for _, t := range tasks {
		if isDateInRange(t.OpenedDate, startDate, endDate) {
			result = append(result, t)
		}
	}
	return result
}

// isDateInRange 判断 zentao 时间字符串（"2024-01-01 12:00:00" 或 "2024-01-01"）
// 是否落在 [startDate, endDate] 范围内
func isDateInRange(rawDate, startDate, endDate string) bool {
	if rawDate == "" {
		return false
	}
	// 截取日期部分
	day := rawDate
	if len(rawDate) >= 10 {
		day = rawDate[:10]
	}
	if startDate != "" && day < startDate {
		return false
	}
	if endDate != "" && day > endDate {
		return false
	}
	return true
}

// GetProjectOverviewContext 获取项目概览（支持 context 取消）
func (s *DashboardService) GetProjectOverviewContext(ctx context.Context, projectID int) (*vo.ProjectOverviewVO, error) {
	overview := &vo.ProjectOverviewVO{}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	project, err := s.client.GetProjectContext(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("获取项目信息失败: %w", err)
	}
	overview.Project = vo.ProjectInfoVO{
		ID:     project.ID,
		Name:   project.Name,
		Code:   project.Code,
		Status: project.Status,
		Begin:  project.Begin,
		End:    project.End,
	}

	var (
		bugs  []zentaosdk.Bug
		tasks []zentaosdk.Task
		wg    sync.WaitGroup
	)

	wg.Add(2)
	async.GoSafe(func() {
		defer wg.Done()
		bugs, _ = s.client.GetAllBugsByProjectContext(ctx, projectID)
	})
	async.GoSafe(func() {
		defer wg.Done()
		tasks = collectTasksByProjectContext(ctx, s.client, projectID)
	})
	wg.Wait()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if bugs != nil {
		overview.Bugs = calcBugStats(bugs)
	}
	overview.Tasks = calcTaskStats(tasks)

	return overview, nil
}

// GetPersonalTimelogContext 获取个人工时报表（支持 context 取消）
func (s *DashboardService) GetPersonalTimelogContext(ctx context.Context, account string, productID int, dateFrom string, dateTo string, groupBy string) (*vo.PersonalTimelogVO, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	result := &vo.PersonalTimelogVO{}

	execCtxs, err := s.client.GetExecutionsByProductContext(ctx, productID)
	if err != nil {
		return nil, err
	}

	dateMap := make(map[string]float64)
	projectMap := make(map[int]float64)
	projectNames := make(map[int]string)
	var details []vo.TimelogEntryVO

	for _, ec := range execCtxs {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		tasks, err := s.client.GetTasksContext(ctx, ec.Exec.ID, 1, 200)
		if err != nil || len(tasks) == 0 {
			continue
		}
		for _, t := range tasks {
			consumed := toFloat64(t.Consumed)
			if consumed <= 0 {
				continue
			}
			efforts, err := s.client.GetTaskEffortsContext(ctx, t.ID)
			if err != nil {
				continue
			}
			for _, e := range efforts {
				eAccount := e.Account
				if account != "" && eAccount != account {
					continue
				}
				if dateFrom != "" && e.Date < dateFrom {
					continue
				}
				if dateTo != "" && e.Date > dateTo {
					continue
				}

				c := toFloat64(e.Consumed)
				result.TotalHours += c

				dateKey := e.Date
				if groupBy == "week" {
					dateKey = toWeekKey(e.Date)
				} else if groupBy == "month" {
					if len(e.Date) >= 7 {
						dateKey = e.Date[:7]
					}
				}
				dateMap[dateKey] += c

				pid := toInt(e.Project)
				projectMap[pid] += c
				projectNames[pid] = ec.ProjName

				work := e.Work
				details = append(details, vo.TimelogEntryVO{
					ID:          e.ID,
					Work:        work,
					Date:        e.Date,
					Consumed:    c,
					ProjectID:   pid,
					ProjectName: projectNames[pid],
				})
			}
		}
	}

	for date, hours := range dateMap {
		result.ByDate = append(result.ByDate, vo.DateHoursVO{Date: date, Hours: hours})
	}
	for pid, hours := range projectMap {
		result.ByProject = append(result.ByProject, vo.ProjectHoursVO{
			ProjectID:   pid,
			ProjectName: projectNames[pid],
			Hours:       hours,
		})
	}
	result.Details = details
	return result, nil
}

// SearchContext 全局搜索（支持 context 取消）
func (s *DashboardService) SearchContext(ctx context.Context, keyword string, productID int, page int, pageSize int) (*vo.SearchVO, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	result := &vo.SearchVO{}

	if productID <= 0 {
		return result, nil
	}

	var items []vo.SearchItem
	kw := strings.ToLower(keyword)

	{
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		bugs, _ := s.client.GetAllBugsContext(ctx, productID)
		for _, b := range bugs {
			if strings.Contains(strings.ToLower(b.Title), kw) {
				items = append(items, vo.SearchItem{
					Type:   "bug",
					ID:     b.ID,
					Title:  b.Title,
					Status: b.Status,
					Extra:  map[string]interface{}{"severity": b.Severity, "pri": b.Pri},
				})
			}
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		stories, _ := s.client.GetAllStoriesContext(ctx, productID)
		for _, st := range stories {
			if strings.Contains(strings.ToLower(st.Title), kw) {
				items = append(items, vo.SearchItem{
					Type:   "story",
					ID:     st.ID,
					Title:  st.Title,
					Status: st.Status,
					Extra:  map[string]interface{}{"stage": st.Stage, "pri": st.Pri},
				})
			}
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		execCtxs, _ := s.client.GetExecutionsByProductContext(ctx, productID)
		for _, ec := range execCtxs {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			tasks, err := s.client.GetTasksContext(ctx, ec.Exec.ID, 1, 200)
			if err != nil {
				continue
			}
			for _, t := range tasks {
				if strings.Contains(strings.ToLower(t.Name), kw) {
					items = append(items, vo.SearchItem{
						Type:   "task",
						ID:     t.ID,
						Title:  t.Name,
						Status: t.Status,
						Extra:  map[string]interface{}{"execution": ec.ExecName},
					})
				}
			}
		}
	}

	result.Total = len(items)
	start := (page - 1) * pageSize
	if start > len(items) {
		start = len(items)
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	result.Items = items[start:end]
	return result, nil
}

// collectTasksContext 并发收集任务（支持 context 取消）
func collectTasksContext(ctx context.Context, client *myzentao.Client, execCtxs []myzentao.ExecutionContext) []zentaosdk.Task {
	// fanout.Map：并发取各执行的任务列表（单个失败忽略），结果保序拼接
	perExec, _ := fanout.Map(ctx, execCtxs, len(execCtxs),
		func(ctx context.Context, ec myzentao.ExecutionContext) ([]zentaosdk.Task, error) {
			tasks, err := client.GetTasksContext(ctx, ec.Exec.ID, 1, 200)
			if err != nil {
				return nil, nil // 单个执行失败不阻断整体收集
			}
			return tasks, nil
		})
	var all []zentaosdk.Task
	for _, tasks := range perExec {
		all = append(all, tasks...)
	}
	return all
}

// collectTasksByProjectContext 按项目收集任务（支持 context 取消）
func collectTasksByProjectContext(ctx context.Context, client *myzentao.Client, projectID int) []zentaosdk.Task {
	executions, err := client.GetExecutionsContext(ctx, projectID, 1, 200)
	if err != nil {
		return nil
	}
	// fanout 并发取各执行的任务（原串行逐个拉取，executions 多时明显变慢）
	perExec, _ := fanout.Map(ctx, executions, len(executions),
		func(ctx context.Context, e zentaosdk.Execution) ([]zentaosdk.Task, error) {
			tasks, err := client.GetTasksContext(ctx, e.ID, 1, 200)
			if err != nil {
				return nil, nil
			}
			return tasks, nil
		})
	var all []zentaosdk.Task
	for _, tasks := range perExec {
		all = append(all, tasks...)
	}
	return all
}

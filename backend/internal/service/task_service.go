package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	zentaosdk "github.com/yi-nology/zentao-max/backend/internal/infra/zentao/sdk"

	"github.com/yi-nology/zentao-max/backend/internal/dto"
	"github.com/yi-nology/zentao-max/backend/pkg/logger"
	"github.com/yi-nology/zentao-max/backend/pkg/utils"
	"github.com/yi-nology/zentao-max/backend/internal/vo"
	myzentao "github.com/yi-nology/zentao-max/backend/internal/infra/zentao"
)

const (
	tasksCacheDuration = 60 * time.Minute
)

// TaskService 任务业务逻辑服务
// 负责处理任务相关的业务逻辑
type TaskService struct {
	client *myzentao.Client
	cache  *CacheService // 可选；nil = 无 SWR 缓存直连
}

// NewTaskService 创建任务服务
func NewTaskService(client *myzentao.Client) *TaskService {
	return &TaskService{client: client}
}

// SetCache 注入 SWR 缓存（registry 在缓存层就绪后回调装配）。
func (s *TaskService) SetCache(c *CacheService) { s.cache = c }

// GetTasks 获取任务列表
// 业务逻辑：
// 1. 如果指定执行ID，则查询该执行的任务
// 2. 如果未指定执行ID，则查询所有执行的任务
// 3. 应用筛选条件（指派人、状态、时间范围）
// 4. 分页处理
func (s *TaskService) GetTasks(query *dto.TaskQueryDTO) (*vo.PaginatedVO, error) {
	chainFilter, err := s.filterTasks(query)
	if err != nil {
		return nil, err
	}

	// 获取总数
	total := chainFilter.Count()

	// 执行分页
	pagedTasks := chainFilter.Paginate(query.Page, query.PageSize).Result()

	list := s.convertToVO(pagedTasks)

	return &vo.PaginatedVO{
		List:     list,
		Total:    total,
		Page:     query.Page,
		PageSize: query.PageSize,
	}, nil
}

// GetTaskStatusCounts 返回应用筛选条件后（不分页）的各状态任务数量，供统计卡使用
func (s *TaskService) GetTaskStatusCounts(query *dto.TaskQueryDTO) (map[string]int, error) {
	chainFilter, err := s.filterTasks(query)
	if err != nil {
		return nil, err
	}

	counts := map[string]int{"doing": 0, "wait": 0, "done": 0, "closed": 0}
	for _, task := range chainFilter.Result() {
		if _, ok := counts[task.Status]; ok {
			counts[task.Status]++
		}
	}
	return counts, nil
}

// filterTasks 拉取任务（带缓存）并应用指派人/状态/时间范围筛选，返回全量筛选结果（不含分页）。
// 列表与统计卡共用同一筛选口径。
func (s *TaskService) filterTasks(query *dto.TaskQueryDTO) (*utils.ChainFilter[zentaosdk.Task], error) {
	var allTasks []zentaosdk.Task
	var err error

	if query.ExecutionID != 0 {
		cacheKey := fmt.Sprintf("tasks:execution:%d", query.ExecutionID)
		allTasks, err = s.getTasksWithCache(cacheKey, func() ([]zentaosdk.Task, error) {
			return s.client.GetTasks(query.ExecutionID, 1, 10000)
		})
		if err != nil {
			return nil, err
		}
	} else if query.ProductID > 0 {
		// 产品维度：任务挂在执行下，按「产品→项目集合」过滤——与 executions
		// 的产品诚实语义一致。此前不过滤返回全实例任务（产品 1029 执行为 0
		// 但任务页显示全实例任务，自相矛盾）。
		// 全量爬取走共享 SWR 条目 tasks:all（30min，一次 60s 预算的全执行
		// 遍历 + 8 并发爬取），之后各产品即时内存过滤，不再每产品缓存一份
		// 相同的全量数据。
		if s.cache != nil {
			loader := func(ctx context.Context) ([]byte, error) {
				// 传零值 deadline：60s 爬取预算从「执行列表就绪后」起算——
				// 此前在分支入口起算，缓存清空后 58s 的执行列表遍历会吃掉
				// 全部预算，爬取只剩 2s（实测 tasks 接口 data:null）
				tasks, err := s.fetchAllTasksAcrossExecutions(time.Time{})
				if err != nil {
					return nil, err
				}
				if len(tasks) == 0 {
					// 实例级空列表 = 疑似会话失效/实例异常：按错误处理，
					// SWR 会回退旧缓存而不是把空结果当合法数据缓存（防毒化）
					return nil, fmt.Errorf("任务全量遍历为空（疑似会话失效），不缓存空结果")
				}
				return json.Marshal(tasks)
			}
			res, err := s.cache.GetOrLoadSWR(context.Background(), "tasks:all", 0, 30*time.Minute, loader)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(res.Data, &allTasks); err != nil {
				return nil, fmt.Errorf("解析缓存任务失败: %w", err)
			}
		} else {
			allTasks, err = s.fetchAllTasksAcrossExecutions(time.Now().Add(60 * time.Second))
			if err != nil {
				return nil, err
			}
		}
		// 按产品项目集合过滤
		projSet, perr := s.productProjectSet(query.ProductID)
		if perr != nil {
			return nil, perr
		}
		filtered := make([]zentaosdk.Task, 0, len(allTasks)/4)
		for _, t := range allTasks {
			if _, ok := projSet[t.Project]; ok {
				filtered = append(filtered, t)
			}
		}
		logger.Info("产品维度任务过滤",
			"productID", query.ProductID,
			"projects", len(projSet),
			"allTasks", len(allTasks),
			"filtered", len(filtered),
		)
		allTasks = filtered
	} else if query.AssignedTo != "" && query.AssignedTo == s.client.GetAccount() {
		// 查"我的任务"：/my-work-task.json 直达当前用户，不遍历全执行。
		// （大实例上千执行的全遍历 20s+ 仍取不全，池降级后返回空表。）
		allTasks, err = s.client.GetMyWorkTasks(context.Background())
		if err != nil {
			return nil, err
		}
	} else {
		// productId 缺省或 0：直接全 execution 收集（dashboard 同款），独立 60s 闸
		allTasks, err = s.fetchAllTasksAcrossExecutions(time.Time{})
		if err != nil {
			return nil, err
		}
	}

	// 使用链式过滤器进行筛选
	chainFilter := utils.NewChainFilter(allTasks)

	// 按指派人筛选：AssignedTo 形状因数据源而异——token 模式是 UserRef，
	// session 模式是纯账号字符串（实测），v1/部分端点是 {account:...} map
	if query.AssignedTo != "" {
		chainFilter = chainFilter.Filter(func(item zentaosdk.Task) bool {
			switch v := item.AssignedTo.(type) {
			case zentaosdk.UserRef:
				return v.Account == query.AssignedTo
			case string:
				return v == query.AssignedTo
			case map[string]interface{}:
				if acc, ok := v["account"].(string); ok {
					return acc == query.AssignedTo
				}
			}
			return false
		})
	}

	// 按状态筛选：支持逗号分隔多状态（前端如 "wait,doing"）
	if query.Status != "" {
		statusSet := make(map[string]bool)
		for _, s := range strings.Split(query.Status, ",") {
			statusSet[strings.TrimSpace(s)] = true
		}
		chainFilter = chainFilter.Filter(func(item zentaosdk.Task) bool {
			return statusSet[item.Status]
		})
	}

	// 按日期范围筛选
	if query.StartDate != "" || query.EndDate != "" {
		chainFilter = chainFilter.FilterByDate(query.StartDate, query.EndDate, func(item zentaosdk.Task) string {
			return item.OpenedDate
		})
	}

	return chainFilter, nil
}

// RecordEffort 为任务记录一条工时日志（MCP record_effort 工具与任务详情共用）。
func (s *TaskService) RecordEffort(taskID int, date string, consumed, left float64, work string) error {
	return s.client.RecordEffort(taskID, date, consumed, left, work)
}

// convertToVO 将zentao.Task转换为vo.TaskVO
func (s *TaskService) convertToVO(tasks []zentaosdk.Task) []vo.TaskVO {
	if len(tasks) == 0 {
		return []vo.TaskVO{}
	}

	result := make([]vo.TaskVO, 0, len(tasks))
	for _, task := range tasks {
		result = append(result, vo.TaskVO{
			ID:           task.ID,
			Project:      task.Project,
			Execution:    task.Execution,
			Name:         task.Name,
			Type:         task.Type,
			Pri:          task.Pri,
			Status:       task.Status,
			AssignedTo:   task.AssignedTo,
			EstStarted:   task.EstStarted,
			Deadline:     task.Deadline,
			Estimate:     task.Estimate,
			Consumed:     task.Consumed,
			Left:         task.Left,
			Desc:         task.Desc,
			OpenedBy:     task.OpenedBy,
			OpenedDate:   task.OpenedDate,
			FinishedBy:   task.FinishedBy,
			FinishedDate: task.FinishedDate,
			ClosedBy:     task.ClosedBy,
			ClosedDate:   task.ClosedDate,
			StatusName:   task.StatusName,
		})
	}
	return result
}

// getTasksWithCache 直接调 loadFunc（已删除 zentao 包级 GlobalCache，缓存由 service 层 SQLite/其他层提供）。
func (s *TaskService) getTasksWithCache(_ string, loadFunc func() ([]zentaosdk.Task, error)) ([]zentaosdk.Task, error) {
	return loadFunc()
}

// fetchAllTasksAcrossExecutions 直接遍历全 execution 收集 task（dashboard 同款）。
//
// 受限 session 模式下 product 维度跟 user 维度数据不严格对应（<user-account> 的 task
// 不一定都关联到选中的 product），按 product 过滤容易 0 数据。dashboard 风格
// 直接遍历全 execution 是更稳的"全量"语义。
// deadline 为零值时使用独立 60s（dashboard 等直接调用方）；product 分支传入
// 共享总闸。遍历超时返回已收集部分，避免请求无限 pending（调用方有持久缓存兜底）。
func (s *TaskService) fetchAllTasksAcrossExecutions(deadline time.Time) ([]zentaosdk.Task, error) {
	t0 := time.Now()
	allExecs, err := allExecutionsViaSWR(s.cache, s.client)
	if err != nil {
		return nil, err
	}
	logger.Info("任务遍历：执行列表就绪",
		"elapsed", time.Since(t0).Round(time.Millisecond),
		"executions", len(allExecs),
	)
	if deadline.IsZero() {
		deadline = time.Now().Add(60 * time.Second)
	}
	// 并行爬取（此前串行一个执行接一个，客户端读池 cap 4 闲着）：
	// 信号量限 8 并发 + ctx 截止。deadline 必须拦在「进池排队之前」——
	// 否则全部 goroutine 堵在池队列里，wg.Wait 按池 cap 4 排干整条队列
	//（实测 60s 闸变 120s）。
	const workers = 8
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		allTasks []zentaosdk.Task
	)
	sem := make(chan struct{}, workers)
	tasksFor := func(execID int) {
		defer wg.Done()
		// ctx 先查 + 获取信号量后复查：select 在 sem/ctx.Done 双就绪时随机
		// 选择，超时后排队者仍会抢到 sem 去发请求，把 60s 闸拖成 120s+
		if ctx.Err() != nil {
			return
		}
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
		case <-ctx.Done():
			return
		}
		if ctx.Err() != nil {
			return
		}
		// ctx 打进 HTTP 请求：截止一到，在途请求直接取消，
		// wg.Wait 在预算边界确定性收口（此前 GetTasks 无 ctx，
		// 个别慢请求拖到 60s+，60s 预算实际跑到 137s）
		tasks, err := s.client.GetTasksContext(ctx, execID, 1, 10000)
		if err != nil {
			logger.Error("Failed to fetch tasks for execution", "executionID", execID, "error", err)
			return
		}
		mu.Lock()
		allTasks = append(allTasks, tasks...)
		mu.Unlock()
	}
	for _, e := range allExecs {
		if ctx.Err() != nil {
			logger.Warn("跨执行任务遍历超时，返回已收集部分", "executions", len(allExecs))
			break
		}
		wg.Add(1)
		go tasksFor(e.ID)
	}
	wg.Wait()
	logger.Info("跨执行任务遍历完成",
		"tasks", len(allTasks),
		"elapsed_total", time.Since(t0).Round(time.Millisecond),
		"crawl_budget", time.Until(deadline).Round(time.Second),
	)
	return allTasks, nil
}

// productProjectSet 产品→项目 ID 集合（SWR 30min 缓存，产品维度任务/执行
// 过滤共用）。
func (s *TaskService) productProjectSet(productID int) (map[int]struct{}, error) {
	projSet := map[int]struct{}{}
	if s.cache != nil {
		loader := func(ctx context.Context) ([]byte, error) {
			projects, err := s.client.GetProjectsByProduct(productID, 1, 10000)
			if err != nil {
				return nil, err
			}
			return json.Marshal(projects)
		}
		res, err := s.cache.GetOrLoadSWR(context.Background(), "projects:product", productID, 30*time.Minute, loader)
		if err != nil {
			return nil, err
		}
		var projects []zentaosdk.Project
		if err := json.Unmarshal(res.Data, &projects); err != nil {
			return nil, fmt.Errorf("解析缓存项目列表失败: %w", err)
		}
		for _, p := range projects {
			projSet[p.ID] = struct{}{}
		}
		return projSet, nil
	}
	projects, err := s.client.GetProjectsByProduct(productID, 1, 10000)
	if err != nil {
		return nil, err
	}
	for _, p := range projects {
		projSet[p.ID] = struct{}{}
	}
	return projSet, nil
}

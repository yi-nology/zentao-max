package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	zentaosdk "github.com/yi-nology/zentao-max/backend/core/zentao/sdk"
	"go.uber.org/zap"

	"github.com/yi-nology/zentao-max/backend/core/dto"
	"github.com/yi-nology/zentao-max/backend/core/logger"
	"github.com/yi-nology/zentao-max/backend/core/utils"
	"github.com/yi-nology/zentao-max/backend/core/vo"
	myzentao "github.com/yi-nology/zentao-max/backend/core/zentao"
)

const (
	tasksCacheDuration = 60 * time.Minute
)

// TaskService 任务业务逻辑服务
// 负责处理任务相关的业务逻辑
type TaskService struct {
	client *myzentao.Client
}

// NewTaskService 创建任务服务
func NewTaskService(client *myzentao.Client) *TaskService {
	return &TaskService{client: client}
}

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
		// 按 product 维度收集：受限 session 模式下，如果 product 范围内确实有
		// project/execution/task，filter 之后能看到；否则 fallback 到全 execution。
		cacheKey := "tasks:all:product:" + fmt.Sprint(query.ProductID)
		allTasks, err = s.getTasksWithCache(cacheKey, func() ([]zentaosdk.Task, error) {
			return s.fetchAllTasks(query.ProductID)
		})
		if err != nil {
			return nil, err
		}
		if len(allTasks) == 0 {
			// product 维度没数据时，fallback 到全 execution 收集（dashboard 同款）
			// 保证 <user-account> 等用户的 task 不被 product 过滤吞掉。
			allTasks, err = s.fetchAllTasksAcrossExecutions()
			if err != nil {
				return nil, err
			}
		}
	} else if query.AssignedTo != "" && query.AssignedTo == s.client.GetAccount() {
		// 查"我的任务"：/my-work-task.json 直达当前用户，不遍历全执行。
		// （大实例上千执行的全遍历 20s+ 仍取不全，池降级后返回空表。）
		allTasks, err = s.client.GetMyWorkTasks(context.Background())
		if err != nil {
			return nil, err
		}
	} else {
		// productId 缺省或 0：直接全 execution 收集（dashboard 同款）
		allTasks, err = s.fetchAllTasksAcrossExecutions()
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

// fetchAllTasks 获取所有执行的任务
func (s *TaskService) fetchAllTasks(productId int) ([]zentaosdk.Task, error) {
	var allTasks []zentaosdk.Task

	projects, err := s.client.GetProjectsByProduct(productId, 1, 2000)
	if err != nil {
		return nil, err
	}

	for _, project := range projects {
		executions, err := s.client.GetExecutions(project.ID, 1, 1000)
		if err != nil {
			logger.Error("Failed to fetch executions for project",
				zap.Int("projectID", project.ID),
				zap.Error(err))
			continue
		}

		for _, execution := range executions {
			tasks, err := s.client.GetTasks(execution.ID, 1, 10000)
			if err != nil {
				logger.Error("Failed to fetch tasks for execution",
					zap.Int("executionID", execution.ID),
					zap.Error(err))
				continue
			}
			allTasks = append(allTasks, tasks...)
		}
	}

	return allTasks, nil
}

// fetchAllTasksAcrossExecutions 直接遍历全 execution 收集 task（dashboard 同款）。
//
// 受限 session 模式下 product 维度跟 user 维度数据不严格对应（<user-account> 的 task
// 不一定都关联到选中的 product），按 product 过滤容易 0 数据。dashboard 风格
// 直接遍历全 execution 是更稳的"全量"语义。
func (s *TaskService) fetchAllTasksAcrossExecutions() ([]zentaosdk.Task, error) {
	// GetExecutionsByProductContext 在 session 模式下忽略 productID受限实现
	// 注：见 client_session_products.go:374 _ = productID），永远返回全 execution。
	allExecCtxs, err := s.client.GetExecutionsByProductContext(context.Background(), 0)
	if err != nil {
		return nil, err
	}
	var allTasks []zentaosdk.Task
	for _, ec := range allExecCtxs {
		tasks, err := s.client.GetTasks(ec.Exec.ID, 1, 10000)
		if err != nil {
			logger.Error("Failed to fetch tasks for execution",
				zap.Int("executionID", ec.Exec.ID),
				zap.Error(err))
			continue
		}
		allTasks = append(allTasks, tasks...)
	}
	return allTasks, nil
}

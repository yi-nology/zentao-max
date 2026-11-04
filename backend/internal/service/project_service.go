package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	zentaosdk "github.com/yi-nology/zentao-max/backend/internal/infra/zentao/sdk"

	"github.com/yi-nology/zentao-max/backend/internal/dto"
	"github.com/yi-nology/zentao-max/backend/pkg/logger"
	"github.com/yi-nology/zentao-max/backend/internal/vo"
	myzentao "github.com/yi-nology/zentao-max/backend/internal/infra/zentao"
)

// ProjectService 项目业务逻辑服务
// 负责处理项目相关的业务逻辑
type ProjectService struct {
	client *myzentao.Client
}

// NewProjectService 创建项目服务
func NewProjectService(client *myzentao.Client) *ProjectService {
	return &ProjectService{client: client}
}

// GetProjects 获取项目列表
// 业务逻辑：
// 1. 如果有产品ID，按产品筛选项目
// 2. 否则获取所有项目
func (s *ProjectService) GetProjects(query *dto.ProjectQueryDTO) ([]vo.ProjectVO, error) {
	var projects []zentaosdk.Project
	var err error

	if query.ProductID != 0 {
		// 按产品筛选
		projects, err = s.client.GetProjectsByProduct(query.ProductID, query.Page, query.PageSize)
	} else {
		// 获取所有项目
		projects, err = s.client.GetAllProjects(2000)
	}

	if err != nil {
		return nil, err
	}

	// 转换为VO
	return s.convertToVO(projects), nil
}

// convertToVO 将zentao.Project转换为vo.ProjectVO
func (s *ProjectService) convertToVO(projects []zentaosdk.Project) []vo.ProjectVO {
	if len(projects) == 0 {
		return []vo.ProjectVO{}
	}

	result := make([]vo.ProjectVO, 0, len(projects))
	for _, project := range projects {
		result = append(result, vo.ProjectVO{
			ID:       project.ID,
			Name:     project.Name,
			Code:     project.Code,
			Model:    project.Model,
			Type:     project.Type,
			Status:   project.Status,
			PM:       project.PM,
			Begin:    project.Begin,
			End:      project.End,
			Progress: project.Progress,
		})
	}
	return result
}

// ExecutionService 执行/迭代业务逻辑服务
type ExecutionService struct {
	client *myzentao.Client
	cache  *CacheService // 可选；nil = 无 SWR 缓存直连
}

// NewExecutionService 创建执行/迭代服务
func NewExecutionService(client *myzentao.Client) *ExecutionService {
	return &ExecutionService{client: client}
}

// SetCache 注入 SWR 缓存（registry 在缓存层就绪后回调装配）。
func (s *ExecutionService) SetCache(c *CacheService) { s.cache = c }

// GetExecutions 获取执行/迭代列表
// 业务逻辑：
// 1. 如果有项目ID，按项目筛选
// 2. 如果有产品ID，取产品项目集合 + 一次全执行列表后按项目归属过滤（SWR 缓存）
// 3. 否则返回实例全量执行
func (s *ExecutionService) GetExecutions(query *dto.ExecutionQueryDTO) ([]vo.ExecutionVO, error) {
	var executions []zentaosdk.Execution

	if query.ProjectID != 0 {
		// 按项目筛选
		var err error
		executions, err = s.client.GetExecutions(query.ProjectID, query.Page, query.PageSize)
		if err != nil {
			return nil, err
		}
	} else if query.ProductID != 0 {
		// 按产品筛选：此前「取产品项目列表 → 逐项目 GetExecutions」，而 session
		// 模式 GetExecutions 内部是全执行遍历再按项目过滤——N 个项目 = N 次
		// 全量遍历，实测产品维度每次请求 ~56s 且零缓存。改为「一次产品项目
		// 集合 + 一次全执行列表 + 内存过滤」，同样语义但只遍历一次。
		//
		// SWR 缓存：首拉一次全遍历，之后 30min 内秒回；过期秒回旧数据 +
		// 后台刷新（大成本回源不打扰用户）。
		const executionsSWRTTL = 30 * time.Minute
		if s.cache != nil {
			loader := func(ctx context.Context) ([]byte, error) {
				execs, err := s.fetchProductExecutions(query.ProductID)
				if err != nil {
					return nil, err
				}
				return json.Marshal(execs)
			}
			res, err := s.cache.GetOrLoadSWR(context.Background(), "executions:product", query.ProductID, executionsSWRTTL, loader)
			if err != nil {
				return nil, err
			}
			var filtered []zentaosdk.Execution
			if err := json.Unmarshal(res.Data, &filtered); err != nil {
				return nil, fmt.Errorf("解析缓存执行列表失败: %w", err)
			}
			return s.convertToVO(filtered), nil
		}

		filtered, err := s.fetchProductExecutions(query.ProductID)
		if err != nil {
			return nil, err
		}
		return s.convertToVO(filtered), nil
	} else {
		// 无项目/产品过滤：返回实例全量执行。此前直接返回空列表，
		// REST /api/executions 与 MCP get_executions 不带参数时永远空数组。
		var err error
		executions, err = s.client.GetAllExecutions()
		if err != nil {
			return nil, err
		}
	}

	// 转换为VO
	return s.convertToVO(executions), nil
}

// convertToVO 将zentao.Execution转换为vo.ExecutionVO
func (s *ExecutionService) convertToVO(executions []zentaosdk.Execution) []vo.ExecutionVO {
	if len(executions) == 0 {
		return []vo.ExecutionVO{}
	}

	result := make([]vo.ExecutionVO, 0, len(executions))
	for _, execution := range executions {
		result = append(result, vo.ExecutionVO{
			ID:      execution.ID,
			Project: execution.Project,
			Name:    execution.Name,
			Code:    execution.Code,
			Status:  execution.Status,
			Type:    execution.Type,
			Begin:   execution.Begin,
			End:     execution.End,
		})
	}
	return result
}

// fetchProductExecutions 产品维度的执行集合：一次产品项目列表 + 一次全执行
// 遍历 + 按项目归属内存过滤（原逐项目 GetExecutions = N × 全执行遍历，
// 实测产品维度单请求 ~56s 且零缓存）。
func (s *ExecutionService) fetchProductExecutions(productID int) ([]zentaosdk.Execution, error) {
	projects, err := s.client.GetProjectsByProduct(productID, 1, 10000)
	if err != nil {
		return nil, err
	}
	projSet := make(map[int]struct{}, len(projects))
	for _, p := range projects {
		projSet[p.ID] = struct{}{}
	}
	allExecs, err := allExecutionsViaSWR(s.cache, s.client)
	if err != nil {
		return nil, err
	}
	filtered := make([]zentaosdk.Execution, 0, len(allExecs)/8)
	for _, e := range allExecs {
		if _, ok := projSet[e.Project]; ok {
			filtered = append(filtered, e)
		}
	}
	logger.Info("产品维度执行列表（单次遍历+过滤）",
		"productID", productID,
		"projects", len(projects),
		"allExecutions", len(allExecs),
		"filtered", len(filtered),
	)
	return filtered, nil
}

// allExecutionsViaSWR 全实例执行列表（SWR 30min，缓存键 executions:all /
// productID=0）。executions 与 tasks 两条产品路径共用同一缓存条目：
// executions 先调时 tasks 的冷启动直接复用，不必再支付一次 58s 全遍历。
func allExecutionsViaSWR(cache *CacheService, client *myzentao.Client) ([]zentaosdk.Execution, error) {
	if cache == nil {
		return client.GetAllExecutionsContext(context.Background())
	}
	loader := func(ctx context.Context) ([]byte, error) {
		execs, err := client.GetAllExecutionsContext(context.Background())
		if err != nil {
			return nil, err
		}
		if len(execs) == 0 {
			// 实例级空列表 = 疑似会话失效/实例异常：按错误处理让 SWR 回退
			// 旧缓存，不把空结果当合法数据缓存（防毒化，2026-11-04）
			return nil, fmt.Errorf("全执行列表为空（疑似会话失效），不缓存空结果")
		}
		return json.Marshal(execs)
	}
	res, err := cache.GetOrLoadSWR(context.Background(), "executions:all", 0, 30*time.Minute, loader)
	if err != nil {
		return nil, err
	}
	var execs []zentaosdk.Execution
	if err := json.Unmarshal(res.Data, &execs); err != nil {
		return nil, fmt.Errorf("解析缓存全执行列表失败: %w", err)
	}
	return execs, nil
}

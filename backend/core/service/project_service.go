package service

import (
	zentaosdk "github.com/yi-nology/zentao-max/backend/core/zentao/sdk"
	"go.uber.org/zap"

	"github.com/yi-nology/zentao-max/backend/core/dto"
	"github.com/yi-nology/zentao-max/backend/core/logger"
	"github.com/yi-nology/zentao-max/backend/core/vo"
	myzentao "github.com/yi-nology/zentao-max/backend/core/zentao"
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
}

// NewExecutionService 创建执行/迭代服务
func NewExecutionService(client *myzentao.Client) *ExecutionService {
	return &ExecutionService{client: client}
}

// GetExecutions 获取执行/迭代列表
// 业务逻辑：
// 1. 如果有项目ID，按项目筛选
// 2. 如果有产品ID，获取产品下所有项目的执行/迭代
// 3. 否则返回空列表
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
		// 按产品筛选（获取产品下所有项目的执行/迭代）
		projects, err := s.client.GetProjectsByProduct(query.ProductID, query.Page, query.PageSize)
		if err != nil {
			return nil, err
		}

		// 获取所有项目的执行/迭代
		for _, project := range projects {
			projectExecutions, err := s.client.GetExecutions(project.ID, query.Page, query.PageSize)
			if err != nil {
				logger.Error("Failed to fetch executions for project",
					zap.Int("projectID", project.ID),
					zap.Error(err))
				continue
			}
			executions = append(executions, projectExecutions...)
		}
	} else {
		// 如果没有项目ID和产品ID，返回空列表
		return []vo.ExecutionVO{}, nil
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

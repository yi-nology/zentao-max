package mcp

import (
	"fmt"
	"strconv"

	"github.com/yi-nology/zentao-max/backend/core/dto"
)

// handlePing 处理 ping 请求.
func (s *MCPServer) handlePing(_ map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{
		"status":  "ok",
		"message": "Pong",
		"version": "1.0",
	}, nil
}

// handleGetProducts 获取产品列表.
func (s *MCPServer) handleGetProducts(_ map[string]interface{}) (interface{}, error) {
	result, err := s.productService.GetProducts()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"status":  "ok",
		"message": "Products retrieved successfully",
		"data":    result,
	}, nil
}

// handleGetProjects 获取项目列表.
func (s *MCPServer) handleGetProjects(params map[string]interface{}) (interface{}, error) {
	query := &dto.ProjectQueryDTO{}
	if v, ok := params["productId"]; ok {
		if id, err := strconv.Atoi(fmt.Sprintf("%v", v)); err == nil {
			query.ProductID = id
		}
	}

	result, err := s.projectService.GetProjects(query)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"status":  "ok",
		"message": "Projects retrieved successfully",
		"data":    result,
	}, nil
}

// handleGetExecutions 获取执行/迭代列表.
func (s *MCPServer) handleGetExecutions(params map[string]interface{}) (interface{}, error) {
	query := &dto.ExecutionQueryDTO{}
	if v, ok := params["projectId"]; ok {
		if id, err := strconv.Atoi(fmt.Sprintf("%v", v)); err == nil {
			query.ProjectID = id
		}
	}
	if v, ok := params["productId"]; ok {
		if id, err := strconv.Atoi(fmt.Sprintf("%v", v)); err == nil {
			query.ProductID = id
		}
	}

	result, err := s.executionService.GetExecutions(query)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"status":  "ok",
		"message": "Executions retrieved successfully",
		"data":    result,
	}, nil
}

// handleGetBugs 获取 Bug 列表.
func (s *MCPServer) handleGetBugs(params map[string]interface{}) (interface{}, error) {
	query := &dto.BugQueryDTO{}
	if v, ok := params["productId"]; ok {
		if id, err := strconv.Atoi(fmt.Sprintf("%v", v)); err == nil {
			query.ProductID = id
		}
	}
	if v, ok := params["status"]; ok {
		query.Status = fmt.Sprintf("%v", v)
	}
	query.Page = 1
	query.PageSize = 100

	result, err := s.bugService.GetBugs(query)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"status":  "ok",
		"message": "Bugs retrieved successfully",
		"data":    result,
	}, nil
}

// handleGetStories 获取需求列表.
func (s *MCPServer) handleGetStories(params map[string]interface{}) (interface{}, error) {
	query := &dto.StoryQueryDTO{}
	if v, ok := params["productId"]; ok {
		if id, err := strconv.Atoi(fmt.Sprintf("%v", v)); err == nil {
			query.ProductID = id
		}
	}
	query.Page = 1
	query.PageSize = 100

	result, err := s.storyService.GetStories(query)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"status":  "ok",
		"message": "Stories retrieved successfully",
		"data":    result,
	}, nil
}

// handleGetTasks 获取任务列表.
func (s *MCPServer) handleGetTasks(params map[string]interface{}) (interface{}, error) {
	query := &dto.TaskQueryDTO{}
	if v, ok := params["productId"]; ok {
		if id, err := strconv.Atoi(fmt.Sprintf("%v", v)); err == nil {
			query.ProductID = id
		}
	}
	if v, ok := params["executionId"]; ok {
		if id, err := strconv.Atoi(fmt.Sprintf("%v", v)); err == nil {
			query.ExecutionID = id
		}
	}
	query.Page = 1
	query.PageSize = 100

	result, err := s.taskService.GetTasks(query)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"status":  "ok",
		"message": "Tasks retrieved successfully",
		"data":    result,
	}, nil
}

// handleGetUsers 获取用户列表.
func (s *MCPServer) handleGetUsers(_ map[string]interface{}) (interface{}, error) {
	result, err := s.userService.GetUsers(1, 100)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"status":  "ok",
		"message": "Users retrieved successfully",
		"data":    result,
	}, nil
}

// handleGetTimelog 获取工时数据.
func (s *MCPServer) handleGetTimelog(params map[string]interface{}) (interface{}, error) {
	query := &dto.TimelogQueryDTO{}
	if v, ok := params["productId"]; ok {
		query.ProductID = fmt.Sprintf("%v", v)
	}
	if v, ok := params["dateFrom"]; ok {
		query.DateFrom = fmt.Sprintf("%v", v)
	}
	if v, ok := params["dateTo"]; ok {
		query.DateTo = fmt.Sprintf("%v", v)
	}

	result, err := s.timelogService.GetTimelogDashboard(query)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"status":  "ok",
		"message": "Timelog retrieved successfully",
		"data":    result,
	}, nil
}

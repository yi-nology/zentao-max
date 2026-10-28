package handlers

import (
	"strconv"

	"github.com/yi-nology/zentao-max/backend/internal/dto"
)

func (h *ProductHandler) GetProductsHTTP() (interface{}, error) {
	return h.productService.GetProducts()
}

func (h *ProjectHandler) GetProjectsHTTP(productId string) (interface{}, error) {
	query := &dto.ProjectQueryDTO{}
	if productId != "" {
		if v, err := strconv.Atoi(productId); err == nil {
			query.ProductID = v
		}
	}
	return h.projectService.GetProjects(query)
}

func (h *ExecutionHandler) GetExecutionsHTTP(projectId, productId string) (interface{}, error) {
	query := &dto.ExecutionQueryDTO{}
	if projectId != "" {
		if v, err := strconv.Atoi(projectId); err == nil {
			query.ProjectID = v
		}
	}
	if productId != "" {
		if v, err := strconv.Atoi(productId); err == nil {
			query.ProductID = v
		}
	}
	return h.executionService.GetExecutions(query)
}

func (h *BugHandler) GetBugsHTTP(productId, status string) (interface{}, error) {
	query := &dto.BugQueryDTO{}
	if productId != "" {
		if v, err := strconv.Atoi(productId); err == nil {
			query.ProductID = v
		}
	}
	query.Status = status
	query.Page = 1
	query.PageSize = 100
	return h.bugService.GetBugs(query)
}

func (h *StoryHandler) GetStoriesHTTP(productId string) (interface{}, error) {
	query := &dto.StoryQueryDTO{}
	if productId != "" {
		if v, err := strconv.Atoi(productId); err == nil {
			query.ProductID = v
		}
	}
	query.Page = 1
	query.PageSize = 100
	return h.storyService.GetStories(query)
}

func (h *TaskHandler) GetTasksHTTP(productId, executionId string) (interface{}, error) {
	query := &dto.TaskQueryDTO{}
	if productId != "" {
		if v, err := strconv.Atoi(productId); err == nil {
			query.ProductID = v
		}
	}
	if executionId != "" {
		if v, err := strconv.Atoi(executionId); err == nil {
			query.ExecutionID = v
		}
	}
	query.Page = 1
	query.PageSize = 100
	return h.taskService.GetTasks(query)
}

func (h *UserHandler) GetUsersHTTP() (interface{}, error) {
	return h.userService.GetUsers(1, 100)
}

func (h *TimelogHandler) GetTimelogHTTP(productId, dateFrom, dateTo string) (interface{}, error) {
	query := &dto.TimelogQueryDTO{
		ProductID: productId,
		DateFrom:  dateFrom,
		DateTo:    dateTo,
	}
	return h.timelogService.GetTimelogDashboard(query)
}

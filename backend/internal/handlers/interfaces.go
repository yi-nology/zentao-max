package handlers

import (
	"context"

	"github.com/yi-nology/zentao-max/backend/internal/dto"
	"github.com/yi-nology/zentao-max/backend/internal/vo"
)

// Service interfaces — handlers depend on these, not concrete types.
// The concrete *service.XxxService structs satisfy them via Go's structural typing.

type BugServicer interface {
	GetBugs(query *dto.BugQueryDTO) (*vo.PaginatedVO, error)
	GetBugStatusCounts(query *dto.BugQueryDTO) (map[string]int, error)
	GetBugsWithStatusCounts(query *dto.BugQueryDTO) (*vo.PaginatedVO, map[string]int, error)
}

type BuildServicer interface {
	GetBuildsByProject(projectID int) ([]vo.BuildVO, error)
	GetBuildsByExecution(executionID int) ([]vo.BuildVO, error)
}

type TaskServicer interface {
	GetTasks(query *dto.TaskQueryDTO) (*vo.PaginatedVO, error)
	GetTaskStatusCounts(query *dto.TaskQueryDTO) (map[string]int, error)
}

type StoryServicer interface {
	GetStories(query *dto.StoryQueryDTO) (*vo.PaginatedVO, error)
	GetStoryStatusCounts(query *dto.StoryQueryDTO) (map[string]int, error)
}

type ProductServicer interface {
	GetProducts() ([]vo.ProductVO, error)
}

type ProjectServicer interface {
	GetProjects(query *dto.ProjectQueryDTO) ([]vo.ProjectVO, error)
}

type ExecutionServicer interface {
	GetExecutions(query *dto.ExecutionQueryDTO) ([]vo.ExecutionVO, error)
}

type UserServicer interface {
	GetUsers(page, pageSize int) (*vo.PaginatedVO, error)
	GetUsersAll() ([]vo.UserVO, error)
	GetCurrentUser() (*vo.UserVO, error)
}

type TimelogServicer interface {
	GetTimelogAnalysis(query *dto.TimelogQueryDTO) (map[string]interface{}, error)
	GetTimelogDashboard(query *dto.TimelogQueryDTO) (map[string]interface{}, error)
	GetTimelogEfforts(query *dto.TimelogQueryDTO) (interface{}, error)
}

type DashboardServicer interface {
	GetDashboardContext(ctx context.Context, productID int, startDate, endDate string) (*vo.DashboardVO, error)
	GetProjectOverviewContext(ctx context.Context, projectID int) (*vo.ProjectOverviewVO, error)
	GetPersonalTimelogContext(ctx context.Context, account string, productID int, dateFrom, dateTo, groupBy string) (*vo.PersonalTimelogVO, error)
	SearchContext(ctx context.Context, keyword string, productID, page, pageSize int) (*vo.SearchVO, error)
}

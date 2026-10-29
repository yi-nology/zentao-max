package handlers

import (
	"context"

	"github.com/yi-nology/zentao-max/backend/internal/dto"
	"github.com/yi-nology/zentao-max/backend/internal/vo"
)

// Mock implementations for unit testing.
// Each mock allows configuring the return value or error.

type MockBugService struct {
	Result *vo.PaginatedVO
	Err    error
	Called bool
	Query  *dto.BugQueryDTO
}

func (m *MockBugService) GetBugs(query *dto.BugQueryDTO) (*vo.PaginatedVO, error) {
	m.Called = true
	m.Query = query
	return m.Result, m.Err
}

func (m *MockBugService) GetBugStatusCounts(query *dto.BugQueryDTO) (map[string]int, error) {
	return map[string]int{"active": 0, "resolved": 0, "closed": 0}, nil
}

func (m *MockBugService) GetBugsWithStatusCounts(query *dto.BugQueryDTO) (*vo.PaginatedVO, map[string]int, error) {
	m.Called = true
	m.Query = query
	return m.Result, map[string]int{"active": 0, "resolved": 0, "closed": 0}, m.Err
}

type MockTaskService struct {
	Result *vo.PaginatedVO
	Err    error
	Called bool
	Query  *dto.TaskQueryDTO
}

func (m *MockTaskService) GetTasks(query *dto.TaskQueryDTO) (*vo.PaginatedVO, error) {
	m.Called = true
	m.Query = query
	return m.Result, m.Err
}

func (m *MockTaskService) GetTaskStatusCounts(query *dto.TaskQueryDTO) (map[string]int, error) {
	return map[string]int{"wait": 0, "doing": 0, "done": 0, "closed": 0}, nil
}

type MockStoryService struct {
	Result *vo.PaginatedVO
	Err    error
	Called bool
}

func (m *MockStoryService) GetStories(query *dto.StoryQueryDTO) (*vo.PaginatedVO, error) {
	m.Called = true
	return m.Result, m.Err
}

func (m *MockStoryService) GetStoryStatusCounts(query *dto.StoryQueryDTO) (map[string]int, error) {
	return map[string]int{"draft": 0, "active": 0, "reviewing": 0, "changed": 0, "closed": 0}, nil
}

type MockProductService struct {
	Result []vo.ProductVO
	Err    error
	Called bool
}

func (m *MockProductService) GetProducts() ([]vo.ProductVO, error) {
	m.Called = true
	return m.Result, m.Err
}

type MockProjectService struct {
	Result []vo.ProjectVO
	Err    error
	Called bool
}

func (m *MockProjectService) GetProjects(query *dto.ProjectQueryDTO) ([]vo.ProjectVO, error) {
	m.Called = true
	return m.Result, m.Err
}

type MockExecutionService struct {
	Result []vo.ExecutionVO
	Err    error
	Called bool
}

func (m *MockExecutionService) GetExecutions(query *dto.ExecutionQueryDTO) ([]vo.ExecutionVO, error) {
	m.Called = true
	return m.Result, m.Err
}

type MockUserService struct {
	UsersResult       *vo.PaginatedVO
	UsersAllResult    []vo.UserVO
	CurrentUser       *vo.UserVO
	Err               error
	GetUsersCalled    bool
	GetUsersAllCalled bool
}

func (m *MockUserService) GetUsers(page, pageSize int) (*vo.PaginatedVO, error) {
	m.GetUsersCalled = true
	return m.UsersResult, m.Err
}

func (m *MockUserService) GetUsersAll() ([]vo.UserVO, error) {
	m.GetUsersAllCalled = true
	return m.UsersAllResult, m.Err
}

func (m *MockUserService) GetCurrentUser() (*vo.UserVO, error) {
	return m.CurrentUser, m.Err
}

type MockTimelogService struct {
	AnalysisResult map[string]interface{}
	Err            error
	Called         bool
}

func (m *MockTimelogService) GetTimelogAnalysis(query *dto.TimelogQueryDTO) (map[string]interface{}, error) {
	m.Called = true
	return m.AnalysisResult, m.Err
}

func (m *MockTimelogService) GetTimelogDashboard(query *dto.TimelogQueryDTO) (map[string]interface{}, error) {
	return m.AnalysisResult, m.Err
}

func (m *MockTimelogService) GetTimelogEfforts(query *dto.TimelogQueryDTO) (interface{}, error) {
	return m.AnalysisResult["efforts"], m.Err
}

type MockDashboardService struct {
	DashboardResult *vo.DashboardVO
	OverviewResult  *vo.ProjectOverviewVO
	TimelogResult   *vo.PersonalTimelogVO
	SearchResult    *vo.SearchVO
	Err             error
	DashboardCalled bool
	OverviewCalled  bool
	ProductIDArg    int
	StartDateArg    string
	EndDateArg      string
}

func (m *MockDashboardService) GetDashboardContext(ctx context.Context, productID int, startDate, endDate string) (*vo.DashboardVO, error) {
	m.DashboardCalled = true
	m.ProductIDArg = productID
	m.StartDateArg = startDate
	m.EndDateArg = endDate
	return m.DashboardResult, m.Err
}

func (m *MockDashboardService) GetProjectOverviewContext(ctx context.Context, projectID int) (*vo.ProjectOverviewVO, error) {
	m.OverviewCalled = true
	return m.OverviewResult, m.Err
}

func (m *MockDashboardService) GetPersonalTimelogContext(ctx context.Context, account string, productID int, dateFrom, dateTo, groupBy string) (*vo.PersonalTimelogVO, error) {
	return m.TimelogResult, m.Err
}

func (m *MockDashboardService) SearchContext(ctx context.Context, keyword string, productID, page, pageSize int) (*vo.SearchVO, error) {
	return m.SearchResult, m.Err
}

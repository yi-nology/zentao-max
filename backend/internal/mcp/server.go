package mcp

import (
	"github.com/yi-nology/zentao-max/backend/internal/service"
	myzentao "github.com/yi-nology/zentao-max/backend/internal/infra/zentao"
)

// MCPServer 是 MCP 服务的核心，直接依赖 service 层
// 统一处理所有 action，消除 stdio/HTTP 的代码重复.
type MCPServer struct {
	productService   *service.ProductService
	projectService   *service.ProjectService
	executionService *service.ExecutionService
	bugService       *service.BugService
	storyService     *service.StoryService
	taskService      *service.TaskService
	userService      *service.UserService
	timelogService   *service.TimelogService
	// client 直连句柄：写工具（record_effort 等）需要 service 未覆盖的客户端能力
	client *myzentao.Client
}

// NewMCPServer 创建 MCP 服务实例.
func NewMCPServer(client *myzentao.Client) *MCPServer {
	return &MCPServer{
		client:           client,
		productService:   service.NewProductService(client),
		projectService:   service.NewProjectService(client),
		executionService: service.NewExecutionService(client),
		bugService:       service.NewBugService(client),
		storyService:     service.NewStoryService(client),
		taskService:      service.NewTaskService(client),
		userService:      service.NewUserService(client),
		timelogService:   service.NewTimelogService(client),
	}
}

// NewMCPServerFromServices 从已有的 service 实例创建 MCP 服务.
func NewMCPServerFromServices(client *myzentao.Client,
	productService *service.ProductService,
	projectService *service.ProjectService,
	executionService *service.ExecutionService,
	bugService *service.BugService,
	storyService *service.StoryService,
	taskService *service.TaskService,
	userService *service.UserService,
	timelogService *service.TimelogService,
) *MCPServer {
	return &MCPServer{
		client:           client,
		projectService:   projectService,
		executionService: executionService,
		bugService:       bugService,
		storyService:     storyService,
		taskService:      taskService,
		userService:      userService,
		timelogService:   timelogService,
	}
}

// HandleAction 统一入口，处理所有 MCP action.
func (s *MCPServer) HandleAction(action string, params map[string]interface{}) (interface{}, error) {
	switch action {
	case "ping":
		return s.handlePing(params)
	case "get_products":
		return s.handleGetProducts(params)
	case "get_projects":
		return s.handleGetProjects(params)
	case "get_executions":
		return s.handleGetExecutions(params)
	case "get_bugs":
		return s.handleGetBugs(params)
	case "get_stories":
		return s.handleGetStories(params)
	case "get_tasks":
		return s.handleGetTasks(params)
	case "get_users":
		return s.handleGetUsers(params)
	case "get_timelog":
		return s.handleGetTimelog(params)
	case "record_effort":
		return s.handleRecordEffort(params)
	case "add_bug_comment":
		return s.handleAddBugComment(params)
	case "transition_bug":
		return s.handleTransitionBug(params)
	default:
		return nil, &ActionError{Action: action, Message: "unknown action"}
	}
}

// ActionError 未知 action 错误.
type ActionError struct {
	Action  string
	Message string
}

func (e *ActionError) Error() string {
	return e.Message + ": " + e.Action
}

// IsWriteAction 判断 action 是否为写操作（用于只读模式拦截）
// 当前 MCP 全为查询接口，写操作为未来扩展（如 create_*/update_*/delete_*）预留.
func IsWriteAction(action string) bool {
	switch action {
	case "create_product", "create_project", "create_bug", "create_story", "create_task",
		"update_product", "update_project", "update_bug", "update_story", "update_task",
		"delete_product", "delete_project", "delete_bug", "delete_story", "delete_task",
		"record_effort", "add_bug_comment", "transition_bug":
		return true
	default:
		return false
	}
}

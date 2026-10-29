package mcp

import (
	"sync/atomic"
	"time"

	"github.com/yi-nology/zentao-max/backend/internal/auth"
	myzentao "github.com/yi-nology/zentao-max/backend/internal/infra/zentao"
	"github.com/yi-nology/zentao-max/backend/internal/initialization"
	"github.com/yi-nology/zentao-max/backend/internal/service"
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
	// 基础配置/认证能力（可选注入，见 WithConfigDeps）：
	// initService 供 update_zentao_config 持久化 auth.db；ssoClient 供麒麟 SSO 短信登录两步；
	// cacheClear 供配置变更后清业务缓存（HTTP 模式注入，stdio 无业务缓存传 nil）
	initService *initialization.InitService
	ssoClient   *auth.SSOClient
	cacheClear  func() error
	// ssoLastSend 上次触发 SSO 短信发送的时间（UnixNano）；60s 冷却窗防 AI 循环重试
	// 触发 SSO 平台频控/临时封号
	ssoLastSend atomic.Int64
}

// ssoSendCooldown 两次 SSO 验证码发送的最小间隔。
const ssoSendCooldown = 60 * time.Second

// tryClaimSSOSend 冷却窗内只放行一次真实发送（CAS 抢占，并发安全）。
func (s *MCPServer) tryClaimSSOSend(nowNano int64) bool {
	last := s.ssoLastSend.Load()
	if last > 0 && nowNano-last < int64(ssoSendCooldown) {
		return false
	}
	return s.ssoLastSend.CompareAndSwap(last, nowNano)
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
		productService:   productService,
		projectService:   projectService,
		executionService: executionService,
		bugService:       bugService,
		storyService:     storyService,
		taskService:      taskService,
		userService:      userService,
		timelogService:   timelogService,
	}
}

// WithConfigDeps 注入基础配置能力（链式）：连接配置热更新的 auth.db 持久化 +
// 麒麟统一认证 SSO 登录客户端（nil = SSO 未启用，对应工具会明确报错）+
// 业务缓存清理回调（HTTP 模式传 registry 的 CacheService.ClearAll 包装，
// 保证 MCP 改配置后 Web 页面缓存不残留毒化数据；stdio 无业务缓存传 nil）。
// HTTP 与 stdio 两个入口都需要接线，否则 update_zentao_config / sso 工具不可用。
func (s *MCPServer) WithConfigDeps(initService *initialization.InitService, ssoClient *auth.SSOClient, cacheClear func() error) *MCPServer {
	s.initService = initService
	s.ssoClient = ssoClient
	s.cacheClear = cacheClear
	return s
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
	case "get_bug_detail":
		return s.handleGetBugDetail(params)
	case "get_task_detail":
		return s.handleGetTaskDetail(params)
	case "get_story_detail":
		return s.handleGetStoryDetail(params)
	case "get_my_tasks":
		return s.handleGetMyTasks(params)
	case "get_effort_calendar":
		return s.handleGetEffortCalendar(params)
	case "get_zentao_config":
		return s.handleGetZentaoConfig(params)
	case "update_zentao_config":
		return s.handleUpdateZentaoConfig(params)
	case "test_connection":
		return s.handleTestConnection(params)
	case "send_verify_code":
		return s.handleSendVerifyCode(params)
	case "sso_login":
		return s.handleSSOLogin(params)
	case "record_effort":
		return s.handleRecordEffort(params)
	case "add_bug_comment":
		return s.handleAddBugComment(params)
	case "transition_bug":
		return s.handleTransitionBug(params)
	case "create_bug":
		return s.handleCreateBug(params)
	case "transition_task":
		return s.handleTransitionTask(params)
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

// IsWriteAction 判断 action 是否为写操作（用于只读模式拦截）.
func IsWriteAction(action string) bool {
	switch action {
	case "create_product", "create_project", "create_bug", "create_story", "create_task",
		"update_product", "update_project", "update_bug", "update_story", "update_task",
		"delete_product", "delete_project", "delete_bug", "delete_story", "delete_task",
		"record_effort", "add_bug_comment", "transition_bug", "transition_task",
		"update_zentao_config", "send_verify_code", "sso_login":
		return true
	default:
		return false
	}
}

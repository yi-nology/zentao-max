package zentao

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"

	coreerrors "github.com/yi-nology/zentao-max/backend/core/errors"
)

// ─── API v1 routes ───

func GetVersion(ctx context.Context, c *app.RequestContext) {
	coreerrors.Success(c, versionInfo())
}

func HealthCheck(ctx context.Context, c *app.RequestContext) {
	healthHandler.Check(ctx, c)
}

func GetProducts(ctx context.Context, c *app.RequestContext) {
	productHandler.GetProducts(ctx, c)
}

func GetProjects(ctx context.Context, c *app.RequestContext) {
	projectHandler.GetProjects(ctx, c)
}

func GetExecutions(ctx context.Context, c *app.RequestContext) {
	executionHandler.GetExecutions(ctx, c)
}

func GetBugs(ctx context.Context, c *app.RequestContext) {
	bugHandler.GetBugs(ctx, c)
}

func GetBuildsByProject(ctx context.Context, c *app.RequestContext) {
	buildHandler.GetBuildsByProject(ctx, c)
}

func GetBuildsByExecution(ctx context.Context, c *app.RequestContext) {
	buildHandler.GetBuildsByExecution(ctx, c)
}

func GetStories(ctx context.Context, c *app.RequestContext) {
	storyHandler.GetStories(ctx, c)
}

func GetTasks(ctx context.Context, c *app.RequestContext) {
	taskHandler.GetTasks(ctx, c)
}

func GetUsers(ctx context.Context, c *app.RequestContext) {
	userHandler.GetUsers(ctx, c)
}

func GetUsersAll(ctx context.Context, c *app.RequestContext) {
	userHandler.GetUsersAll(ctx, c)
}

func GetCurrentUser(ctx context.Context, c *app.RequestContext) {
	userHandler.GetCurrentUser(ctx, c)
}

func GetTimelogAnalysis(ctx context.Context, c *app.RequestContext) {
	timelogHandler.GetTimelogAnalysis(ctx, c)
}

func GetTimelogDashboard(ctx context.Context, c *app.RequestContext) {
	timelogHandler.GetTimelogDashboard(ctx, c)
}

func GetTimelogEfforts(ctx context.Context, c *app.RequestContext) {
	timelogHandler.GetTimelogEfforts(ctx, c)
}

func GetEffortCalendar(ctx context.Context, c *app.RequestContext) {
	effortCalendarHandler.GetEffortCalendar(ctx, c)
}

func GetDashboard(ctx context.Context, c *app.RequestContext) {
	dashboardHandler.GetDashboard(ctx, c)
}

func GetProjectOverview(ctx context.Context, c *app.RequestContext) {
	dashboardHandler.GetProjectOverview(ctx, c)
}

func GetPersonalTimelog(ctx context.Context, c *app.RequestContext) {
	dashboardHandler.GetPersonalTimelog(ctx, c)
}

func Search(ctx context.Context, c *app.RequestContext) {
	dashboardHandler.Search(ctx, c)
}

// ─── Init routes ───

func GetInitStatus(ctx context.Context, c *app.RequestContext) {
	initHandler.GetInitStatus(ctx, c)
}

func GetAccountInfo(ctx context.Context, c *app.RequestContext) {
	initHandler.GetAccountInfo(ctx, c)
}

// ─── Scheduler routes ───

func ListSchedulerTasks(ctx context.Context, c *app.RequestContext) {
	schedulerHandler.ListTasks(ctx, c)
}

func CreateSchedulerTask(ctx context.Context, c *app.RequestContext) {
	schedulerHandler.CreateTask(ctx, c)
}

func UpdateSchedulerTask(ctx context.Context, c *app.RequestContext) {
	schedulerHandler.UpdateTask(ctx, c)
}

func DeleteSchedulerTask(ctx context.Context, c *app.RequestContext) {
	schedulerHandler.DeleteTask(ctx, c)
}

func ToggleSchedulerTask(ctx context.Context, c *app.RequestContext) {
	schedulerHandler.ToggleTask(ctx, c)
}

func RunSchedulerTaskNow(ctx context.Context, c *app.RequestContext) {
	schedulerHandler.RunTaskNow(ctx, c)
}

func GetSchedulerTaskLogs(ctx context.Context, c *app.RequestContext) {
	schedulerHandler.GetTaskLogs(ctx, c)
}

func GetSchedulerAllLogs(ctx context.Context, c *app.RequestContext) {
	schedulerHandler.GetAllLogs(ctx, c)
}

func TestSchedulerWebhook(ctx context.Context, c *app.RequestContext) {
	schedulerHandler.TestWebhook(ctx, c)
}

func PreviewSchedulerReport(ctx context.Context, c *app.RequestContext) {
	schedulerHandler.PreviewReport(ctx, c)
}

// ─── MCP routes ───

func MCPHandleAction(ctx context.Context, c *app.RequestContext) {
	mcpHTTPTransport.HandleAction(ctx, c)
}

func MCPHandleActionGet(ctx context.Context, c *app.RequestContext) {
	mcpHTTPTransport.HandleActionGet(ctx, c)
}

func MCPHandleListTools(ctx context.Context, c *app.RequestContext) {
	mcpHTTPTransport.HandleListTools(ctx, c)
}

func MCPHandleGetTool(ctx context.Context, c *app.RequestContext) {
	mcpHTTPTransport.HandleGetTool(ctx, c)
}

func MCPPing(ctx context.Context, c *app.RequestContext) {
	mcpHTTPTransport.HandleActionByName(ctx, "ping", c)
}

func MCPGetProducts(ctx context.Context, c *app.RequestContext) {
	mcpHTTPTransport.HandleActionByName(ctx, "get_products", c)
}

func MCPGetProjects(ctx context.Context, c *app.RequestContext) {
	mcpHTTPTransport.HandleActionByName(ctx, "get_projects", c)
}

func MCPGetExecutions(ctx context.Context, c *app.RequestContext) {
	mcpHTTPTransport.HandleActionByName(ctx, "get_executions", c)
}

func MCPGetBugs(ctx context.Context, c *app.RequestContext) {
	mcpHTTPTransport.HandleActionByName(ctx, "get_bugs", c)
}

func MCPGetStories(ctx context.Context, c *app.RequestContext) {
	mcpHTTPTransport.HandleActionByName(ctx, "get_stories", c)
}

func MCPGetTasks(ctx context.Context, c *app.RequestContext) {
	mcpHTTPTransport.HandleActionByName(ctx, "get_tasks", c)
}

func MCPGetUsers(ctx context.Context, c *app.RequestContext) {
	mcpHTTPTransport.HandleActionByName(ctx, "get_users", c)
}

func MCPGetTimelog(ctx context.Context, c *app.RequestContext) {
	mcpHTTPTransport.HandleActionByName(ctx, "get_timelog", c)
}

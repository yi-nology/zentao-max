package handlers

import (
	"log"

	"github.com/yi-nology/zentao-max/backend/internal/initialization"
	"github.com/yi-nology/zentao-max/backend/internal/service"
	"github.com/yi-nology/zentao-max/backend/internal/repository/sqlite"
	myzentao "github.com/yi-nology/zentao-max/backend/internal/infra/zentao"
)

// HandlerRegistry Handler注册表
// 使用单例模式确保所有handler和service只初始化一次，避免重复创建
// 这是依赖注入模式的核心组件，提供统一的handler访问入口
type HandlerRegistry struct {
	client *myzentao.Client

	productService   *service.ProductService
	projectService   *service.ProjectService
	executionService *service.ExecutionService
	bugService       *service.BugService
	buildService     *service.BuildService
	storyService     *service.StoryService
	taskService      *service.TaskService
	userService      *service.UserService
	timelogService   *service.TimelogService
	dashboardService *service.DashboardService
	reportService    *service.ReportService
	webhookService   *service.WebhookService
	schedulerService *service.SchedulerService
	cacheService     *service.CacheService
	cacheStore       storage.Store

	// Phase2b 新增实体
	caseService     *service.CaseService
	planService     *service.PlanService
	programService  *service.ProgramService
	releaseService  *service.ReleaseService
	testTaskService *service.TestTaskService

	productHandler   *ProductHandler
	projectHandler   *ProjectHandler
	executionHandler *ExecutionHandler
	bugHandler       *BugHandler
	buildHandler     *BuildHandler
	storyHandler     *StoryHandler
	taskHandler      *TaskHandler
	userHandler      *UserHandler
	timelogHandler   *TimelogHandler
	dashboardHandler *DashboardHandler
	schedulerHandler *SchedulerHandler
	logHandler       *LogHandler
	cacheHandler     *CacheHandler
	initHandler      *InitHandler
	healthHandler    *HealthHandler

	// effort-calendar
	effortCalendarService *service.EffortCalendarService
	effortCalendarHandler *EffortCalendarHandler

	// effort 批量创建（受限实例 batchCreate 端点转发）
	effortBatchHandler *EffortBatchHandler

	// daily-note（工时日历的"这天我干了什么"）
	dailyNoteStore   *myzentao.DailyNoteStore
	dailyNoteHandler *DailyNoteHandler

	// Phase2b 新增 handler
	caseHandler     *CaseHandler
	planHandler     *PlanHandler
	programHandler  *ProgramHandler
	releaseHandler  *ReleaseHandler
	testTaskHandler *TestTaskHandler

	// Phase2c 写操作 handler
	writeHandler *WriteHandler
}

// NewHandlerRegistry 创建Handler注册表
// 所有service和handler在此处统一初始化，确保整个应用生命周期内只创建一次
// 采用分层架构：Client -> Service -> Handler
func NewHandlerRegistry(client *myzentao.Client, initService *initialization.InitService) *HandlerRegistry {
	registry := &HandlerRegistry{
		client: client,
	}

	registry.productService = service.NewProductService(client)
	registry.projectService = service.NewProjectService(client)
	registry.executionService = service.NewExecutionService(client)
	registry.bugService = service.NewBugService(client)
	registry.buildService = service.NewBuildService(client)
	registry.storyService = service.NewStoryService(client)
	registry.taskService = service.NewTaskService(client)
	registry.userService = service.NewUserService(client)
	registry.timelogService = service.NewTimelogService(client)
	registry.dashboardService = service.NewDashboardService(client)

	// Phase2b 新增实体 service
	registry.caseService = service.NewCaseService(client)
	registry.planService = service.NewPlanService(client)
	registry.programService = service.NewProgramService(client)
	registry.releaseService = service.NewReleaseService(client)
	registry.testTaskService = service.NewTestTaskService(client)

	registry.productHandler = NewProductHandler(registry.productService)
	registry.projectHandler = NewProjectHandler(registry.projectService)
	registry.executionHandler = NewExecutionHandler(registry.executionService)
	registry.bugHandler = NewBugHandler(registry.bugService)
	registry.buildHandler = NewBuildHandler(registry.buildService)
	registry.storyHandler = NewStoryHandler(registry.storyService)
	registry.taskHandler = NewTaskHandler(registry.taskService)
	registry.userHandler = NewUserHandler(registry.userService)
	registry.timelogHandler = NewTimelogHandler(registry.timelogService)
	registry.dashboardHandler = NewDashboardHandler(registry.dashboardService)

	// Phase2b 新增 handler
	registry.caseHandler = NewCaseHandler(registry.caseService)
	registry.planHandler = NewPlanHandler(registry.planService)
	registry.programHandler = NewProgramHandler(registry.programService)
	registry.releaseHandler = NewReleaseHandler(registry.releaseService)
	registry.testTaskHandler = NewTestTaskHandler(registry.testTaskService)

	// Phase2c 写操作 handler（直接依赖 client，不经 service）
	registry.writeHandler = NewWriteHandler(client)

	registry.logHandler = NewLogHandler()

	// effort-calendar
	registry.effortCalendarService = service.NewEffortCalendarService(client)
	registry.effortCalendarHandler = NewEffortCalendarHandler(registry.effortCalendarService)

	// effort 批量创建
	registry.effortBatchHandler = NewEffortBatchHandler(client)

	// daily-note：构造 SQLite，失败时降级（store=nil 时路由不注册）。
	// 用空 prefix → 走 NewDailyNoteStore 默认路径 ~/.zentao-max/cache-notes.db。
	if store, err := myzentao.NewDailyNoteStore(""); err != nil {
		log.Printf("Warning: failed to init daily note store: %v (daily note disabled)", err)
	} else {
		registry.dailyNoteStore = store
		// service 包装:写时同步到 zentao effort，失败降级本地。
		// 注意 service 依赖 client（zentao session），client 早于 dailyNoteStore 构造。
		dailyNoteService := service.NewDailyNoteService(store, client)
		registry.dailyNoteHandler = NewDailyNoteHandler(store, dailyNoteService)
	}

	// 初始化 SQLite 缓存层（离线模式用）
	cacheStore, err := storage.NewSQLiteStore("")
	if err != nil {
		log.Printf("Warning: failed to init SQLite cache store: %v (offline mode disabled)", err)
	} else {
		registry.cacheStore = cacheStore
		registry.cacheService = service.NewCacheService(cacheStore)
		registry.cacheHandler = NewCacheHandler(registry.cacheService)
		// 把缓存注入到 DashboardService（支持离线查看仪表盘）
		registry.dashboardService.SetCacheService(registry.cacheService)
	}

	registry.initHandler = NewInitHandler(initService, client, registry.cacheService)

	registry.healthHandler = NewHealthHandler(
		client,
		registry.productService,
		registry.projectService,
		registry.bugService,
		registry.storyService,
		registry.taskService,
		registry.userService,
		nil,
	)

	return registry
}

func (r *HandlerRegistry) InitScheduler(store *initialization.ConfigStore) {
	r.reportService = service.NewReportService(r.client)
	r.webhookService = service.NewWebhookService()
	r.schedulerService = service.NewSchedulerService(store, r.reportService, r.webhookService)
	r.schedulerHandler = NewSchedulerHandler(r.schedulerService, r.webhookService, r.reportService)
	r.healthHandler.SetSchedulerService(r.schedulerService)
	if err := r.schedulerService.Start(); err != nil {
		log.Printf("Failed to start scheduler: %v", err)
	}
}

func (r *HandlerRegistry) StopScheduler() {
	if r.schedulerService != nil {
		r.schedulerService.Stop()
	}
}

// CloseCache 关闭缓存存储（应用退出时调用）
func (r *HandlerRegistry) CloseCache() {
	if r.cacheStore != nil {
		if err := r.cacheStore.Close(); err != nil {
			log.Printf("Failed to close cache store: %v", err)
		}
	}
}

// GetProductHandler 获取产品Handler
func (r *HandlerRegistry) GetProductHandler() *ProductHandler {
	return r.productHandler
}

// GetProjectHandler 获取项目Handler
func (r *HandlerRegistry) GetProjectHandler() *ProjectHandler {
	return r.projectHandler
}

// GetExecutionHandler 获取执行Handler
func (r *HandlerRegistry) GetExecutionHandler() *ExecutionHandler {
	return r.executionHandler
}

// GetBugHandler 获取Bug Handler
func (r *HandlerRegistry) GetBugHandler() *BugHandler {
	return r.bugHandler
}

// GetBuildHandler 获取版本 Handler
func (r *HandlerRegistry) GetBuildHandler() *BuildHandler {
	return r.buildHandler
}

// GetStoryHandler 获取需求Handler
func (r *HandlerRegistry) GetStoryHandler() *StoryHandler {
	return r.storyHandler
}

// GetTaskHandler 获取任务Handler
func (r *HandlerRegistry) GetTaskHandler() *TaskHandler {
	return r.taskHandler
}

// GetUserHandler 获取用户Handler
func (r *HandlerRegistry) GetUserHandler() *UserHandler {
	return r.userHandler
}

// GetTimelogHandler 获取工时Handler
func (r *HandlerRegistry) GetTimelogHandler() *TimelogHandler {
	return r.timelogHandler
}

// GetProductService 获取产品 Service
func (r *HandlerRegistry) GetProductService() *service.ProductService { return r.productService }

// GetProjectService 获取项目 Service
func (r *HandlerRegistry) GetProjectService() *service.ProjectService { return r.projectService }

// GetExecutionService 获取执行 Service
func (r *HandlerRegistry) GetExecutionService() *service.ExecutionService { return r.executionService }

// GetBugService 获取Bug Service
func (r *HandlerRegistry) GetBugService() *service.BugService { return r.bugService }

// GetBuildService 获取版本 Service
func (r *HandlerRegistry) GetBuildService() *service.BuildService { return r.buildService }

// GetStoryService 获取需求 Service
func (r *HandlerRegistry) GetStoryService() *service.StoryService { return r.storyService }

// GetTaskService 获取任务 Service
func (r *HandlerRegistry) GetTaskService() *service.TaskService { return r.taskService }

// GetUserService 获取用户 Service
func (r *HandlerRegistry) GetUserService() *service.UserService { return r.userService }

// GetTimelogService 获取工时 Service
func (r *HandlerRegistry) GetTimelogService() *service.TimelogService { return r.timelogService }

// GetInitHandler 获取初始化Handler
func (r *HandlerRegistry) GetInitHandler() *InitHandler {
	return r.initHandler
}

// GetLogHandler 获取日志 Handler
func (r *HandlerRegistry) GetLogHandler() *LogHandler {
	return r.logHandler
}

// GetCacheHandler 获取缓存 Handler（可能为 nil，当 SQLite 初始化失败时）
func (r *HandlerRegistry) GetCacheHandler() *CacheHandler {
	return r.cacheHandler
}

func (r *HandlerRegistry) GetDashboardHandler() *DashboardHandler {
	return r.dashboardHandler
}

func (r *HandlerRegistry) GetSchedulerHandler() *SchedulerHandler {
	return r.schedulerHandler
}

func (r *HandlerRegistry) GetHealthHandler() *HealthHandler {
	return r.healthHandler
}

// Phase2b 新增 handler 访问器
func (r *HandlerRegistry) GetCaseHandler() *CaseHandler         { return r.caseHandler }
func (r *HandlerRegistry) GetPlanHandler() *PlanHandler         { return r.planHandler }
func (r *HandlerRegistry) GetProgramHandler() *ProgramHandler   { return r.programHandler }
func (r *HandlerRegistry) GetReleaseHandler() *ReleaseHandler   { return r.releaseHandler }
func (r *HandlerRegistry) GetTestTaskHandler() *TestTaskHandler { return r.testTaskHandler }

// Phase2c 写操作 handler 访问器
func (r *HandlerRegistry) GetWriteHandler() *WriteHandler { return r.writeHandler }

// GetEffortCalendarHandler 获取工时日历 Handler
func (r *HandlerRegistry) GetEffortCalendarHandler() *EffortCalendarHandler {
	return r.effortCalendarHandler
}

// GetEffortBatchHandler 获取工时批量创建 Handler
func (r *HandlerRegistry) GetEffortBatchHandler() *EffortBatchHandler {
	return r.effortBatchHandler
}

// GetDailyNoteHandler 获取 daily note Handler（store 初始化失败时为 nil）。
func (r *HandlerRegistry) GetDailyNoteHandler() *DailyNoteHandler {
	return r.dailyNoteHandler
}

// CloseDailyNote 关闭 daily note store（应用退出时调用）。
func (r *HandlerRegistry) CloseDailyNote() {
	if r.dailyNoteStore != nil {
		_ = r.dailyNoteStore.Close()
	}
}

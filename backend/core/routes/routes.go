package routes

import (
	"context"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/route"
	
	bizhandler "github.com/yi-nology/zentao-max/backend/biz/handler/zentao"
	bizrouter "github.com/yi-nology/zentao-max/backend/biz/router"
	"github.com/yi-nology/zentao-max/backend/core/auth"
	"github.com/yi-nology/zentao-max/backend/core/config"
	"github.com/yi-nology/zentao-max/backend/core/errors"
	"github.com/yi-nology/zentao-max/backend/core/handlers"
	"github.com/yi-nology/zentao-max/backend/core/initialization"
	"github.com/yi-nology/zentao-max/backend/core/logger"
	"github.com/yi-nology/zentao-max/backend/core/mcp"
	"github.com/yi-nology/zentao-max/backend/core/metrics"
	"github.com/yi-nology/zentao-max/backend/core/middleware"
	"github.com/yi-nology/zentao-max/backend/core/utils"
	"github.com/yi-nology/zentao-max/backend/core/zentao"
)

func SetupRouter(initService *initialization.InitService, zentaoClient *zentao.Client) *server.Hertz {
	registry := handlers.NewHandlerRegistry(zentaoClient, initService)
	return SetupRouterWithHandlers(initService, zentaoClient, registry, ":12345", nil)
}

func SetupRouterWithHandlers(initService *initialization.InitService, zentaoClient *zentao.Client, registry *handlers.HandlerRegistry, hostPort string, staticFS http.FileSystem) *server.Hertz {
	hertzServer := server.New(server.WithHostPorts(hostPort))

	// 平台访问控制：匿名只读（GET），登录管理员读写
	cfg := config.Get()
	authManager := auth.NewManager(
		cfg.Auth.Admin.Username,
		cfg.Auth.Admin.Password,
		auth.SecretFromEncryptionKey(cfg.Security.EncryptionKey),
	)
	// SSO 登录（麒麟统一认证，用户名+密码+短信验证码）
	if cfg.Auth.SSO.Enabled {
		authManager.SetSSO(auth.NewSSOClient(cfg.Auth.SSO.BaseURL))
	}
	hertzServer.Use(middleware.RecoveryMiddleware())
	hertzServer.Use(middleware.TraceIDMiddleware())
	hertzServer.Use(middleware.LoggerMiddleware())
	hertzServer.Use(middleware.MetricsMiddleware())
	// 限流参数来自配置文件的 rate_limit 段（未配置时回退内置默认 600/分钟）
	rlCfg := config.Get().RateLimit
	hertzServer.Use(errors.RateLimitMiddlewareWithConfig(rlCfg.RequestsPerMinute, rlCfg.BlockDurationMinutes))
	hertzServer.Use(errors.CORSMiddleware())
	// 认证中间件放在 CORS 之后，保证 401 响应也带 CORS 头
	hertzServer.Use(authManager.Middleware())
	hertzServer.Use(utils.PaginationMiddleware())

	registerAuthRoutes(hertzServer, authManager)

	mcpServer := mcp.NewMCPServerFromServices(
		zentaoClient,
		registry.GetProductService(),
		registry.GetProjectService(),
		registry.GetExecutionService(),
		registry.GetBugService(),
		registry.GetStoryService(),
		registry.GetTaskService(),
		registry.GetUserService(),
		registry.GetTimelogService(),
	)
	mcpTransport := mcp.NewHTTPTransport(mcpServer)

	// 初始化 MCP 模式管理器：从配置加载，HTTP 入口标记传输模式为 http
	mcpCfg := config.Get().MCP
	mcp.GetMCPModeManager().InitFromConfig(mcpCfg)
	mcp.GetMCPModeManager().SetTransport(mcp.TransportHTTP)

	// both 模式：HTTP 入口额外启动后台 stdio 监听，与 HTTP 共享同一 MCPServer
	if mcpCfg.Transport == mcp.TransportBoth {
		mcp.NewStdioTransport(mcpServer).Start()
		logger.Info("MCP stdio transport started in 'both' mode")
	}

	bizhandler.Init(registry, mcpTransport)

	bizrouter.GeneratedRegister(hertzServer)

	registerCustomRoutes(hertzServer, registry, mcpTransport)

	if staticFS != nil {
		hertzServer.NoRoute(func(ctx context.Context, c *app.RequestContext) {
			filePath := string(c.Path())
			if filePath == "/" {
				filePath = "/index.html"
			}

			f, err := staticFS.Open(filePath)
			if err == nil {
				if stat, statErr := f.Stat(); statErr != nil || stat.IsDir() {
					// 目录请求回退到 index.html
					_ = f.Close()
					f, err = staticFS.Open("/index.html")
					filePath = "/index.html"
				}
			} else {
				// SPA fallback: serve index.html for non-file routes
				f, err = staticFS.Open("/index.html")
				filePath = "/index.html"
			}
			if err != nil {
				c.SetStatusCode(404)
				return
			}
			defer func() { _ = f.Close() }()

			stat, err := f.Stat()
			if err != nil {
				c.SetStatusCode(404)
				return
			}

			data := make([]byte, stat.Size())
			// io.ReadFull：单次 Read 不保证读满，截断会导致页面/资源静默损坏
			if _, err := io.ReadFull(f, data); err != nil {
				logger.Error("读取静态文件失败", "path", filePath, "error", err)
				c.SetStatusCode(500)
				return
			}

			// Content-Type 按实际返回的文件推断：SPA fallback 回落到 index.html 时
			// 请求路径无扩展名，若沿用请求路径会得到 application/octet-stream，浏览器会把页面当文件下载
			if ct := getContentType(getExt(filePath)); ct != "" {
				c.Header("Content-Type", ct)
			}
			c.Header("Cache-Control", "public, max-age=3600")
			c.SetStatusCode(200)
			_, _ = c.Write(data)
		})
		logger.Info("Static file system mounted")
	}

	logger.Info("Router setup completed")

	return hertzServer
}

func registerCustomRoutes(r *server.Hertz, registry *handlers.HandlerRegistry, transport *mcp.HTTPTransport) {
	r.GET("/health", func(ctx context.Context, c *app.RequestContext) {
		errors.Success(c, map[string]interface{}{
			"status":  "ok",
			"message": "zentao-max backend is running",
		})
	})

	r.GET("/metrics", metrics.Handler())

	// Effort 批量创建（受限实例 batchCreate 端点的 server-side 转发）：
	// 走 user 真实 curl 验证的 form 格式（id[]/work[]/objectType[]/execution[N]/consumed[]/left[N]），
	// 端点是 /effort-batchCreate-{date}.html?onlybody=yes。
	apiV1 := r.Group("/api/v1")
	apiV1.POST("/efforts/batch", registry.GetEffortBatchHandler().Create)

	registerBackwardCompatRoutes(r, registry)
	registerMCPPostRoutes(r, transport)
	registerMCPAdminRoutes(r)
}

// registerAuthRoutes 注册平台认证路由（登录/登出/状态）
func registerAuthRoutes(r *server.Hertz, authManager *auth.Manager) {
	api := r.Group("/api")
	authManager.RegisterRoutes(api)
}

// registerMCPAdminRoutes 注册 MCP 运行时管理 API（热重载 / 状态查询）
func registerMCPAdminRoutes(r *server.Hertz) {
	admin := mcp.NewMCPAdminHandler()
	api := r.Group("/api/v1")
	admin.RegisterAdminRoutes(api)
}

func registerBackwardCompatRoutes(r *server.Hertz, registry *handlers.HandlerRegistry) {
	api := r.Group("/api")

	api.GET("/init/account", bizhandler.GetAccountInfo)

	registerDomainRoutes(api)
	registerSchedulerRoutes(api, registry.GetSchedulerHandler())

	// Phase2b 新增实体路由（直接经 registry，不走 biz forwarder，保持简单）
	api.GET("/cases", registry.GetCaseHandler().GetCases)
	api.GET("/plans", registry.GetPlanHandler().GetPlans)
	api.GET("/programs", registry.GetProgramHandler().GetPrograms)
	api.GET("/releases", registry.GetReleaseHandler().GetReleases)
	api.GET("/testtasks", registry.GetTestTaskHandler().GetTestTasks)

	// Phase2c 写操作路由
	wh := registry.GetWriteHandler()
	// Bug
	api.POST("/bugs", wh.CreateBug)
	api.PUT("/bugs/:id", wh.UpdateBug)
	api.DELETE("/bugs/:id", wh.DeleteBug)
	api.POST("/bugs/:id/resolve", wh.ResolveBug)
	api.POST("/bugs/:id/close", wh.CloseBug)
	api.POST("/bugs/:id/assign", wh.AssignBug)
	api.POST("/bugs/:id/confirm", wh.ConfirmBug)
	api.POST("/bugs/:id/activate", wh.ActivateBug)
	// Task
	api.POST("/tasks", wh.CreateTask)
	api.PUT("/tasks/:id", wh.UpdateTask)
	api.DELETE("/tasks/:id", wh.DeleteTask)
	api.POST("/tasks/:id/start", wh.StartTask)
	api.POST("/tasks/:id/finish", wh.FinishTask)
	api.POST("/tasks/:id/pause", wh.PauseTask)
	api.POST("/tasks/:id/assign", wh.AssignTask)
	api.POST("/tasks/:id/activate", wh.ActivateTask)
	api.POST("/tasks/:id/effort", wh.RecordEffort)
	// Story
	api.POST("/stories", wh.CreateStory)
	api.PUT("/stories/:id", wh.UpdateStory)
	api.DELETE("/stories/:id", wh.DeleteStory)
	api.POST("/stories/:id/change", wh.ChangeStory)
	// Plan
	api.POST("/plans", wh.CreatePlan)
	api.PUT("/plans/:id", wh.UpdatePlan)
	api.DELETE("/plans/:id", wh.DeletePlan)
	api.POST("/plans/:id/link-stories", wh.LinkStoriesToPlan)
	api.POST("/plans/:id/unlink-stories", wh.UnlinkStoriesFromPlan)
	api.POST("/plans/:id/link-bugs", wh.LinkBugsToPlan)
	api.POST("/plans/:id/unlink-bugs", wh.UnlinkBugsFromPlan)
	// Case
	api.POST("/cases", wh.CreateCase)
	api.PUT("/cases/:id", wh.UpdateCase)
	api.DELETE("/cases/:id", wh.DeleteCase)
	// Product / Project / Program / Execution / Build / User CRUD
	api.POST("/products", wh.CreateProduct)
	api.PUT("/products/:id", wh.UpdateProduct)
	api.DELETE("/products/:id", wh.DeleteProduct)
	api.POST("/projects", wh.CreateProject)
	api.PUT("/projects/:id", wh.UpdateProject)
	api.DELETE("/projects/:id", wh.DeleteProject)
	api.POST("/programs", wh.CreateProgram)
	api.PUT("/programs/:id", wh.UpdateProgram)
	api.DELETE("/programs/:id", wh.DeleteProgram)
	api.POST("/executions", wh.CreateExecution)
	api.PUT("/executions/:id", wh.UpdateExecution)
	api.DELETE("/executions/:id", wh.DeleteExecution)
	api.POST("/builds", wh.CreateBuild)
	api.PUT("/builds/:id", wh.UpdateBuild)
	api.DELETE("/builds/:id", wh.DeleteBuild)
	api.POST("/users", wh.CreateUser)
	api.PUT("/users/:id", wh.UpdateUser)
	api.DELETE("/users/:id", wh.DeleteUser)

	// 日志查看接口（供前端日志页消费）
	logHandler := registry.GetLogHandler()
	api.GET("/logs", logHandler.GetLogs)
	api.DELETE("/logs", logHandler.ClearLogs)
	api.GET("/logs/status", logHandler.LogsStatus)

	// 离线缓存管理接口（缓存初始化失败时跳过注册）
	if cacheH := registry.GetCacheHandler(); cacheH != nil {
		api.GET("/cache/status", cacheH.GetStatus)
		api.DELETE("/cache", cacheH.ClearAll)
		api.DELETE("/cache/:entityType", cacheH.Invalidate)
	}

	// daily note（工时日历"这天我干了什么"）
	if noteH := registry.GetDailyNoteHandler(); noteH != nil {
		api.GET("/effort/notes", noteH.ListNotes)
		api.GET("/effort/notes/:user/:date", noteH.GetNote)
		api.PUT("/effort/notes/:user/:date", noteH.PutNote)
		api.DELETE("/effort/notes/:user/:date", noteH.DeleteNote)
	}
}

func registerDomainRoutes(g *route.RouterGroup) {
	g.GET("/products", bizhandler.GetProducts)
	g.GET("/projects", bizhandler.GetProjects)
	g.GET("/executions", bizhandler.GetExecutions)
	g.GET("/bugs", bizhandler.GetBugs)
	g.GET("/builds/project", bizhandler.GetBuildsByProject)
	g.GET("/builds/execution", bizhandler.GetBuildsByExecution)
	g.GET("/stories", bizhandler.GetStories)
	g.GET("/tasks", bizhandler.GetTasks)
	g.GET("/users", bizhandler.GetUsers)
	g.GET("/users/all", bizhandler.GetUsersAll)
	g.GET("/users/current", bizhandler.GetCurrentUser)
	g.GET("/timelog/analysis", bizhandler.GetTimelogAnalysis)
	g.GET("/timelog/dashboard", bizhandler.GetTimelogDashboard)
	g.GET("/timelog/efforts", bizhandler.GetTimelogEfforts)
	g.GET("/effort-calendar", bizhandler.GetEffortCalendar)
	g.GET("/dashboard", bizhandler.GetDashboard)
	g.GET("/project/overview", bizhandler.GetProjectOverview)
	g.GET("/personal/timelog", bizhandler.GetPersonalTimelog)
	g.GET("/search", bizhandler.Search)
}

func registerSchedulerRoutes(g *route.RouterGroup, h *handlers.SchedulerHandler) {
	scheduler := g.Group("/scheduler")
	scheduler.GET("/tasks", h.ListTasks)
	scheduler.POST("/tasks", h.CreateTask)
	scheduler.PUT("/tasks/:id", h.UpdateTask)
	scheduler.DELETE("/tasks/:id", h.DeleteTask)
	scheduler.PATCH("/tasks/:id/toggle", h.ToggleTask)
	scheduler.POST("/tasks/:id/run", h.RunTaskNow)
	scheduler.GET("/tasks/:id/logs", h.GetTaskLogs)
	scheduler.GET("/logs", h.GetAllLogs)
	scheduler.POST("/test-webhook", h.TestWebhook)
	scheduler.POST("/preview", h.PreviewReport)
}

func registerMCPPostRoutes(r *server.Hertz, transport *mcp.HTTPTransport) {
	actionMap := map[string]string{
		"ping":       "ping",
		"products":   "get_products",
		"projects":   "get_projects",
		"executions": "get_executions",
		"bugs":       "get_bugs",
		"stories":    "get_stories",
		"tasks":      "get_tasks",
		"users":      "get_users",
		"timelog":    "get_timelog",
	}

	for path, action := range actionMap {
		act := action
		r.POST("/mcp/"+path, func(ctx context.Context, c *app.RequestContext) {
			transport.HandleActionByName(ctx, act, c)
		})
	}
}

func getExt(filePath string) string {
	return strings.ToLower(path.Ext(filePath))
}

func getContentType(ext string) string {
	types := map[string]string{
		".html":  "text/html; charset=utf-8",
		".css":   "text/css; charset=utf-8",
		".js":    "application/javascript; charset=utf-8",
		".json":  "application/json",
		".png":   "image/png",
		".jpg":   "image/jpeg",
		".jpeg":  "image/jpeg",
		".gif":   "image/gif",
		".svg":   "image/svg+xml",
		".ico":   "image/x-icon",
		".woff":  "font/woff",
		".woff2": "font/woff2",
		".ttf":   "font/ttf",
		".eot":   "application/vnd.ms-fontobject",
		".map":   "application/json",
	}
	if ct, ok := types[ext]; ok {
		return ct
	}
	return "application/octet-stream"
}

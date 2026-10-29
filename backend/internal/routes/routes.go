package routes

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/hertz-contrib/gzip"
	"github.com/oekit/middleware"
	"github.com/oekit/middleware/hmw"

	bizhandler "github.com/yi-nology/zentao-max/backend/biz/handler/zentao"
	bizrouter "github.com/yi-nology/zentao-max/backend/biz/router"
	"github.com/yi-nology/zentao-max/backend/internal/auth"
	"github.com/yi-nology/zentao-max/backend/internal/config"
	"github.com/yi-nology/zentao-max/backend/internal/handlers"
	"github.com/yi-nology/zentao-max/backend/internal/infra/zentao"
	"github.com/yi-nology/zentao-max/backend/internal/initialization"
	"github.com/yi-nology/zentao-max/backend/internal/mcp"
	versionpkg "github.com/yi-nology/zentao-max/backend/internal/version"
	"github.com/yi-nology/zentao-max/backend/internal/update"
	"github.com/yi-nology/zentao-max/backend/internal/middleware"
	"github.com/yi-nology/zentao-max/backend/pkg/errors"
	"github.com/yi-nology/zentao-max/backend/pkg/logger"
	"github.com/yi-nology/zentao-max/backend/pkg/metrics"
	"github.com/yi-nology/zentao-max/backend/pkg/utils"
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
	// SSO 登录（统一认证平台，用户名+密码+短信验证码）；客户端同时供 MCP sso_* 工具使用。
	// 平台地址全部来自配置（auth.sso.base_url/auth_page_url），未配置时 SSO 入口自动关闭
	var ssoClient *auth.SSOClient
	if cfg.Auth.SSO.Enabled {
		if strings.TrimSpace(cfg.Auth.SSO.BaseURL) == "" {
			logger.GetLogger().Warn("auth.sso.enabled=true 但未配置 auth.sso.base_url，SSO 登录入口不可用",
				"config_key", "auth.sso.base_url")
		} else {
			ssoClient = auth.NewSSOClient(cfg.Auth.SSO.BaseURL, cfg.Auth.SSO.AuthPageURL)
			authManager.SetSSO(ssoClient)
		}
	}
	// panic 防护与 trace_id 由 oekit/middleware 提供；Recovery 的响应体保持
	// 项目 errors.Response 结构（前端按 code/message/data 解析）
	hertzServer.Use(mw.RecoveryMiddleware(mw.WithRecoveryErrorHandler(func(c *app.RequestContext, _ any, _ string) {
		c.JSON(500, errors.Response{
			Code:    errors.CodeInternalError,
			Message: "服务器内部错误",
			Data:    nil,
		})
	})))
	hertzServer.Use(hmw.Middleware()...)
	// 访问日志由 oekit AccessLog 承担（trace_id 与 hmw 集成），/metrics 与
	// /health 高频探活路径跳过降噪
	hertzServer.Use(mw.AccessLogMiddleware(
		mw.WithAccessLogLogger(logger.GetLogger()),
		mw.WithAccessLogSkipPaths("/metrics", "/health"),
	))
	hertzServer.Use(middleware.MetricsMiddleware())
	// API/静态资源 gzip 压缩（前端 JS bundle 与列表接口收益明显）；
	// /metrics 排除——promhttp 自带压缩协商，双层会冲突
	hertzServer.Use(gzip.Gzip(gzip.DefaultCompression,
		gzip.WithExcludedPaths([]string{"/metrics"})))
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
	).WithConfigDeps(initService, ssoClient, func() error {
		// MCP 改配置后清业务缓存（与设置页同语义）
		return registry.ClearAllCaches(context.Background())
	})
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
			// 缓存策略：assets/* 文件名带内容 hash → immutable 一年（发版换 hash
			// 自然失效）；index.html/SPA fallback → no-cache（每次协商，发版即生效）
			if strings.HasPrefix(filePath, "/assets/") {
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				c.Header("Cache-Control", "no-cache")
			}
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
	registerUpdateRoutes(r)
}

// registerUpdateRoutes 注册软件版本检查/更新端点。
// GET status 匿名可读；POST check/download 由平台认证中间件要求管理员。
// 检查服务同时在启动时自动拉起（HTTPApp.Start → StartAutoCheck）。
func registerUpdateRoutes(r *server.Hertz) {
	updCfg := config.Get().Update
	updateSvc := update.New(updCfg, versionpkg.Version)
	updateHandler := handlers.NewUpdateHandler(updateSvc)

	r.GET("/api/update/status", updateHandler.GetStatus)
	r.POST("/api/update/check", updateHandler.CheckNow)
	r.POST("/api/update/download", updateHandler.Download)
	r.POST("/api/update/apply", updateHandler.Apply)
	r.POST("/api/update/restart", updateHandler.Restart)

	registerSystemInstallRoutes(r)

	// 启动注册检查 + 定期复查（独立 goroutine，进程生命周期）
	updateHandler.StartAutoCheck(context.Background())
}

// registerSystemInstallRoutes 注册系统服务安装（开机自启）端点。
// GET status 匿名可读；POST 动作由平台认证中间件要求管理员。
func registerSystemInstallRoutes(r *server.Hertz) {
	h := handlers.NewSystemInstallHandler()
	r.GET("/api/system/install-status", h.GetStatus)
	r.POST("/api/system/install", h.Install)
	r.POST("/api/system/uninstall", h.Uninstall)
	r.POST("/api/system/service/:action", h.ServiceAction)
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

// registerMCPPostRoutes 注册 MCP 简化协议的 POST 路由。
// 全量工具从 mcp.Tools 动态注册（POST /mcp/<工具全名>）——新增工具无需改这里；
// 另保留早期集成的短别名（/mcp/bugs 等）向后兼容。
func registerMCPPostRoutes(r *server.Hertz, transport *mcp.HTTPTransport) {
	for _, tool := range mcp.Tools {
		name := tool.Name
		r.POST("/mcp/"+name, func(ctx context.Context, c *app.RequestContext) {
			callMCPAction(c, transport, name)
		})
	}

	aliases := map[string]string{
		"products":   "get_products",
		"projects":   "get_projects",
		"executions": "get_executions",
		"bugs":       "get_bugs",
		"stories":    "get_stories",
		"tasks":      "get_tasks",
		"users":      "get_users",
		"timelog":    "get_timelog",
	}
	for path, action := range aliases {
		act := action
		r.POST("/mcp/"+path, func(ctx context.Context, c *app.RequestContext) {
			callMCPAction(c, transport, act)
		})
	}
}

// callMCPAction POST /mcp/<action> 的统一处理：优先解析 JSON body 里的 params
// （{"action":...,"params":{...}} 形式），body 无有效 params 时回退 query 参数
// （GET 风格兼容）。此前该路径只收 query 白名单参数，POST body 的 params 被
// 静默丢弃——所有带参调用实际都在无参执行。
func callMCPAction(c *app.RequestContext, transport *mcp.HTTPTransport, action string) {
	var body struct {
		Params map[string]interface{} `json:"params"`
	}
	if len(c.Request.Body()) > 0 {
		if err := json.Unmarshal(c.Request.Body(), &body); err != nil {
			body.Params = nil // 容忍非 JSON body，走 query
		}
	}
	if len(body.Params) > 0 {
		transport.HandleActionByNameWithParams(context.Background(), action, body.Params, c)
		return
	}
	transport.HandleActionByName(context.Background(), action, c)
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

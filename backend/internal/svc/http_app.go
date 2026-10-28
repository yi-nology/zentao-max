package svc

import (
	"context"
	"fmt"
	"os"

	"github.com/yi-nology/zentao-max/backend/internal/routes"
	"github.com/yi-nology/zentao-max/backend/pkg/logger"

	"github.com/oekit/concurrency/async"

	"github.com/cloudwego/hertz/pkg/app/server"
)

type HTTPApp struct {
	config    *AppConfig
	deps      *Dependencies
	hertz     *server.Hertz
	ctx       context.Context
	cancel    context.CancelFunc
	embedMode bool
}

func newHTTPApp(config *AppConfig, deps *Dependencies) *HTTPApp {
	return &HTTPApp{
		config:    config,
		deps:      deps,
		embedMode: config.StaticPath != "",
	}
}

// Start 前置要求：入口层已完成 config.Init 与 logger.Init
// （bootstrap 组件序保证：config → logger → metrics → userFn）。
func (a *HTTPApp) Start(ctx context.Context) error {
	a.ctx, a.cancel = context.WithCancel(ctx)

	a.deps.Handlers.InitScheduler(a.deps.ConfigStore)

	port := a.config.Port
	if port == "" {
		port = os.Getenv("PORT")
		if port == "" {
			port = "12345"
		}
	}

	a.hertz = routes.SetupRouterWithHandlers(
		a.deps.InitService,
		a.deps.ZentaoClient,
		a.deps.Handlers,
		":"+port,
		a.config.StaticFS,
	)

	// Run 阻塞在 accept loop，不监听信号——优雅关停由入口层的
	// bootstrap 编排（App.AddResource → HTTPApp.Stop → hertz.Shutdown）。
	// RunSafe 收编 Run 的 panic（如端口占用），启动失败即进程退出，
	// 避免服务未起而进程假活的静默故障（与旧 Spin 出错退出语义一致）。
	async.GoSafe(func() {
		logger.Info("HTTP server starting",
			"name", a.Name(),
			"port", port,
			"zentao_server", a.config.ZentaoServer,
		)

		var runner async.Runner
		if err := runner.RunSafeErr(func() error { return a.hertz.Run() }); err != nil {
			logger.Fatal("HTTP server failed to start", "error", err)
		}
	})

	return nil
}

func (a *HTTPApp) Stop(ctx context.Context) error {
	if a.cancel != nil {
		a.cancel()
	}

	a.deps.Handlers.StopScheduler()
	a.deps.Handlers.CloseCache()

	if a.hertz != nil {
		logger.Info("HTTP server shutting down", "name", a.Name())
		if err := a.hertz.Shutdown(ctx); err != nil {
			if err.Error() != "engine is not running" {
				return fmt.Errorf("failed to shutdown server: %w", err)
			}
		}
		logger.Info("HTTP server stopped", "name", a.Name())
	}

	return nil
}

func (a *HTTPApp) Name() string {
	return "HTTP-Server"
}

func (a *HTTPApp) GetRouter() *server.Hertz {
	return a.hertz
}

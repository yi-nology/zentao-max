package app

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/yi-nology/zentao-max/backend/core/config"
	"github.com/yi-nology/zentao-max/backend/core/logger"
	"github.com/yi-nology/zentao-max/backend/core/routes"

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

func NewHTTPApp(config *AppConfig, deps *Dependencies) *HTTPApp {
	return &HTTPApp{
		config:    config,
		deps:      deps,
		embedMode: config.StaticPath != "",
	}
}

func (a *HTTPApp) Start(ctx context.Context) error {
	a.ctx, a.cancel = context.WithCancel(ctx)

	if err := config.Init("", "ZENTAO_MAX"); err != nil {
		log.Printf("Warning: failed to initialize config: %v", err)
	}
	cfg := config.Get()

	if err := logger.Init(&cfg.Log); err != nil {
		log.Printf("Warning: failed to initialize logger: %v", err)
	}

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
	// bootstrap 编排（App.AddResource → HTTPApp.Stop → hertz.Shutdown）
	go func() {
		logger.Info("HTTP server starting",
			"name", a.Name(),
			"port", port,
			"zentao_server", a.config.ZentaoServer,
		)

		if err := a.hertz.Run(); err != nil {
			logger.Error("HTTP server exited with error", "error", err)
		}
	}()

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

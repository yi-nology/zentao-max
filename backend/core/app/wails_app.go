package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/yi-nology/zentao-max/backend/core/config"
	"github.com/yi-nology/zentao-max/backend/core/logger"
	"github.com/yi-nology/zentao-max/backend/core/metrics"
	"github.com/yi-nology/zentao-max/backend/core/routes"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

type WailsApp struct {
	config *AppConfig
	deps   *Dependencies
	hertz  *server.Hertz
	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
}

func NewWailsApp(config *AppConfig, deps *Dependencies) *WailsApp {
	return &WailsApp{
		config: config,
		deps:   deps,
	}
}

func (a *WailsApp) Start(ctx context.Context) error {
	a.ctx = ctx

	ctxWithCancel, cancel := context.WithCancel(ctx)
	a.cancel = cancel

	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	if err := config.Init("", "ZENTAO_MAX"); err != nil {
		log.Printf("Warning: failed to initialize config: %v", err)
	}
	cfg := config.Get()

	if err := logger.Init(&cfg.Log); err != nil {
		log.Printf("Warning: failed to initialize logger: %v", err)
	}

	a.deps.Handlers.InitScheduler(a.deps.ConfigStore)

	if err := metrics.Init(); err != nil {
		logger.Error("Failed to initialize metrics", zap.Error(err))
	}

	go func() {
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
			nil,
		)

		logger.Info("Wails backend starting",
			zap.String("name", a.Name()),
			zap.String("port", port),
			zap.String("zentao_server", a.config.ZentaoServer),
		)

		go func() {
			a.hertz.Spin()
		}()

		<-ctxWithCancel.Done()
		logger.Info("Wails backend shutting down", zap.String("name", a.Name()))

		a.mu.Lock()
		hertz := a.hertz
		a.mu.Unlock()
		if hertz != nil {
			if err := hertz.Shutdown(context.Background()); err != nil {
				logger.Error("Server forced to shutdown", zap.Error(err))
			}
		}

		logger.Info("Wails backend stopped", zap.String("name", a.Name()))
	}()

	return nil
}

func (a *WailsApp) Stop(ctx context.Context) error {
	a.deps.Handlers.StopScheduler()
	a.deps.Handlers.CloseCache()
	if a.cancel != nil {
		a.cancel()
	}
	log.Printf("%s backend service stopped", a.Name())
	return nil
}

func (a *WailsApp) Name() string {
	return "Wails-Desktop"
}

func (a *WailsApp) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

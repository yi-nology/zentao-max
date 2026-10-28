package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/oekit/bootstrap"

	"github.com/yi-nology/zentao-max/backend/core/app"
	"github.com/yi-nology/zentao-max/backend/core/config"
	"github.com/yi-nology/zentao-max/backend/core/initialization"
	"github.com/yi-nology/zentao-max/backend/core/logger"
	"github.com/yi-nology/zentao-max/backend/core/metrics"
)

var (
	encryptionKey  = ""
	zentaoServer   = ""
	zentaoAccount  = ""
	zentaoPassword = ""
	// authDBPath 可由 ldflags 注入固定路径；默认空，由 initialization.ResolveAuthDBPath
	// 统一解析（ZENTAO_AUTH_DB_PATH env → 已存在候选 → ./data/auth.db），见 auth_path.go。
	authDBPath = ""
)

//go:embed static/*
var embeddedStaticFS embed.FS

func getFileSystem() http.FileSystem {
	staticFS, err := fs.Sub(embeddedStaticFS, "static")
	if err != nil {
		panic(err)
	}
	return http.FS(staticFS)
}

func main() {
	if err := logger.Init(&config.LogConfig{
		Level:            "info",
		Format:           "console",
		OutputPath:       "",
		EnableCaller:     true,
		EnableStacktrace: false,
	}); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	if err := metrics.Init(); err != nil {
		log.Fatalf("Failed to initialize metrics: %v", err)
	}

	// 预加载配置（HTTPApp.Start 内会再幂等 Init 一次）：
	// bootstrap 的优雅关停超时需要 cfg.Server.ShutdownTimeout
	if err := config.Init("", "ZENTAO_MAX"); err != nil {
		log.Printf("Warning: failed to initialize config: %v", err)
	}
	cfg := config.Get()

	resolvedAuthDBPath := initialization.ResolveAuthDBPath(authDBPath)
	log.Printf("Auth DB path: %s", resolvedAuthDBPath)

	appConfig := &app.AppConfig{
		Type:           "http",
		Port:           os.Getenv("PORT"),
		AuthDBPath:     resolvedAuthDBPath,
		EncryptionKey:  encryptionKey,
		ZentaoServer:   zentaoServer,
		ZentaoAccount:  zentaoAccount,
		ZentaoPassword: zentaoPassword,
		StaticPath:     "embedded",
		StaticFS:       getFileSystem(),
	}

	application, err := app.InitializeHTTPApp(appConfig)
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	httpApp, ok := application.(*app.HTTPApp)
	if !ok {
		log.Fatalf("Expected HTTPApp, got %T", application)
	}

	// 启动编排交给 oekit/bootstrap：等 SIGINT/SIGTERM → 执行注册的 closer
	// （HTTPApp.Stop：停 scheduler → 关缓存 → hertz.Shutdown）→ 进程退出。
	// 原实现 select{} 永久阻塞，优雅关停完全依赖 hertz.Spin 内置信号处理。
	b, err := bootstrap.New(
		bootstrap.WithServiceName("zentao-max"),
		bootstrap.WithShutdownTimeout(cfg.Server.GetShutdownTimeout()),
	)
	if err != nil {
		log.Fatalf("Failed to create bootstrapper: %v", err)
	}

	if err := b.Run(func(a *bootstrap.App) error {
		if err := httpApp.Start(context.Background()); err != nil {
			return err
		}
		log.Printf("Serving frontend from embedded static file system")
		a.AddResource("http-app", httpApp.Stop)
		// userFn 返回即触发关停（bootstrap 语义）：阻塞到收到 SIGINT/SIGTERM
		<-a.Context().Done()
		return nil
	}); err != nil {
		log.Fatalf("Application exited with error: %v", err)
	}
}

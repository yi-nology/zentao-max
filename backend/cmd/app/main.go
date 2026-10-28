package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/oekit/bootstrap"
	"github.com/oekit/core/lifecycle"

	"github.com/yi-nology/zentao-max/backend/internal/config"
	"github.com/yi-nology/zentao-max/backend/internal/initialization"
	"github.com/yi-nology/zentao-max/backend/internal/svc"
	"github.com/yi-nology/zentao-max/backend/pkg/logger"
	"github.com/yi-nology/zentao-max/backend/pkg/metrics"
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

// loggerComponent / metricsComponent：启动任务组件化（oekit bootstrap）。
// 组件按注册序 Init，任一失败自动回滚已初始化组件并退出——替代 main 里
// 的手写 if err != nil 序列。
type fnComponent struct {
	name string
	fn   func() error
}

func (c fnComponent) Name() string { return c.name }
func (c fnComponent) Init(context.Context, *bootstrap.App, *lifecycle.Manager) error {
	return c.fn()
}

func main() {
	// config 是 bootstrap 的前置依赖（ShutdownTimeout 等构造参数依赖它），
	// 先于 bootstrap.New 执行；logger/metrics 走组件序
	if err := config.Init("", "ZENTAO_MAX"); err != nil {
		log.Printf("Warning: failed to initialize config: %v", err)
	}
	cfg := config.Get()

	resolvedAuthDBPath := initialization.ResolveAuthDBPath(authDBPath)
	log.Printf("Auth DB path: %s", resolvedAuthDBPath)

	appConfig := &svc.AppConfig{
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

	application, err := svc.InitializeHTTPApp(appConfig)
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	httpApp, ok := application.(*svc.HTTPApp)
	if !ok {
		log.Fatalf("Expected HTTPApp, got %T", application)
	}

	// 启动编排交给 oekit/bootstrap：组件序初始化（logger → metrics）→
	// userFn 拉起服务 → 等 SIGINT/SIGTERM → closer（HTTPApp.Stop：
	// 停 scheduler → 关缓存 → hertz.Shutdown）→ 进程退出
	b, err := bootstrap.New(
		bootstrap.WithServiceName("zentao-max"),
		bootstrap.WithShutdownTimeout(cfg.Server.GetShutdownTimeout()),
		bootstrap.WithComponent(fnComponent{
			name: "logger",
			fn:   func() error { return logger.Init(&cfg.Log) },
		}),
		bootstrap.WithComponent(fnComponent{
			name: "metrics",
			fn:   metrics.Init,
		}),
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

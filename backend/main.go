// Package main 是 zentao-max 唯一服务入口（对齐 octopus 根 main 布局）。
//
// 形态由 -static 决定：
//   - 默认 embedded：前端构建产物（copy-static.sh → backend/static/）以
//     embed 方式打进单二进制，CGO_ENABLED=0 交叉编译即完整发布物；
//   - -static <dir>：从磁盘目录伺服前端（本地联调 vite build 产物）。
//
// 原 cmd/app（嵌入）与 cmd/server（纯 HTTP）两入口的差异仅此一处，
// 合并后配置加载/凭据解析/bootstrap 编排共用同一份实现。
package main

import (
	"context"
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/oekit/bootstrap"
	oekitconfig "github.com/oekit/core/config"
	"github.com/oekit/core/lifecycle"

	"github.com/yi-nology/zentao-max/backend/internal/initialization"
	"github.com/yi-nology/zentao-max/backend/internal/svc"
	"github.com/yi-nology/zentao-max/backend/pkg/logger"
	"github.com/yi-nology/zentao-max/backend/pkg/metrics"

	internalconfig "github.com/yi-nology/zentao-max/backend/internal/config"
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

// fnComponent 启动任务组件（oekit bootstrap Component）。
type fnComponent struct {
	name string
	fn   func() error
}

func (c fnComponent) Name() string { return c.name }
func (c fnComponent) Init(context.Context, *bootstrap.App, *lifecycle.Manager) error {
	return c.fn()
}

func main() {
	configPath := flag.String("config", "", "配置文件路径")
	envFile := flag.String("env", "", "环境变量文件路径（.env）")
	// static 静态资源来源：embedded（默认，编译期嵌入）或磁盘目录
	staticDir := flag.String("static", "embedded", "静态资源来源：embedded 或磁盘目录")
	flag.Parse()

	if *envFile != "" {
		// LoadDotEnv 不覆盖已存在的进程环境变量
		oekitconfig.LoadDotEnv(*envFile)
	} else {
		oekitconfig.LoadDotEnv()
	}

	// config 是 bootstrap 的前置依赖（ShutdownTimeout 等构造参数依赖它），
	// 先于 bootstrap.New 执行；logger/metrics 走组件序
	if err := internalconfig.Init(*configPath, "ZENTAO_MAX"); err != nil {
		log.Printf("Warning: failed to initialize config: %v", err)
	}
	cfg := internalconfig.Get()

	resolvedAuthDBPath := initialization.ResolveAuthDBPath(authDBPath)
	log.Printf("Auth DB path: %s", resolvedAuthDBPath)

	// PORT env 优先（部署脚本/平台惯例），其次 config server.port
	port := os.Getenv("PORT")
	if port == "" {
		port = cfg.Server.Port
	}

	appConfig := &svc.AppConfig{
		Type:           "http",
		Port:           port,
		AuthDBPath:     resolvedAuthDBPath,
		EncryptionKey:  encryptionKey,
		ZentaoServer:   zentaoServer,
		ZentaoAccount:  zentaoAccount,
		ZentaoPassword: zentaoPassword,
		StaticPath:     *staticDir,
	}
	switch *staticDir {
	case "embedded":
		sub, err := fs.Sub(embeddedStaticFS, "static")
		if err != nil {
			panic(err)
		}
		appConfig.StaticFS = http.FS(sub)
	default:
		// 磁盘目录伺服（本地联调 vite build 产物）
		appConfig.StaticFS = http.Dir(*staticDir)
	}

	httpApp, err := svc.InitializeHTTPApp(appConfig)
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
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

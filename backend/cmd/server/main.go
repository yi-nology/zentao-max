package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/oekit/bootstrap"
	"github.com/oekit/core/lifecycle"

	"github.com/yi-nology/zentao-max/backend/internal/svc"
	"github.com/yi-nology/zentao-max/backend/internal/config"
	"github.com/yi-nology/zentao-max/backend/pkg/logger"
	"github.com/yi-nology/zentao-max/backend/pkg/metrics"

	oekitconfig "github.com/oekit/core/config"
)

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
	// 解析命令行参数
	configPath := flag.String("config", "", "配置文件路径")
	envFile := flag.String("env", "", "环境变量文件路径（.env）")
	flag.Parse()

	// 加载.env文件（如果指定）
	if *envFile != "" {
		// LoadDotEnv 不覆盖已存在的进程环境变量
		oekitconfig.LoadDotEnv(*envFile)
	} else {
		// 尝试加载默认.env文件
		oekitconfig.LoadDotEnv()
	}

	// 初始化配置
	if err := config.Init(*configPath, "ZENTAO_MAX"); err != nil {
		fmt.Printf("Failed to initialize config: %v\n", err)
		os.Exit(1)
	}
	cfg := config.Get()

	logger.Info("Starting zentao-max server",
		"type", cfg.Server.Type,
		"port", cfg.Server.Port,
		"log_level", cfg.Log.Level,
		"log_format", cfg.Log.Format,
	)

	// 创建应用配置
	appConfig := &svc.AppConfig{
		Type:           cfg.Server.Type,
		Port:           cfg.Server.Port,
		AuthDBPath:     cfg.Auth.DBPath,
		EncryptionKey:  cfg.Security.EncryptionKey,
		ZentaoServer:   cfg.Zentao.Server,
		ZentaoAccount:  cfg.Zentao.Account,
		ZentaoPassword: cfg.Zentao.Password,
		StaticPath:     cfg.Server.StaticPath,
	}

	// 使用依赖注入初始化应用
	application, err := svc.InitializeHTTPApp(appConfig)
	if err != nil {
		logger.Fatal("Failed to initialize application", "error", err)
	}

	// 启动编排交给 oekit/bootstrap：等 SIGINT/SIGTERM → 按注册顺序执行
	// closer（HTTPApp.Stop：停 scheduler → 关缓存 → hertz.Shutdown）→ 进程退出
	b, err := bootstrap.New(
		bootstrap.WithServiceName("zentao-max-server"),
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
		logger.Fatal("Failed to create bootstrapper", "error", err)
	}

	if err := b.Run(func(a *bootstrap.App) error {
		if err := application.Start(context.Background()); err != nil {
			return err
		}
		logger.Info("Application started successfully")
		a.AddResource("http-app", application.Stop)
		// userFn 返回即触发关停（bootstrap 语义）：阻塞到收到 SIGINT/SIGTERM
		<-a.Context().Done()
		return nil
	}); err != nil {
		logger.Fatal("Application exited with error", "error", err)
	}
}

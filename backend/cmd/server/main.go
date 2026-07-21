package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/yi-nology/zentao-max/backend/core/app"
	"github.com/yi-nology/zentao-max/backend/core/config"
	"github.com/yi-nology/zentao-max/backend/core/logger"
	"github.com/yi-nology/zentao-max/backend/core/metrics"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func main() {
	// 解析命令行参数
	configPath := flag.String("config", "", "配置文件路径")
	envFile := flag.String("env", "", "环境变量文件路径（.env）")
	flag.Parse()

	// 加载.env文件（如果指定）
	if *envFile != "" {
		if err := godotenv.Load(*envFile); err != nil {
			fmt.Printf("Warning: failed to load .env file: %v\n", err)
		}
	} else {
		// 尝试加载默认.env文件
		godotenv.Load()
	}

	// 初始化配置
	if err := config.Init(*configPath, "ZENTAO_MAX"); err != nil {
		fmt.Printf("Failed to initialize config: %v\n", err)
		os.Exit(1)
	}
	cfg := config.Get()

	// 初始化日志
	if err := logger.Init(&cfg.Log); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// 初始化性能监控
	if err := metrics.Init(); err != nil {
		logger.Fatal("Failed to initialize metrics", zap.Error(err))
	}

	logger.Info("Starting chandao-mini application",
		zap.String("type", cfg.Server.Type),
		zap.String("port", cfg.Server.Port),
		zap.String("log_level", cfg.Log.Level),
		zap.String("log_format", cfg.Log.Format),
	)

	// 创建应用配置
	appConfig := &app.AppConfig{
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
	application, err := app.InitializeHTTPApp(appConfig)
	if err != nil {
		logger.Fatal("Failed to initialize application", zap.Error(err))
	}

	// 启动应用
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := application.Start(ctx); err != nil {
		logger.Fatal("Failed to start application", zap.Error(err))
	}

	logger.Info("Application started successfully")

	// 等待中断信号进行优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down application...")

	// 创建超时上下文
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.GetShutdownTimeout())
	defer shutdownCancel()

	// 执行优雅关闭
	if err := application.Stop(shutdownCtx); err != nil {
		logger.Error("Error during shutdown", zap.Error(err))
	}

	logger.Info("Application stopped")
}

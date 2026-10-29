// Package main 是 MCP stdio 传输的独立入口。
//
// 编译产物 zentao-max-mcp 专供 AI 工具（Claude Desktop / Cursor / Claude Code 等）
// 作为 stdio MCP Server 子进程启动。进程通过 stdin/stdout 以 JSON Lines 协议通信：
//
//	请求: {"action":"get_bugs","params":{"productId":1}}
//	响应: {"status":"ok","message":"...","data":[...]}
//
// 启动方式：
//
//	zentao-max-mcp                 # 默认从 config.yaml / 环境变量读取禅道配置
//	zentao-max-mcp --config x.yaml # 指定配置文件
//	zentao-max-mcp --env .env      # 指定 .env 文件
//
// 禅道连接配置优先级：config 文件 → 环境变量 → auth.db 加密存储（与 HTTP 入口一致）。
// MCP 鉴权 Token 通过 MCP_TOKEN 或 ZENTAO_MAX_MCP_TOKEN 环境变量设置。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/yi-nology/zentao-max/backend/internal/auth"
	"github.com/yi-nology/zentao-max/backend/internal/config"
	"github.com/yi-nology/zentao-max/backend/internal/infra/zentao"
	"github.com/yi-nology/zentao-max/backend/internal/initialization"
	"github.com/yi-nology/zentao-max/backend/internal/mcp"
	"github.com/yi-nology/zentao-max/backend/pkg/logger"
	"github.com/yi-nology/zentao-max/backend/pkg/metrics"

	oekitconfig "github.com/oekit/core/config"
)

func main() {
	// 解析命令行参数
	configPath := flag.String("config", "", "配置文件路径")
	envFile := flag.String("env", "", "环境变量文件路径（.env）")
	flag.Parse()

	// 加载 .env 文件（如果指定）
	if *envFile != "" {
		// LoadDotEnv 不覆盖已存在的进程环境变量
		oekitconfig.LoadDotEnv(*envFile)
	} else {
		oekitconfig.LoadDotEnv()
	}

	// 初始化配置
	if err := config.Init(*configPath, "ZENTAO_MAX"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize config: %v\n", err)
		os.Exit(1)
	}
	cfg := config.Get()

	// 初始化日志。stdio 模式下日志必须走 stderr——JSON Lines 协议独占 stdout，
	// 任何 stdout 上的日志都会让 MCP 客户端解析失败（config 未显式配置时强制 stderr）
	if cfg.Log.OutputPath == "" {
		cfg.Log.OutputPath = "stderr"
	}
	if err := logger.Init(&cfg.Log); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// 初始化性能监控
	if err := metrics.Init(); err != nil {
		logger.Fatal("Failed to initialize metrics", "error", err)
	}

	logger.Info("Starting MCP stdio server",
		"configured_transport", cfg.MCP.Transport,
		"actual_transport", "stdio",
		"enabled", cfg.MCP.Enabled,
		"read_only", cfg.MCP.ReadOnly,
		"token_set", cfg.MCP.Token != "",
	)

	// 初始化 MCP 模式管理器（从配置加载）
	mcp.GetMCPModeManager().InitFromConfig(cfg.MCP)

	// 装配禅道客户端：优先 config/env，回退 auth.db 加密存储
	initSvc := initialization.NewInitService(cfg.Auth.DBPath, cfg.Security.EncryptionKey)
	zentaoServer, zentaoAccount, zentaoPassword, zentaoRealm := resolveZentaoConfig(cfg, initSvc)
	if zentaoServer == "" {
		logger.Warn("Zentao server not configured; stdio MCP will start but queries will fail until config is provided",
			"hint", "AI 可引导用户调 update_zentao_config 工具完成首次配置（server/account/password）",
		)
	}

	client := zentao.NewSessionClient(zentaoServer, zentaoAccount, zentaoPassword, zentaoRealm, "", nil)

	// 构造 MCP Server（直接从 client，无需 HTTP/handler 依赖）
	// 注入基础配置能力：auth.db 持久化（update_zentao_config）+ 统一认证平台 SSO 登录（send_verify_code/sso_login）
	mcpServer := mcp.NewMCPServer(client)
	var ssoClient *auth.SSOClient
	if cfg.Auth.SSO.Enabled {
		if strings.TrimSpace(cfg.Auth.SSO.BaseURL) == "" {
			logger.Warn("auth.sso.enabled=true but auth.sso.base_url is empty; sso MCP tools unavailable",
				"config_key", "auth.sso.base_url")
		} else {
			ssoClient = auth.NewSSOClient(cfg.Auth.SSO.BaseURL, cfg.Auth.SSO.AuthPageURL)
		}
	}
	mcpServer.WithConfigDeps(initSvc, ssoClient, nil) // stdio 无业务缓存，无需清理回调

	// 启动 stdio 传输（阻塞主 goroutine）
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	transport := mcp.NewStdioTransport(mcpServer)

	// 监听中断信号优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-quit
		logger.Info("Received signal, shutting down MCP stdio server", "signal", sig.String())
		cancel()
	}()

	logger.Info("MCP stdio server listening on stdin/stdout")
	transport.Run(ctx)

	logger.Info("MCP stdio server stopped")
}

// resolveZentaoConfig 解析禅道连接配置
// 优先级：config 文件 / 环境变量（已合并进 cfg）→ auth.db 加密存储.
func resolveZentaoConfig(cfg *config.Config, initSvc *initialization.InitService) (server, account, password, realm string) {
	// Init 已按「默认值 → config.yaml → env 覆盖」合并到 cfg.Zentao
	server = cfg.Zentao.Server
	account = cfg.Zentao.Account
	password = cfg.Zentao.Password
	realm = cfg.Zentao.Realm

	// 若 config/env 已提供，直接使用
	if server != "" && account != "" {
		return
	}

	// 回退：从 auth.db 加密存储读取（与 HTTP 入口 provideZentaoClient 一致）
	dbServer, dbAccount, dbPassword, dbRealm := initialization.LoadZentaoConfigWithRealm(initSvc)
	if server == "" {
		server = dbServer
	}
	if account == "" {
		account = dbAccount
	}
	if password == "" {
		password = dbPassword
	}
	if realm == "" {
		realm = dbRealm
	}
	return
}

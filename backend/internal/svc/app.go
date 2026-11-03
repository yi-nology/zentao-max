package svc

import (
	"net/http"

	"github.com/yi-nology/zentao-max/backend/internal/handlers"
	"github.com/yi-nology/zentao-max/backend/internal/infra/zentao"
	"github.com/yi-nology/zentao-max/backend/internal/initialization"
)

// AppConfig 服务构造配置（入口层 main.go 组装）。
type AppConfig struct {
	Type          string
	Port          string
	AuthDBPath    string
	EncryptionKey string
	// Zentao 凭据：入口层填入「ldflags 注入值，缺省回退 config.yaml zentao 段」；
	// 最终生效优先级（按字段）由 svc 装配统一处理：
	// 设置页保存(auth.db) > env(ZENTAO_* 裸前缀) > 本字段 > 内置默认
	ZentaoServer   string
	ZentaoAccount  string
	ZentaoPassword string
	// 认证域：非空（如 "kydc"）= 会话模式（受限实例 SSO），空 = Token 模式。
	// 无 ldflags 注入点，直接来自 config.yaml zentao.realm（env 可经
	// ZENTAO_MAX_ 前缀 overlay 进 config，效果等同）。
	ZentaoRealm string
	// 静态资源：StaticPath 非 "" 时启用 SPA 伺服（embedded 或磁盘目录），
	// StaticFS 为对应的文件系统实现
	StaticPath string
	StaticFS   http.FileSystem
}

// Dependencies 应用级共享依赖（装配产物，handlers/registry 消费）。
type Dependencies struct {
	InitService  *initialization.InitService
	ZentaoClient *zentao.Client
	ConfigStore  *initialization.ConfigStore
	Handlers     *handlers.HandlerRegistry
}

func NewDependencies(initService *initialization.InitService, zentaoClient *zentao.Client, configStore *initialization.ConfigStore) *Dependencies {
	return &Dependencies{
		InitService:  initService,
		ZentaoClient: zentaoClient,
		ConfigStore:  configStore,
		Handlers:     handlers.NewHandlerRegistry(zentaoClient, initService),
	}
}

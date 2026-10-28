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
	// Zentao 凭据：ldflags/config/env 优先级由 svc 装配统一处理（见 wire 注释），
	// 全空时回退 auth.db 加密存储
	ZentaoServer   string
	ZentaoAccount  string
	ZentaoPassword string
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

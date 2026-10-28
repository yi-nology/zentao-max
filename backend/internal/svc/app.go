package svc

import (
	"context"
	"net/http"

	"github.com/yi-nology/zentao-max/backend/internal/handlers"
	"github.com/yi-nology/zentao-max/backend/internal/initialization"
	"github.com/yi-nology/zentao-max/backend/internal/infra/zentao"
)

type Application interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Name() string
}

type AppConfig struct {
	Type string
	Port string

	ZentaoServer   string
	ZentaoAccount  string
	ZentaoPassword string

	AuthDBPath    string
	EncryptionKey string

	StaticPath string
	StaticFS   http.FileSystem
}

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

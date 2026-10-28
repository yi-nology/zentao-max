package handlers

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"

	"github.com/yi-nology/zentao-max/backend/core/errors"
	"github.com/yi-nology/zentao-max/backend/core/initialization"
	"github.com/yi-nology/zentao-max/backend/core/logger"
	"github.com/yi-nology/zentao-max/backend/core/zentao"
)

type InitHandler struct {
	initService  *initialization.InitService
	zentaoClient *zentao.Client
}

func NewInitHandler(initService *initialization.InitService, zentaoClient *zentao.Client) *InitHandler {
	return &InitHandler{
		initService:  initService,
		zentaoClient: zentaoClient,
	}
}

func (h *InitHandler) GetInitStatus(ctx context.Context, c *app.RequestContext) {
	status := h.initService.GetInitStatus()

	logger.Info("初始化状态检查",
		zap.Bool("isFirstStart", status.IsFirstStart),
		zap.Bool("hasConfig", status.HasConfig),
		zap.String("message", status.Message),
	)

	errors.Success(c, map[string]interface{}{
		"isFirstStart": status.IsFirstStart,
		"hasConfig":    status.HasConfig,
		"message":      status.Message,
	})
}

func (h *InitHandler) GetAccountInfo(ctx context.Context, c *app.RequestContext) {
	errors.Success(c, map[string]interface{}{
		"domain":    h.zentaoClient.GetServer(),
		"account":   h.zentaoClient.GetAccount(),
		"connected": h.zentaoClient.IsConnected(),
		"mode":      "session",
		"realm":     h.zentaoClient.GetRealm(),
	})
}

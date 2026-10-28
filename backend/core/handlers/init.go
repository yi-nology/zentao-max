package handlers

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	
	"github.com/yi-nology/zentao-max/backend/core/errors"
	"github.com/yi-nology/zentao-max/backend/core/initialization"
	"github.com/yi-nology/zentao-max/backend/core/logger"
	"github.com/yi-nology/zentao-max/backend/core/service"
	"github.com/yi-nology/zentao-max/backend/core/zentao"
)

type InitHandler struct {
	initService  *initialization.InitService
	zentaoClient *zentao.Client
	cacheService *service.CacheService
}

// zentaoConfigRequest 禅道连接配置表单请求体（设置页「连接配置」）。
type zentaoConfigRequest struct {
	Domain   string `json:"domain"`
	Account  string `json:"account"`
	Password string `json:"password"`
	// Realm 认证域（kydc / local）。空字符串走 Token 模式（REST API），
	// 非空（如 "kydc"）走会话模式（受限实例 SSO，密码明文登录）。
	Realm string `json:"realm"`
}

func NewInitHandler(initService *initialization.InitService, zentaoClient *zentao.Client, cacheService *service.CacheService) *InitHandler {
	return &InitHandler{
		initService:  initService,
		zentaoClient: zentaoClient,
		cacheService: cacheService,
	}
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

// UpdateZentaoConfig 禅道连接配置端点（POST /api/v1/zentao/config，平台登录后可用）。
//
// 流程：
//  1. 校验 domain/account/password 非空
//  2. 调 zentaoClient.UpdateSessionConfig 触发实际登录验证（同步，热切换客户端）
//  3. 登录成功 → 加密持久化 AuthConfig（含 realm）到 auth.db（路径见 ResolveAuthDBPath）
//  4. 登录失败 → 返回 400 + 后端原始错误（不落盘，不触发前端 401 重定向）
//  5. 成功后失效全部业务缓存：坏配置期写入的空结果会残留毒化健康期数据
//
// 前置：全局 auth 中间件对非 GET 请求要求平台登录，匿名调用不到这里。
func (h *InitHandler) UpdateZentaoConfig(ctx context.Context, c *app.RequestContext) {
	var req zentaoConfigRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "请求参数无效: "+err.Error())
		return
	}
	if req.Domain == "" || req.Account == "" || req.Password == "" {
		errors.BadRequest(c, "domain、account、password 不能为空")
		return
	}
	if req.Realm == "local" {
		// local 域 = Token 模式，realm 置空走 REST（与旧 init-guide 表单语义一致）
		req.Realm = ""
	}

	if err := h.zentaoClient.UpdateSessionConfig(req.Domain, req.Account, req.Password, req.Realm); err != nil {
		logger.Warn("禅道连接配置验证失败",
			"domain", req.Domain,
			"account", req.Account,
			"realm", req.Realm,
			"error", err,
		)
		errors.BadRequest(c, "连接验证失败: "+err.Error())
		return
	}

	authConfig := &initialization.AuthConfig{
		Username: req.Account,
		Password: req.Password,
		Domain:   req.Domain,
		Realm:    req.Realm,
	}
	if err := h.initService.StoreAuthConfigFromStruct(authConfig); err != nil {
		logger.Error("持久化禅道连接配置失败", "error", err)
		errors.InternalError(c, "保存连接配置失败")
		return
	}
	if h.cacheService != nil {
		if err := h.cacheService.ClearAll(ctx); err != nil {
			logger.Warn("清空业务缓存失败（不影响配置保存）", "error", err)
		}
	}

	errors.SuccessWithMessage(c, "连接成功，配置已保存", map[string]interface{}{
		"domain":    h.zentaoClient.GetServer(),
		"account":   h.zentaoClient.GetAccount(),
		"connected": h.zentaoClient.IsConnected(),
		"mode":      "session",
		"realm":     h.zentaoClient.GetRealm(),
	})
}

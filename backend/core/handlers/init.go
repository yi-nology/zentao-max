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

// loginRequest 账号密码登录表单请求体。
type loginRequest struct {
	Domain   string `json:"domain"`
	Account  string `json:"account"`
	Password string `json:"password"`
	// Realm 认证域（kydc / local）。空字符串走禅道本地账号（md5 混淆），
	// 非空（如 "kydc"）走受限实例 SSO（密码明文）。
	Realm string `json:"realm"`
}

func NewInitHandler(initService *initialization.InitService, zentaoClient *zentao.Client) *InitHandler {
	return &InitHandler{
		initService:  initService,
		zentaoClient: zentaoClient,
	}
}

// Login 账号密码登录端点（POST /api/init/login）。
//
// 流程：
//  1. 校验 domain/account/password 非空
//  2. 调 zentaoClient.UpdateSessionConfig 触发实际登录验证
//  3. 登录成功 → 加密持久化 AuthConfig（含 realm）到 auth.db
//  4. 登录失败 → 返回 400 + 后端原始错误（不触发前端 401 重定向）
//
// 注意：先验证再持久化，避免把错误凭据写盘。
func (h *InitHandler) Login(ctx context.Context, c *app.RequestContext) {
	var req loginRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "请求参数无效: "+err.Error())
		return
	}
	if req.Domain == "" || req.Account == "" || req.Password == "" {
		errors.BadRequest(c, "domain、account、password 不能为空")
		return
	}

	// 验证凭据（热重载客户端）。失败立即返回，不落盘。
	if err := h.zentaoClient.UpdateSessionConfig(req.Domain, req.Account, req.Password, req.Realm); err != nil {
		logger.Warn("会话登录失败",
			zap.String("domain", req.Domain),
			zap.String("account", req.Account),
			zap.String("realm", req.Realm),
			zap.Error(err),
		)
		errors.BadRequest(c, "登录失败: "+err.Error())
		return
	}

	// 验证通过，持久化。Session 模式下 connected 已被 UpdateSessionConfig 置 true。
	authConfig := &initialization.AuthConfig{
		Username: req.Account,
		Password: req.Password,
		Domain:   req.Domain,
		Realm:    req.Realm,
	}
	if err := h.initService.StoreAuthConfigFromStruct(authConfig); err != nil {
		logger.Error("持久化登录配置失败", zap.Error(err))
		errors.InternalError(c, "保存登录配置失败")
		return
	}

	errors.SuccessWithMessage(c, "登录成功", map[string]interface{}{
		"domain":    h.zentaoClient.GetServer(),
		"account":   h.zentaoClient.GetAccount(),
		"connected": h.zentaoClient.IsConnected(),
		"mode":      "session",
		"realm":     req.Realm,
	})
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

// zentaoSIDRequest 是 ImportZentaoSID 端点的请求体。
type zentaoSIDRequest struct {
	ZentaoSID string `json:"zentaosid"`
	Note      string `json:"note,omitempty"`
}

// ImportZentaoSID 接收用户从浏览器 DevTools 复制的 zentaosid cookie，
// 持久化到 ~/.zentao-max/zentaosid.json。server 重启后 wire.go 自动读这个
// 文件调 Client.SetZentaoSID 注入 cookie jar，避免 server 端 login 触发
// 受限实例 SSO "密码尝试次数太多" 限流。
//
// 注意：本端点只是持久化 + 注入到当前 client，并不"测试"该 sid 是否
// 有效（避免一旦 sid 失效就立刻再触发 login 限流）。无效 sid 会导致
// 后续请求全部返 user-login redirect，doSessionJSONWithRetry 兜底
// 走一次 password login（仍可能失败，但不会持续累积）。
func (h *InitHandler) ImportZentaoSID(ctx context.Context, c *app.RequestContext) {
	var req zentaoSIDRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "请求参数无效: "+err.Error())
		return
	}
	if req.ZentaoSID == "" {
		errors.BadRequest(c, "zentaosid 不能为空")
		return
	}
	if len(req.ZentaoSID) < 16 || len(req.ZentaoSID) > 64 {
		errors.BadRequest(c, "zentaosid 长度异常（应在 16-64 字符之间）")
		return
	}

	// 持久化到磁盘
	if err := initialization.SaveZentaoSID(req.ZentaoSID, req.Note); err != nil {
		logger.Error("zentaosid 持久化失败", zap.Error(err))
		errors.InternalError(c, "保存 zentaosid 失败: "+err.Error())
		return
	}

	// 立刻注入到当前 client（不需要重启 server）
	if h.zentaoClient != nil {
		if err := h.zentaoClient.SetZentaoSID(req.ZentaoSID); err != nil {
			logger.Warn("zentaosid 注入失败（持久化已成功，下次启动自动生效）", zap.Error(err))
			errors.Success(c, map[string]interface{}{
				"saved":        true,
				"injected":     false,
				"injected_msg": err.Error(),
			})
			return
		}
	}

	logger.Info("zentaosid 导入成功（受限实例 SSO 限流 bypass 启用）")
	errors.Success(c, map[string]interface{}{
		"saved":    true,
		"injected": true,
	})
}

// DeleteZentaoSID 清掉持久化的 zentaosid.json（清掉注入的 zentaosid，
// 下次启动走正常 login 路径）。主要用于调试/切换账号。
func (h *InitHandler) DeleteZentaoSID(ctx context.Context, c *app.RequestContext) {
	if err := initialization.DeleteZentaoSID(); err != nil {
		logger.Warn("删除 zentaosid 失败", zap.Error(err))
		errors.InternalError(c, "删除 zentaosid 失败: "+err.Error())
		return
	}
	errors.Success(c, map[string]interface{}{"deleted": true})
}

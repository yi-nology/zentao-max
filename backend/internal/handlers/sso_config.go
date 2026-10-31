package handlers

import (
	"context"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/yi-nology/zentao-max/backend/internal/auth"
	"github.com/yi-nology/zentao-max/backend/internal/initialization"
	"github.com/yi-nology/zentao-max/backend/pkg/errors"
	"github.com/yi-nology/zentao-max/backend/pkg/logger"
)

// SSOConfigHandler 设置页「SSO 登录配置」（GET/POST /api/v1/sso-config）。
//
// 与「连接配置」同语义：先验证再保存、热生效（无需重启）、加密持久化 auth.db。
// auth.Manager 的 SSO 通道运行时可换（SetSSO），onSSOChange 供调用方同步其它
// 持有 SSOClient 的组件（如 MCP server 的 sso_* 工具）。
//
// 鸡生蛋兜底：SSO 是平台登录门槛，配置错误可能把人锁在门外——config.yaml 仍是
// 首次部署的 bootstrap；页面保存过的配置（auth.db ssoManaged=true）优先生效，
// 应急可通过保留的 /api/auth/login 本地管理员 API 或改回 config.yaml 恢复。
type SSOConfigHandler struct {
	initService *initialization.InitService
	authManager *auth.Manager
	// onSSOChange SSO 通道热切换后的同步回调（可为 nil）；参数与 authManager
	// 收到的相同（nil = 停用）。
	onSSOChange func(*auth.SSOClient)
	// yaml 配置（config.Get().Auth.SSO 的三个字段），数据库无保存记录时的 bootstrap。
	yamlEnabled     bool
	yamlBaseURL     string
	yamlAuthPageURL string
}

// NewSSOConfigHandler 构造 SSO 配置 handler。
func NewSSOConfigHandler(
	initService *initialization.InitService,
	authManager *auth.Manager,
	onSSOChange func(*auth.SSOClient),
	yamlEnabled bool,
	yamlBaseURL string,
	yamlAuthPageURL string,
) *SSOConfigHandler {
	return &SSOConfigHandler{
		initService:     initService,
		authManager:     authManager,
		onSSOChange:     onSSOChange,
		yamlEnabled:     yamlEnabled,
		yamlBaseURL:     yamlBaseURL,
		yamlAuthPageURL: yamlAuthPageURL,
	}
}

// ssoConfigView GET/POST 响应体。managed=false 时数据来自 config.yaml（bootstrap）。
type ssoConfigView struct {
	Enabled     bool   `json:"enabled"`
	BaseURL     string `json:"baseUrl"`
	AuthPageURL string `json:"authPageUrl"`
	Managed     bool   `json:"managed"`
	Source      string `json:"source"`
}

// Effective 解析当前生效的 SSO 配置：auth.db 有页面保存记录（ssoManaged）→ 数据库
// 为准；否则回退 yaml bootstrap。启动接线（routes.go）与本 handler 共用同一规则。
func (h *SSOConfigHandler) Effective() ssoConfigView {
	if cfg, _, err := h.initService.LoadAuthConfig(); err == nil && cfg != nil && cfg.SSOManaged {
		return ssoConfigView{
			Enabled:     cfg.SSOEnabled,
			BaseURL:     cfg.SSOBaseURL,
			AuthPageURL: cfg.SSOAuthPageURL,
			Managed:     true,
			Source:      "db",
		}
	}
	return ssoConfigView{
		Enabled:     h.yamlEnabled,
		BaseURL:     h.yamlBaseURL,
		AuthPageURL: h.yamlAuthPageURL,
		Managed:     false,
		Source:      "yaml",
	}
}

// SetOnSSOChange 接线 SSO 通道热切换的同步回调（MCP server 创建后调用——
// 构造时它还不存在）。
func (h *SSOConfigHandler) SetOnSSOChange(fn func(*auth.SSOClient)) {
	h.onSSOChange = fn
}

// GetConfig GET /api/v1/sso-config 查看当前生效的 SSO 登录配置。
func (h *SSOConfigHandler) GetConfig(ctx context.Context, c *app.RequestContext) {
	errors.Success(c, h.Effective())
}

// ssoConfigRequest SSO 登录配置表单请求体。
type ssoConfigRequest struct {
	Enabled bool `json:"enabled"`
	// BaseURL 统一认证平台根地址；启用时必填。
	BaseURL string `json:"baseUrl"`
	// AuthPageURL 登录页完整地址（作为 Referer 发送）；留空回退 {base_url}/sso/auth。
	AuthPageURL string `json:"authPageUrl"`
}

// UpdateConfig POST /api/v1/sso-config 保存 SSO 登录配置：
// 校验 → 可达性预检 → 热切换（authManager + onSSOChange）→ 合并持久化 auth.db。
// 全程不重启；失败不落盘不切换。
func (h *SSOConfigHandler) UpdateConfig(ctx context.Context, c *app.RequestContext) {
	var req ssoConfigRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "请求参数无效: "+err.Error())
		return
	}
	req.BaseURL = strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
	req.AuthPageURL = strings.TrimSpace(req.AuthPageURL)

	if req.Enabled {
		if req.BaseURL == "" {
			errors.BadRequest(c, "启用 SSO 登录必须填写统一认证平台地址")
			return
		}
		// 预检：坏地址存进去会把整个登录入口带走，5s 内给出明确错误
		if err := probeZentaoReachable(req.BaseURL, 5*time.Second); err != nil {
			errors.BadRequest(c, "统一认证平台不可达: "+err.Error())
			return
		}
	}

	// 热切换（先切后存，与连接配置同序：验证成本已付，切换本身不会失败）
	var ssoClient *auth.SSOClient
	if req.Enabled {
		ssoClient = auth.NewSSOClient(req.BaseURL, req.AuthPageURL)
	}
	h.authManager.SetSSO(ssoClient)
	if h.onSSOChange != nil {
		h.onSSOChange(ssoClient)
	}

	// 合并持久化：auth.db 单条记录，SSO 字段与禅道连接字段共存（UpdateAuthConfig 读改写）
	if err := h.initService.UpdateAuthConfig(func(cfg *initialization.AuthConfig) {
		cfg.SSOManaged = true
		cfg.SSOEnabled = req.Enabled
		cfg.SSOBaseURL = req.BaseURL
		cfg.SSOAuthPageURL = req.AuthPageURL
	}); err != nil {
		logger.Error("持久化 SSO 登录配置失败", "error", err)
		errors.InternalError(c, "保存 SSO 配置失败")
		return
	}

	msg := "SSO 登录已启用，配置已保存并即时生效"
	if !req.Enabled {
		msg = "SSO 登录已停用，登录页将回到本地管理员方式（保存即生效）"
	}
	errors.SuccessWithMessage(c, msg, h.Effective())
}

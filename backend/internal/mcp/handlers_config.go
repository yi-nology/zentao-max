package mcp

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/yi-nology/zentao-max/backend/internal/auth"
	"github.com/yi-nology/zentao-max/backend/internal/initialization"
	"github.com/yi-nology/zentao-max/backend/pkg/logger"
)

// MCP 基础配置/认证工具：连接配置查看与热更新、连通性探针、麒麟 SSO 短信登录两步。
// 让外部 AI 在无 Web UI 的场景（stdio 子进程、首次部署）能自助完成初始化配置与登录排错。

// handleGetZentaoConfig 查看当前禅道连接配置（密码不回显）。
func (s *MCPServer) handleGetZentaoConfig(_ map[string]interface{}) (interface{}, error) {
	account := s.client.GetAccount()
	return map[string]interface{}{
		"status":  "ok",
		"message": "当前禅道连接配置",
		"data": map[string]interface{}{
			"configured": account != "",
			"server":     s.client.GetServer(),
			"account":    account,
			"realm":      s.client.GetRealm(),
			"connected":  s.client.IsConnected(),
		},
	}, nil
}

// handleUpdateZentaoConfig 更新禅道连接配置：TCP 预检 → 同步登录验证 → 持久化 auth.db。
// 与设置页 POST /api/v1/zentao-config 同语义；保存成功后所有查询工具立即可用。
func (s *MCPServer) handleUpdateZentaoConfig(params map[string]interface{}) (interface{}, error) {
	if s.initService == nil {
		return nil, fmt.Errorf("当前入口未接入配置持久化，无法保存连接配置")
	}
	server := stringFromParams(params, "server")
	account := stringFromParams(params, "account")
	password := stringFromParams(params, "password")
	realm := stringFromParams(params, "realm")
	if server == "" || account == "" || password == "" {
		return nil, fmt.Errorf("server、account、password 不能为空")
	}
	if realm == "local" {
		// local 域 = Token 模式，realm 置空走 REST（与设置页表单语义一致）
		realm = ""
	}
	// 快速连通性预检：目标不可达时登录拨号要挂 75s+，5s 内给明确错误
	if err := probeZentaoReachable(server, 5*time.Second); err != nil {
		return nil, err
	}
	if err := s.client.UpdateSessionConfig(server, account, password, realm); err != nil {
		logger.Warn("MCP 更新禅道连接配置验证失败",
			"server", server, "account", account, "realm", realm, "error", err)
		return nil, fmt.Errorf("连接验证失败: %w", err)
	}
	if err := s.initService.StoreAuthConfigFromStruct(&initialization.AuthConfig{
		Username: account,
		Password: password,
		Domain:   server,
		Realm:    realm,
	}); err != nil {
		logger.Error("MCP 持久化禅道连接配置失败", "error", err)
		return nil, fmt.Errorf("登录成功但保存配置失败: %w", err)
	}
	// 清业务缓存：坏配置期写入的空结果会残留毒化健康期数据（与设置页同语义）。
	// client 层 SessionCache 已由 UpdateSessionConfig 的 UpdateNamespace 处理。
	if s.cacheClear != nil {
		if err := s.cacheClear(); err != nil {
			logger.Warn("MCP 清空业务缓存失败（不影响配置保存）", "error", err)
		}
	}
	return map[string]interface{}{
		"status": "ok",
		"message": fmt.Sprintf("连接成功，配置已保存：%s（%s，认证域 %s）。查询工具现在可用",
			s.client.GetServer(), s.client.GetAccount(), realmDisplay(s.client.GetRealm())),
		"data": map[string]interface{}{
			"server":    s.client.GetServer(),
			"account":   s.client.GetAccount(),
			"realm":     s.client.GetRealm(),
			"connected": s.client.IsConnected(),
		},
	}, nil
}

// handleTestConnection 实时探测禅道连通性（/user-profile.json 探针 + 延迟）。
// 返回结构化结果而非 error，便于 AI 拿到完整诊断信息。
func (s *MCPServer) handleTestConnection(_ map[string]interface{}) (interface{}, error) {
	start := time.Now()
	user, err := s.client.GetCurrentUser()
	latency := time.Since(start).Milliseconds()
	data := map[string]interface{}{
		"server":     s.client.GetServer(),
		"account":    s.client.GetAccount(),
		"latency_ms": latency,
	}
	if err != nil {
		data["connected"] = false
		data["error"] = err.Error()
		return map[string]interface{}{
			"status":  "ok",
			"message": fmt.Sprintf("禅道连接失败（%dms）：%s", latency, err.Error()),
			"data":    data,
		}, nil
	}
	data["connected"] = true
	data["realname"] = user.Realname
	return map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("禅道连接正常（%dms），当前账号 %s", latency, user.Account),
		"data":    data,
	}, nil
}

// handleSendVerifyCode 麒麟统一认证第一步：账号+密码触发短信验证码，返回脱敏手机号。
// 60s 冷却窗：SSO 平台对频繁发送会频控甚至临时封账号，防止 AI 循环重试刷爆配额。
func (s *MCPServer) handleSendVerifyCode(params map[string]interface{}) (interface{}, error) {
	sso, err := s.ssoClientOrError()
	if err != nil {
		return nil, err
	}
	username := stringFromParams(params, "username")
	password := stringFromParams(params, "password")
	if username == "" || password == "" {
		return nil, fmt.Errorf("username、password 不能为空")
	}
	if !s.tryClaimSSOSend(time.Now().UnixNano()) {
		return nil, fmt.Errorf("60 秒内已触发过验证码发送，请稍后再试（频繁发送会触发 SSO 频控甚至临时封号；未收到短信请先检查手机号而非重试）")
	}
	result, err := sso.SendCode(context.Background(), username, password)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("验证码已发送至 %s，收到后用 sso_login 提交", result.Mobile),
		"data":    map[string]interface{}{"mobile": result.Mobile},
	}, nil
}

// handleSSOLogin 麒麟统一认证第二步：账号+密码+短信验证码完成登录，返回认证账号。
func (s *MCPServer) handleSSOLogin(params map[string]interface{}) (interface{}, error) {
	sso, err := s.ssoClientOrError()
	if err != nil {
		return nil, err
	}
	username := stringFromParams(params, "username")
	password := stringFromParams(params, "password")
	code := stringFromParams(params, "code")
	if username == "" || password == "" || code == "" {
		return nil, fmt.Errorf("username、password、code 不能为空")
	}
	account, err := sso.DoSSOLogin(context.Background(), username, password, code)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("SSO 登录成功：%s（凭据有效。注意：频繁登录会触发 SSO 限流）", account),
		"data":    map[string]interface{}{"account": account},
	}, nil
}

// ssoClientOrError SSO 通道未注入（auth.sso.enabled=false）时的统一报错。
func (s *MCPServer) ssoClientOrError() (*auth.SSOClient, error) {
	if s.ssoClient == nil {
		return nil, fmt.Errorf("SSO 登录未启用（auth.sso.enabled=false 或未配置 base_url），请改用禅道账号密码直接配置连接")
	}
	return s.ssoClient, nil
}

// realmDisplay 认证域的展示语义（空 = 标准 REST Token 模式）。
func realmDisplay(realm string) string {
	if realm == "" {
		return "标准 REST"
	}
	return realm
}

// probeZentaoReachable TCP 拨号预检。与 handlers 包的 probeZentaoReachable 同逻辑；
// 不跨包复用是为了避免 stdio 入口经 mcp → handlers 引入 hertz 依赖。
func probeZentaoReachable(rawDomain string, timeout time.Duration) error {
	domain := rawDomain
	if !strings.HasPrefix(domain, "http://") && !strings.HasPrefix(domain, "https://") {
		domain = "http://" + domain
	}
	u, err := url.Parse(domain)
	if err != nil || u.Hostname() == "" {
		return fmt.Errorf("域名格式无效: %s", rawDomain)
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(u.Hostname(), port), timeout)
	if err != nil {
		return fmt.Errorf("无法连接到 %s（%s 端口 %v 内无响应），请检查域名或网络/VPN", rawDomain, port, timeout)
	}
	_ = conn.Close()
	return nil
}

package mcp

import (
	"testing"
	"time"
)

// 基础配置/认证工具的参数校验路径（不触网）。
// 依赖注入缺失与必填参数缺失必须在触达网络前给出明确错误。

func TestUpdateZentaoConfigRequiresInitService(t *testing.T) {
	s := &MCPServer{} // 未注入 initService
	if _, err := s.handleUpdateZentaoConfig(map[string]interface{}{
		"server": "https://x.example.com", "account": "a", "password": "p",
	}); err == nil {
		t.Fatal("expected error when initService not injected")
	}
}

func TestSendVerifyCodeRequiresSSOClient(t *testing.T) {
	s := &MCPServer{} // 未注入 ssoClient（auth.sso.enabled=false 场景）
	if _, err := s.handleSendVerifyCode(map[string]interface{}{
		"username": "u", "password": "p",
	}); err == nil {
		t.Fatal("expected error when ssoClient not injected")
	}
	if _, err := s.handleSSOLogin(map[string]interface{}{
		"username": "u", "password": "p", "code": "123456",
	}); err == nil {
		t.Fatal("expected error when ssoClient not injected")
	}
}

func TestConfigToolsAreClassifiedAsWrite(t *testing.T) {
	for _, name := range []string{"update_zentao_config", "send_verify_code", "sso_login"} {
		if !IsWriteAction(name) || !IsWriteTool(name) {
			t.Errorf("%s should be classified as write (read_only must block it)", name)
		}
		if GetToolByName(name) == nil {
			t.Errorf("%s missing from Tools definitions", name)
		}
	}
	for _, name := range []string{"get_zentao_config", "test_connection"} {
		if IsWriteAction(name) {
			t.Errorf("%s should be readable in read_only mode", name)
		}
		if GetToolByName(name) == nil {
			t.Errorf("%s missing from Tools definitions", name)
		}
	}
}

func TestSSOSendCooldown(t *testing.T) {
	s := &MCPServer{}
	now := time.Now().UnixNano()
	if !s.tryClaimSSOSend(now) {
		t.Fatal("first claim should pass")
	}
	if s.tryClaimSSOSend(now + int64(time.Second)) {
		t.Fatal("claim within 60s cooldown should be rejected")
	}
	if !s.tryClaimSSOSend(now + int64(61*time.Second)) {
		t.Fatal("claim after cooldown should pass")
	}
}

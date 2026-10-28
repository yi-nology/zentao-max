package mcp

import (
	"sync"
	"testing"

	"github.com/yi-nology/zentao-max/backend/internal/config"
)

// resetManagerForTest 将全局单例重置为已知状态（enabled, no token, all actions）
// 由于 MCPModeManager 是进程级单例，测试需在开头显式设置状态.
func resetManagerForTest() {
	m := GetMCPModeManager()
	m.InitFromConfig(config.MCPConfig{
		Enabled:   true,
		Transport: TransportHTTP,
		Token:     "",
		ReadOnly:  false,
		Actions:   nil,
	})
}

func TestMCPModeManager_InitFromConfig(t *testing.T) {
	resetManagerForTest()

	m := GetMCPModeManager()
	m.InitFromConfig(config.MCPConfig{
		Enabled:   true,
		Transport: TransportHTTP,
		Token:     "secret",
		ReadOnly:  true,
		Actions:   []string{"get_bugs", "get_tasks"},
	})

	if !m.IsEnabled() {
		t.Error("expected enabled=true")
	}
	if !m.IsReadOnly() {
		t.Error("expected readOnly=true")
	}
	if !m.HasToken() {
		t.Error("expected token set")
	}
	if m.Transport() != TransportHTTP {
		t.Errorf("expected transport=http, got %s", m.Transport())
	}
	if !m.IsActionAllowed("get_bugs") {
		t.Error("get_bugs should be allowed")
	}
	if m.IsActionAllowed("get_products") {
		t.Error("get_products should NOT be allowed (not in whitelist)")
	}
}

func TestMCPModeManager_TokenVerification(t *testing.T) {
	resetManagerForTest()

	m := GetMCPModeManager()
	m.InitFromConfig(config.MCPConfig{Enabled: true, Token: "my-secret-token"})

	// 正确 token
	if !m.VerifyToken("my-secret-token") {
		t.Error("correct token should verify")
	}
	// 错误 token
	if m.VerifyToken("wrong-token") {
		t.Error("wrong token should NOT verify")
	}
	// 空 token
	if m.VerifyToken("") {
		t.Error("empty token should NOT verify when token is configured")
	}
}

func TestMCPModeManager_NoTokenAllowsAll(t *testing.T) {
	resetManagerForTest()

	m := GetMCPModeManager()
	m.InitFromConfig(config.MCPConfig{Enabled: true, Token: ""})

	if !m.VerifyToken("") {
		t.Error("empty token config should allow all (no auth)")
	}
	if !m.VerifyToken("anything") {
		t.Error("empty token config should allow any token")
	}
}

func TestMCPModeManager_UpdateStatus(t *testing.T) {
	resetManagerForTest()

	m := GetMCPModeManager()
	m.InitFromConfig(config.MCPConfig{Enabled: true, Token: "", ReadOnly: false, Actions: nil})

	// 部分更新：禁用
	enabledFalse := false
	m.UpdateStatus(MCPUpdateRequest{Enabled: &enabledFalse})
	if m.IsEnabled() {
		t.Error("expected disabled after update")
	}

	// 重新启用 + 设置 token + 只读
	enabledTrue := true
	readOnlyTrue := true
	token := "new-token"
	m.UpdateStatus(MCPUpdateRequest{Enabled: &enabledTrue, ReadOnly: &readOnlyTrue, Token: &token})
	if !m.IsEnabled() || !m.IsReadOnly() || !m.VerifyToken("new-token") {
		t.Error("update did not apply correctly")
	}

	// 清空 token（关闭鉴权）
	emptyToken := ""
	m.UpdateStatus(MCPUpdateRequest{Token: &emptyToken})
	if m.HasToken() {
		t.Error("setting empty token should disable auth")
	}
}

func TestMCPModeManager_UpdateActions(t *testing.T) {
	resetManagerForTest()

	m := GetMCPModeManager()
	m.InitFromConfig(config.MCPConfig{Enabled: true, Actions: nil})

	// 默认全部允许
	if !m.IsActionAllowed("get_anything") {
		t.Error("empty actions should allow all")
	}

	// 设置白名单
	actions := []string{"get_bugs"}
	m.UpdateStatus(MCPUpdateRequest{Actions: &actions})
	if !m.IsActionAllowed("get_bugs") {
		t.Error("get_bugs should be allowed after whitelist update")
	}
	if m.IsActionAllowed("get_tasks") {
		t.Error("get_tasks should NOT be allowed (not in whitelist)")
	}

	// 清空白名单（允许全部）
	emptyActions := []string{}
	m.UpdateStatus(MCPUpdateRequest{Actions: &emptyActions})
	if !m.IsActionAllowed("get_anything") {
		t.Error("empty whitelist should allow all again")
	}
}

// TestMCPModeManager_ConcurrentAccess 并发读写测试（需配合 -race 运行）.
func TestMCPModeManager_ConcurrentAccess(t *testing.T) {
	resetManagerForTest()

	m := GetMCPModeManager()
	m.InitFromConfig(config.MCPConfig{Enabled: true, Token: "concurrent", ReadOnly: false})

	var wg sync.WaitGroup
	enabledVal := true

	// 并发写
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.UpdateStatus(MCPUpdateRequest{Enabled: &enabledVal})
		}()
	}

	// 并发读
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = m.IsEnabled()
			_ = m.VerifyToken("concurrent")
			_ = m.GetStatus()
			_ = m.IsActionAllowed("get_bugs")
		}()
	}

	wg.Wait()
}

func TestIsWriteAction(t *testing.T) {
	// 当前全部为读操作
	readActions := []string{"ping", "get_products", "get_projects", "get_bugs", "get_stories", "get_tasks", "get_users", "get_timelog"}
	for _, a := range readActions {
		if IsWriteAction(a) {
			t.Errorf("action %s should NOT be write", a)
		}
	}

	// 预留的写操作
	writeActions := []string{"create_bug", "update_task", "delete_project"}
	for _, a := range writeActions {
		if !IsWriteAction(a) {
			t.Errorf("action %s should be write", a)
		}
	}
}

func TestHashToken(t *testing.T) {
	// 空 token 返回空串
	if h := hashToken(""); h != "" {
		t.Errorf("empty token should hash to empty string, got %s", h)
	}
	if h := hashToken("   "); h != "" {
		t.Errorf("whitespace token should hash to empty string, got %s", h)
	}

	// 相同 token 产生相同哈希
	h1 := hashToken("test-token")
	h2 := hashToken("test-token")
	if h1 != h2 {
		t.Error("same token should produce same hash")
	}

	// 不同 token 产生不同哈希
	h3 := hashToken("other-token")
	if h1 == h3 {
		t.Error("different tokens should produce different hashes")
	}

	// 哈希长度固定（SHA-256 hex = 64 字符）
	if len(h1) != 64 {
		t.Errorf("expected 64-char hex hash, got %d", len(h1))
	}
}

package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yi-nology/zentao-max/backend/core/config"
)

// TestMCPUpdateRequest_PartialUpdateJSON 验证 admin PUT 接口的 JSON 部分更新语义
// 省略的字段应解析为 nil（不修改），这是热重载 API 最易错的地方.
func TestMCPUpdateRequest_PartialUpdateJSON(t *testing.T) {
	resetManagerForTest()
	m := GetMCPModeManager()
	m.InitFromConfig(config.MCPConfig{
		Enabled:  true,
		ReadOnly: false,
		Token:    "initial",
	})

	// 场景1：只更新 enabled
	body1 := `{"enabled": false}`
	var req1 MCPUpdateRequest
	if err := json.Unmarshal([]byte(body1), &req1); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if req1.Enabled == nil || *req1.Enabled != false {
		t.Error("enabled should be parsed as false")
	}
	if req1.ReadOnly != nil {
		t.Error("readOnly should be nil (omitted)")
	}
	if req1.Token != nil {
		t.Error("token should be nil (omitted)")
	}
	if req1.Actions != nil {
		t.Error("actions should be nil (omitted)")
	}

	m.UpdateStatus(req1)
	if m.IsEnabled() {
		t.Error("enabled should be false after update")
	}
	if !m.HasToken() {
		t.Error("token should remain set (not modified)")
	}

	// 场景2：更新 token 为空串（关闭鉴权）
	body2 := `{"token": ""}`
	var req2 MCPUpdateRequest
	if err := json.Unmarshal([]byte(body2), &req2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if req2.Token == nil {
		t.Error("token field present but empty should parse as *string(\"\"), not nil")
	}
	m.UpdateStatus(req2)
	if m.HasToken() {
		t.Error("setting empty token should disable auth")
	}

	// 场景3：actions 空数组表示允许全部
	body3 := `{"actions": []}`
	var req3 MCPUpdateRequest
	if err := json.Unmarshal([]byte(body3), &req3); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if req3.Actions == nil {
		t.Error("actions [] should parse as non-nil empty slice, not nil")
	}
	m.UpdateStatus(req3)
	if !m.IsActionAllowed("any_action") {
		t.Error("empty actions slice should allow all")
	}
}

// TestMCPAdminHandler_GetUpdateRoundTrip 验证 admin handler 的业务逻辑往返：
// Update 后 GetStatus 立即反映新状态（热重载生效）.
func TestMCPAdminHandler_GetUpdateRoundTrip(t *testing.T) {
	resetManagerForTest()
	handler := NewMCPAdminHandler()

	// 初始状态
	status := handler.mgr.GetStatus()
	if !status.Enabled {
		t.Error("expected enabled by default")
	}

	// 通过 UpdateRequest 禁用 + 设置只读
	enabled := false
	readOnly := true
	handler.mgr.UpdateStatus(MCPUpdateRequest{Enabled: &enabled, ReadOnly: &readOnly})

	// 立即查询应反映新状态
	status = handler.mgr.GetStatus()
	if status.Enabled {
		t.Error("expected disabled after update")
	}
	if !status.ReadOnly {
		t.Error("expected readOnly after update")
	}
}

// TestMCPStatus_TokenNeverReturned 验证状态快照永不泄露 token 明文.
func TestMCPStatus_TokenNeverReturned(t *testing.T) {
	resetManagerForTest()
	m := GetMCPModeManager()
	m.InitFromConfig(config.MCPConfig{
		Enabled: true,
		Token:   "super-secret",
	})

	status := m.GetStatus()
	if status.TokenSet != true {
		t.Error("TokenSet should be true when token is configured")
	}
	// MCPStatus 结构体无 Token 字段，编译期已保证不泄露；运行期再断言一次
	data, _ := json.Marshal(status)
	if strings.Contains(string(data), "super-secret") {
		t.Error("status snapshot must not leak token plaintext")
	}
}

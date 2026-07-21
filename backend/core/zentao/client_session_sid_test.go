package zentao

import (
	"strings"
	"testing"
)

// TestSessionTransport_SetZentaoSID_Empty 拒绝空字符串，避免注入空 cookie。
func TestSessionTransport_SetZentaoSID_Empty(t *testing.T) {
	s := &SessionTransport{} // cookieJar nil 也会先拦
	err := s.SetZentaoSID("")
	if err == nil {
		t.Fatal("expected error for empty zentaosid")
	}
	if !strings.Contains(err.Error(), "不能为空") {
		t.Errorf("expected '不能为空', got: %v", err)
	}
}

// TestSessionTransport_SetZentaoSID_NoJar cookieJar 未初始化时返明确错误。
func TestSessionTransport_SetZentaoSID_NoJar(t *testing.T) {
	s := &SessionTransport{server: "https://your-zentao-server.com"}
	err := s.SetZentaoSID("abc123")
	if err == nil {
		t.Fatal("expected error when cookie jar not initialized")
	}
	if !strings.Contains(err.Error(), "cookie jar") {
		t.Errorf("expected 'cookie jar' in error, got: %v", err)
	}
}

// TestSessionTransport_SetZentaoSID_Realistic 模拟 client.NewSessionClient
// 真实初始化路径（带 cookieJar + httpClient），验证 SetZentaoSID 后 jar 里有值。
func TestSessionTransport_SetZentaoSID_Realistic(t *testing.T) {
	s := newTestSessionTransport(t)

	const sid = "08e4dde1d0718437ca8f6fc2ea865a9d"
	if err := s.SetZentaoSID(sid); err != nil {
		// 一些内网域名会拒绝 SetCookies（publicsuffix），但兜底重建 jar 仍应该成功
		// 这里不 fail，打印观察
		t.Logf("SetZentaoSID: %v (内网域名 SetCookies 失败可能正常)", err)
	}

	// 不管 SetZentaosid 内部路径如何，最终 jar 里必须有 zentaosid
	if !s.hasZentaoSID() {
		t.Error("hasZentaoSID should be true after SetZentaoSID")
	}

	// IsLoggedIn 应该 true
	if !s.IsLoggedIn() {
		t.Error("IsLoggedIn should be true after SetZentaoSID")
	}
}

// TestClient_SetZentaoSID public API 端到端验证。
func TestClient_SetZentaoSID(t *testing.T) {
	c := NewSessionClient(
		"https://your-zentao-server.com",
		"<user-account>",
		"test-password",
		"kydc",
		"",  // DBPrefix 空 → L1 only, 不创建 SQLite
		nil, // pool nil
	)

	if err := c.SetZentaoSID(""); err == nil {
		t.Error("empty sid should fail")
	}

	const sid = "abc123def456"
	if err := c.SetZentaoSID(sid); err != nil {
		t.Fatalf("SetZentaoSID: %v", err)
	}
	if !c.IsConnected() {
		t.Error("IsConnected should be true after SetZentaoSID")
	}
}

// newTestSessionTransport 构造一个跟 client.NewSessionClient 路径一致的
// SessionTransport，server 用受限实例（内网域名），用于 SetZentaoSID 真实路径测。
func newTestSessionTransport(t *testing.T) *SessionTransport {
	t.Helper()
	return NewSessionTransport(
		"https://your-zentao-server.com",
		"<user-account>",
		"test-password",
		"kydc",
	)
}

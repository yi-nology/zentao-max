package initialization

import (
	"os"
	"path/filepath"
	"testing"
)

// TestStoreAndLoadAuthConfigWithRealm 验证登录表单的持久化往返：
// StoreAuthConfigFromStruct（AES 加密落盘）→ LoadAuthConfig（解密读取），
// 确保 realm 字段正确保留，重启后能恢复会话模式。
func TestStoreAndLoadAuthConfigWithRealm(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "auth.db")
	svc := NewInitService(dbPath, "test-encryption-key-for-unit-test")

	original := &AuthConfig{
		Username: "<user-account>",
		Password: "REDACTED",
		Domain:   "https://your-zentao-server.com",
		Realm:    "kydc",
	}

	if err := svc.StoreAuthConfigFromStruct(original); err != nil {
		t.Fatalf("StoreAuthConfigFromStruct failed: %v", err)
	}

	// 文件应存在且非空（加密内容）。
	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("auth.db not written: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("auth.db is empty")
	}

	loaded, _, err := svc.LoadAuthConfig()
	if err != nil {
		t.Fatalf("LoadAuthConfig failed: %v", err)
	}
	if loaded.Username != original.Username {
		t.Errorf("Username: want %q, got %q", original.Username, loaded.Username)
	}
	if loaded.Password != original.Password {
		t.Errorf("Password: want %q, got %q", original.Password, loaded.Password)
	}
	if loaded.Domain != original.Domain {
		t.Errorf("Domain: want %q, got %q", original.Domain, loaded.Domain)
	}
	if loaded.Realm != original.Realm {
		t.Errorf("Realm: want %q, got %q (会话模式重启后无法恢复)", original.Realm, loaded.Realm)
	}
}

// TestStoreAuthConfigWithoutRealm 验证 realm 为空（Token 模式）时往返正常，
// 即新增字段不破坏旧配置的兼容性。
func TestStoreAuthConfigWithoutRealm(t *testing.T) {
	dir := t.TempDir()
	svc := NewInitService(filepath.Join(dir, "auth.db"), "key")

	original := &AuthConfig{
		Username: "admin",
		Password: "secret",
		Domain:   "https://demo.zentao.net",
		// Realm 留空 = Token 模式
	}
	if err := svc.StoreAuthConfigFromStruct(original); err != nil {
		t.Fatalf("store failed: %v", err)
	}
	loaded, _, err := svc.LoadAuthConfig()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.Realm != "" {
		t.Errorf("Realm should be empty for token mode, got %q", loaded.Realm)
	}
}

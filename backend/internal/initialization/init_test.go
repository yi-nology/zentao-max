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
		Password: "test-password-123",
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

// TestManagedKeyFileStableAcrossInstances 验证托管密钥文件方案：
// 无显式密钥时首次生成 data/encryption.key（0600），之后跨实例稳定——
// 这是 hostname 漂移导致 auth.db 反复解密失败的根治。
func TestManagedKeyFileStableAcrossInstances(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "auth.db")

	// 实例 1 写入（生成密钥文件）
	svc1 := NewInitService(dbPath, "")
	original := &AuthConfig{Username: "u", Password: "p", Domain: "https://x.example.com", Realm: "kydc"}
	if err := svc1.StoreAuthConfigFromStruct(original); err != nil {
		t.Fatalf("store failed: %v", err)
	}

	// 密钥文件存在、权限 0600
	keyPath := filepath.Join(dir, keyFileName)
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("encryption.key not created: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("encryption.key perm = %v, want 0600", info.Mode().Perm())
	}

	// 实例 2（新进程模拟）读取成功——密钥文件保证跨实例稳定
	svc2 := NewInitService(dbPath, "")
	loaded, _, err := svc2.LoadAuthConfig()
	if err != nil {
		t.Fatalf("load with fresh instance failed: %v", err)
	}
	if loaded.Username != original.Username || loaded.Realm != original.Realm {
		t.Errorf("roundtrip mismatch: %+v", loaded)
	}
}

// TestLegacyMachineKeyFallback 验证历史兼容：密钥文件方案启用前
// 用旧机器派生密钥写入的 auth.db，新实例可通过 legacy 回退解密。
func TestLegacyMachineKeyFallback(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "auth.db")

	// 模拟旧版本：直接以机器派生密钥为主密钥写入
	oldSvc := &InitService{dbPath: dbPath, encryptionKey: deriveLegacyMachineKey()}
	original := &AuthConfig{Username: "legacy-user", Password: "lp", Domain: "https://old.example.com"}
	if err := oldSvc.StoreAuthConfigFromStruct(original); err != nil {
		t.Fatalf("legacy store failed: %v", err)
	}

	// 新实例（托管密钥文件新生成，与 legacy 不同）回退解密成功
	newSvc := NewInitService(dbPath, "")
	loaded, _, err := newSvc.LoadAuthConfig()
	if err != nil {
		t.Fatalf("legacy fallback load failed: %v", err)
	}
	if loaded.Username != original.Username {
		t.Errorf("Username: want %q, got %q", original.Username, loaded.Username)
	}
}

// TestKeyFileDeletionStartsFresh 验证密钥文件被删后的预期行为：
// 生成新密钥，旧 auth.db 解不开（需重新配置连接）——失败模式显式可控，
// 不再随网络环境静默漂移。
func TestKeyFileDeletionStartsFresh(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "auth.db")

	svc1 := NewInitService(dbPath, "")
	if err := svc1.StoreAuthConfigFromStruct(&AuthConfig{Username: "u", Password: "p", Domain: "d"}); err != nil {
		t.Fatalf("store failed: %v", err)
	}

	// 删除密钥文件 → 新实例拿到不同密钥 → 旧文件解密失败
	if err := os.Remove(filepath.Join(dir, keyFileName)); err != nil {
		t.Fatalf("remove key file: %v", err)
	}
	svc2 := NewInitService(dbPath, "")
	if _, _, err := svc2.LoadAuthConfig(); err == nil {
		t.Fatal("expected decrypt failure after key file deletion (different random key)")
	}
}

// TestLegacyAutoMigration 验证回退命中后自动迁移：首次 LoadAuthConfig 用
// legacy 机器密钥解开历史 auth.db 后，立即用主密钥（托管密钥文件）原子重写，
// 下次读取不再依赖会漂移的机器派生密钥。
func TestLegacyAutoMigration(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "auth.db")

	// 模拟历史文件：旧机器派生密钥加密
	oldSvc := &InitService{dbPath: dbPath, encryptionKey: deriveLegacyMachineKey()}
	original := &AuthConfig{Username: "legacy-u", Password: "lp", Domain: "https://old.example.com", Realm: "kydc"}
	if err := oldSvc.StoreAuthConfigFromStruct(original); err != nil {
		t.Fatalf("legacy store failed: %v", err)
	}

	// 新实例首次加载：回退解密 + 自动迁移
	newSvc := NewInitService(dbPath, "")
	loaded, _, err := newSvc.LoadAuthConfig()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.Username != original.Username {
		t.Errorf("Username mismatch after migration: %q", loaded.Username)
	}

	// 迁移后文件应以主密钥加密：直接用主密钥构造裸实例即可解开（无需回退）
	direct := &InitService{dbPath: dbPath, encryptionKey: newSvc.encryptionKey}
	if _, _, err := direct.LoadAuthConfig(); err != nil {
		t.Fatalf("file not migrated to managed key: %v", err)
	}
}

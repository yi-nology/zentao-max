package initialization

import (
	"path/filepath"
	"testing"
)

// TestUpdateAuthConfig_MergesFields auth.db 是单条加密记录，连接配置与 SSO 配置
// 同住一处——UpdateAuthConfig 必须读改写合并，任何一方保存都不能抹掉另一方。
// 背景：两处 update_zentao_config 历史上都是「新建只含禅道字段的记录整体覆盖」，
// SSO 字段引入后若不同步改为合并写，当天的第一次连接配置保存就会清掉 SSO 配置。
func TestUpdateAuthConfig_MergesFields(t *testing.T) {
	dir := t.TempDir()
	svc := NewInitService(filepath.Join(dir, "auth.db"), "test-merge-key")

	// 1. 先写禅道连接字段
	if err := svc.UpdateAuthConfig(func(cfg *AuthConfig) {
		cfg.Username = "user01"
		cfg.Password = "pw-zentao"
		cfg.Domain = "https://zentao.example.com"
		cfg.Realm = "kydc"
	}); err != nil {
		t.Fatalf("写入禅道字段失败: %v", err)
	}

	// 2. 再写 SSO 字段（另一条保存路径）
	if err := svc.UpdateAuthConfig(func(cfg *AuthConfig) {
		cfg.SSOManaged = true
		cfg.SSOEnabled = true
		cfg.SSOBaseURL = "https://sso.example.com"
		cfg.SSOAuthPageURL = "https://sso.example.com/sso/auth"
	}); err != nil {
		t.Fatalf("写入 SSO 字段失败: %v", err)
	}

	// 3. 双方字段都必须还在
	got, _, err := svc.LoadAuthConfig()
	if err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	if got.Username != "user01" || got.Domain != "https://zentao.example.com" || got.Realm != "kydc" {
		t.Errorf("禅道字段被 SSO 保存抹掉: %+v", got)
	}
	if !got.SSOManaged || !got.SSOEnabled || got.SSOBaseURL != "https://sso.example.com" {
		t.Errorf("SSO 字段未持久化: %+v", got)
	}

	// 4. 反向：再保存一次禅道连接（模拟用户改连接配置），SSO 字段不能丢
	if err := svc.UpdateAuthConfig(func(cfg *AuthConfig) {
		cfg.Username = "user01"
		cfg.Password = "pw-zentao2"
		cfg.Domain = "https://zentao2.example.com"
		cfg.Realm = ""
	}); err != nil {
		t.Fatalf("二次写入禅道字段失败: %v", err)
	}
	got, _, err = svc.LoadAuthConfig()
	if err != nil {
		t.Fatalf("二次回读失败: %v", err)
	}
	if !got.SSOManaged || got.SSOBaseURL != "https://sso.example.com" {
		t.Errorf("禅道保存抹掉了 SSO 字段: %+v", got)
	}
	if got.Password != "pw-zentao2" {
		t.Errorf("禅道字段未更新: %+v", got)
	}
}

// TestUpdateAuthConfig_EmptyDB 空库（无 auth.db）时 UpdateAuthConfig 从空记录开始，
// 不应因读失败而报错。
func TestUpdateAuthConfig_EmptyDB(t *testing.T) {
	dir := t.TempDir()
	svc := NewInitService(filepath.Join(dir, "auth.db"), "test-empty-key")
	if err := svc.UpdateAuthConfig(func(cfg *AuthConfig) {
		cfg.SSOManaged = true
		cfg.SSOBaseURL = "https://sso.example.com"
	}); err != nil {
		t.Fatalf("空库上写入失败: %v", err)
	}
	got, _, err := svc.LoadAuthConfig()
	if err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	if !got.SSOManaged || got.SSOBaseURL != "https://sso.example.com" {
		t.Errorf("字段未持久化: %+v", got)
	}
}

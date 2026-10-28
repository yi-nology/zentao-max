package initialization

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSaveLoadZentaoSID_RoundTrip 验证保存到磁盘 + 读回 一致。
// 文件位置：t.TempDir()（隔离，不污染 ~/.zentao-max/）。
func TestSaveLoadZentaoSID_RoundTrip(t *testing.T) {
	origPath, err := ZentaoSIDFilePath()
	if err != nil {
		t.Fatalf("get path: %v", err)
	}
	// 用 t.TempDir() 隔离
	tmp := t.TempDir()
	realSid := "08e4dde1d0718437ca8f6fc2ea865a9d"

	// 临时覆盖 home 目录指向 tmp，Save/Load 用的就是 tmp/.zentao-max/zentaosid.json
	t.Setenv("HOME", tmp)
	// 验证路径确实变了
	newPath, _ := ZentaoSIDFilePath()
	if newPath == origPath {
		t.Skipf("HOME override 没生效: orig=%s new=%s", origPath, newPath)
	}

	if err := SaveZentaoSID(realSid, "test note"); err != nil {
		t.Fatalf("save: %v", err)
	}

	// 验证文件存在 + 权限 0600
	info, err := os.Stat(newPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("file perm: got %o want 0600", info.Mode().Perm())
	}

	loaded, err := LoadZentaoSID()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded != realSid {
		t.Errorf("got %q want %q", loaded, realSid)
	}

	// 验证内容（zentaosid 在 file 里，不能硬编码全文比对 JSON 格式会带时间戳）
	data, _ := os.ReadFile(newPath)
	if !strings.Contains(string(data), realSid) {
		t.Errorf("file content should contain zentaosid, got: %s", string(data))
	}
}

// TestLoadZentaoSID_NotFound 没文件时返明确错误。
func TestLoadZentaoSID_NotFound(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	_, err := LoadZentaoSID()
	if err == nil {
		t.Fatal("expected error when file not found")
	}
	if !os.IsNotExist(err) && !strings.Contains(err.Error(), "no such file") {
		t.Logf("expected ENOENT, got: %v", err)
	}
}

// TestLoadZentaoSID_Empty 拒绝空 sid（避免 silent 静默注入空 cookie）。
func TestLoadZentaoSID_Empty(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	path := filepath.Join(tmp, ".zentao-max", "zentaosid.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"zentaosid":"","updatedAt":"2026-01-01T00:00:00Z"}`), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadZentaoSID()
	if err == nil {
		t.Fatal("expected error for empty zentaosid")
	}
	if !strings.Contains(err.Error(), "为空") {
		t.Errorf("expected '为空' in error, got: %v", err)
	}
}

// TestDeleteZentaoSID 删存在 / 不存在 都不报错。
func TestDeleteZentaoSID(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	path := filepath.Join(tmp, ".zentao-max", "zentaosid.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"zentaosid":"abc","updatedAt":"2026-01-01T00:00:00Z"}`), 0600); err != nil {
		t.Fatal(err)
	}

	// 存在 → 删除
	if err := DeleteZentaoSID(); err != nil {
		t.Fatalf("delete existing: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should be gone after delete")
	}

	// 不存在 → 不报错（幂等）
	if err := DeleteZentaoSID(); err != nil {
		t.Errorf("delete non-existent should be no-op, got: %v", err)
	}
}

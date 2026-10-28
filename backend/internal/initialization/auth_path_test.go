package initialization

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveAuthDBPath(t *testing.T) {
	t.Run("explicit path always wins", func(t *testing.T) {
		t.Setenv("ZENTAO_AUTH_DB_PATH", "/tmp/env-should-lose.db")
		dir := t.TempDir()
		t.Chdir(dir)
		if got := ResolveAuthDBPath("/custom/auth.db"); got != "/custom/auth.db" {
			t.Fatalf("期望显式路径胜出, got %q", got)
		}
	})

	t.Run("env overrides candidate scan", func(t *testing.T) {
		t.Setenv("ZENTAO_AUTH_DB_PATH", "/tmp/from-env.db")
		dir := t.TempDir()
		t.Chdir(dir)
		if got := ResolveAuthDBPath(""); got != "/tmp/from-env.db" {
			t.Fatalf("期望环境变量路径胜出, got %q", got)
		}
	})

	t.Run("picks existing legacy ./auth.db", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		if err := os.WriteFile("./auth.db", []byte(`{}`), 0600); err != nil {
			t.Fatal(err)
		}
		if got := ResolveAuthDBPath(""); got != "./auth.db" {
			t.Fatalf("期望回退到历史位置 ./auth.db, got %q", got)
		}
	})

	t.Run("prefers canonical ./data/auth.db over legacy", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		if err := os.MkdirAll("./data", 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile("./data/auth.db", []byte(`{}`), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile("./auth.db", []byte(`{}`), 0600); err != nil {
			t.Fatal(err)
		}
		if got := ResolveAuthDBPath(""); got != filepath.Join("data", "auth.db") && got != "./data/auth.db" {
			t.Fatalf("期望规范位置 ./data/auth.db 胜出, got %q", got)
		}
	})

	t.Run("nothing exists falls back to canonical default", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		if got := ResolveAuthDBPath(""); got != defaultAuthDBPath {
			t.Fatalf("期望默认规范位置 %q, got %q", defaultAuthDBPath, got)
		}
	})

	t.Run("empty file is skipped in scan", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		if err := os.WriteFile("./auth.db", []byte{}, 0600); err != nil {
			t.Fatal(err)
		}
		if got := ResolveAuthDBPath(""); got != defaultAuthDBPath {
			t.Fatalf("空文件不应参与候选, 期望 %q, got %q", defaultAuthDBPath, got)
		}
	})
}

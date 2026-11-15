package service

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	storage "github.com/yi-nology/zentao-max/backend/internal/repository/sqlite"
)

// TestStoreBootGeneration 回归：跨进程（重启前）写入的缓存不得作为
// Get/GetStale（SWR 旧值）返回，只能经 GetAnyStale 作离线兜底。
// 真实故障：重启后 dashboard 30 分钟内返回上次启动的 bugs=0/stories=旧值。
func TestStoreBootGeneration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "cache.db")

	s1, err := storage.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("store1: %v", err)
	}
	if err := s1.Set(context.Background(), "dashboard::", 0, []byte(`{"bugs":{"total":0}}`), 30*time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}
	// 同进程内可读（新鲜）
	if d, _, err := s1.Get(context.Background(), "dashboard::", 0); err != nil || d == nil {
		t.Fatalf("same-boot Get should hit: data=%v err=%v", d, err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// 模拟重启：同库路径新进程（新 bootID）
	s2, err := storage.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("store2: %v", err)
	}
	defer s2.Close()

	if d, _, err := s2.Get(context.Background(), "dashboard::", 0); err != nil || d != nil {
		t.Errorf("cross-boot Get must miss (got %q, err=%v)", string(d), err)
	}
	if d, _, err := s2.GetStale(context.Background(), "dashboard::", 0); err != nil || d != nil {
		t.Errorf("cross-boot GetStale (SWR stale-first) must miss (got %q, err=%v)", string(d), err)
	}
	if d, _, err := s2.GetAnyStale(context.Background(), "dashboard::", 0); err != nil || d == nil {
		t.Errorf("GetAnyStale (offline fallback) must hit (got %v, err=%v)", d, err)
	}
}

// TestStoreExpiryWithinSameBoot 同进程内过期条目：Get miss，GetStale 命中（SWR）。
func TestStoreExpiryWithinSameBoot(t *testing.T) {
	s, err := storage.NewSQLiteStore(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer s.Close()

	if err := s.Set(context.Background(), "dash:x:y", 1, []byte(`{"v":1}`), -time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}
	if d, _, err := s.Get(context.Background(), "dash:x:y", 1); err != nil || d != nil {
		t.Errorf("expired Get should miss: %v %v", d, err)
	}
	if d, _, err := s.GetStale(context.Background(), "dash:x:y", 1); err != nil || d == nil {
		t.Errorf("same-boot GetStale should hit: %v %v", d, err)
	}
}

// TestGetOrLoadSWRGuarded_SkipsCacheWhenNotCacheable 回归：不完整结果
// （cacheable=false）只返回给调用方、不写缓存；完整结果正常写缓存。
func TestGetOrLoadSWRGuarded_SkipsCacheWhenNotCacheable(t *testing.T) {
	store, err := storage.NewSQLiteStore(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer store.Close()
	cs := NewCacheService(store)

	ctx := context.Background()
	partial := []byte(`{"bugs":{"total":0},"incomplete":true}`)
	res, err := cs.GetOrLoadSWRGuarded(ctx, "dashboard::", 783, time.Minute,
		func(context.Context) ([]byte, error) { return partial, nil },
		func([]byte) bool { return false },
	)
	if err != nil {
		t.Fatalf("guarded load: %v", err)
	}
	if string(res.Data) != string(partial) {
		t.Errorf("result data mismatch: %s", res.Data)
	}
	time.Sleep(100 * time.Millisecond) // 等待潜在异步写（应为零）
	if d, _, _ := store.Get(ctx, "dashboard::", 783); d != nil {
		t.Errorf("partial result must NOT be cached, found %q", string(d))
	}

	full := []byte(`{"bugs":{"total":1246}}`)
	if _, err := cs.GetOrLoadSWRGuarded(ctx, "dashboard::", 783, time.Minute,
		func(context.Context) ([]byte, error) { return full, nil },
		func([]byte) bool { return true },
	); err != nil {
		t.Fatalf("guarded load #2: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	d, _, err := store.Get(ctx, "dashboard::", 783)
	if err != nil || d == nil {
		t.Fatalf("complete result should be cached, got %v err=%v", d, err)
	}
	if !json.Valid(d) {
		t.Errorf("cached data invalid json: %s", d)
	}
}

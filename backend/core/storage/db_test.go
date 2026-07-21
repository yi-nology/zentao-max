package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	// 用临时目录，测试结束自动清理
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test-cache.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	t.Cleanup(func() {
		store.Close()
		os.RemoveAll(dir)
	})
	return store
}

func TestSQLiteStore_SetAndGet(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	data := []byte(`{"bugs":[{"id":1}],"total":1}`)
	if err := store.Set(ctx, EntityBugs, 100, data, 10*time.Minute); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	gotData, gotCachedAt, err := store.Get(ctx, EntityBugs, 100)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if gotData == nil {
		t.Fatal("Get() returned nil data, expected cached")
	}
	if string(gotData) != string(data) {
		t.Errorf("Get() data = %s, want %s", gotData, data)
	}
	if gotCachedAt.IsZero() {
		t.Error("Get() cachedAt is zero")
	}
}

func TestSQLiteStore_MissReturnsNil(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	gotData, _, err := store.Get(ctx, EntityBugs, 999)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if gotData != nil {
		t.Errorf("Get() for non-existent returned data %v, want nil", gotData)
	}
}

func TestSQLiteStore_ExpiredReturnsNil(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// TTL 1 秒，然后等待过期
	if err := store.Set(ctx, EntityBugs, 100, []byte("data"), 1*time.Second); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// 等待 2 秒确保过期
	time.Sleep(2 * time.Second)

	gotData, _, err := store.Get(ctx, EntityBugs, 100)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if gotData != nil {
		t.Errorf("Get() after expiry returned data %v, want nil", gotData)
	}
}

func TestSQLiteStore_Upsert(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// 第一次写入
	store.Set(ctx, EntityBugs, 100, []byte("v1"), 10*time.Minute)
	// 第二次写入相同 key 应覆盖
	store.Set(ctx, EntityBugs, 100, []byte("v2"), 10*time.Minute)

	gotData, _, _ := store.Get(ctx, EntityBugs, 100)
	if string(gotData) != "v2" {
		t.Errorf("after upsert, got %s, want v2", gotData)
	}

	// 数据库应该只有 1 条
	status, _ := store.Status(ctx)
	if status.EntryCount != 1 {
		t.Errorf("entry count = %d, want 1 (should be upserted)", status.EntryCount)
	}
}

func TestSQLiteStore_Invalidate(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	store.Set(ctx, EntityBugs, 100, []byte("data"), 10*time.Minute)
	store.Set(ctx, EntityStories, 100, []byte("data"), 10*time.Minute)

	if err := store.Invalidate(ctx, EntityBugs, 100); err != nil {
		t.Fatalf("Invalidate() error = %v", err)
	}

	data, _, _ := store.Get(ctx, EntityBugs, 100)
	if data != nil {
		t.Error("Bugs should be invalidated")
	}
	// Stories 应该还在
	data2, _, _ := store.Get(ctx, EntityStories, 100)
	if data2 == nil {
		t.Error("Stories should still be cached")
	}
}

func TestSQLiteStore_ClearAll(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	store.Set(ctx, EntityBugs, 100, []byte("data"), 10*time.Minute)
	store.Set(ctx, EntityStories, 200, []byte("data"), 10*time.Minute)
	store.Set(ctx, EntityTasks, 300, []byte("data"), 10*time.Minute)

	if err := store.ClearAll(ctx); err != nil {
		t.Fatalf("ClearAll() error = %v", err)
	}

	status, _ := store.Status(ctx)
	if status.EntryCount != 0 {
		t.Errorf("after ClearAll, entry count = %d, want 0", status.EntryCount)
	}
}

func TestSQLiteStore_Status(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	store.Set(ctx, EntityBugs, 100, []byte("12345"), 10*time.Minute)

	status, err := store.Status(ctx)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.EntryCount != 1 {
		t.Errorf("EntryCount = %d, want 1", status.EntryCount)
	}
	// data 是 5 字节
	if status.TotalBytes != 5 {
		t.Errorf("TotalBytes = %d, want 5", status.TotalBytes)
	}
	if status.LastUpdateAt.IsZero() {
		t.Error("LastUpdateAt should not be zero")
	}
	if status.DBPath == "" {
		t.Error("DBPath should be set")
	}
}

func TestSQLiteStore_ConcurrentAccess(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// 10 个 goroutine 并发写入不同 key
	done := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			done <- store.Set(ctx, EntityBugs, idx, []byte("data"), 10*time.Minute)
		}(i)
	}
	for i := 0; i < 10; i++ {
		if err := <-done; err != nil {
			t.Errorf("concurrent Set error: %v", err)
		}
	}

	status, _ := store.Status(ctx)
	if status.EntryCount != 10 {
		t.Errorf("EntryCount = %d, want 10", status.EntryCount)
	}
}

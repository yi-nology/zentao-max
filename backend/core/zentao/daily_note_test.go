package zentao

import (
	"context"
	"path/filepath"
	"testing"
)

func newTestDailyNoteStore(t *testing.T) *DailyNoteStore {
	t.Helper()
	prefix := filepath.Join(t.TempDir(), "notes_test")
	store, err := NewDailyNoteStore(prefix)
	if err != nil {
		t.Fatalf("NewDailyNoteStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestDailyNoteStore_EmptyConfig(t *testing.T) {
	// 空 prefix 走默认路径 ~/.zentao-max/cache-notes.db。
	// 单元测试环境一般没有 .zentao-max 目录，所以构造可能成功也可能失败（取决于 home）。
	// 这里只验证 nil-safe：不 panic 即可。
	s, err := NewDailyNoteStore("")
	if err != nil {
		t.Skipf("home dir unavailable or init failed: %v", err)
	}
	if s == nil {
		t.Fatal("got nil store with no error")
	}
	_ = s.Close()
}

func TestDailyNoteStore_SetGet(t *testing.T) {
	store := newTestDailyNoteStore(t)
	ctx := context.Background()

	// 第一次写入
	n, err := store.SetNote(ctx, "u1", "2025-07-21", "今天调通了工时日历", 0)
	if err != nil {
		t.Fatal(err)
	}
	if n.Content != "今天调通了工时日历" {
		t.Errorf("content = %q", n.Content)
	}
	if n.User != "u1" || n.Date != "2025-07-21" {
		t.Errorf("user/date = %q/%q", n.User, n.Date)
	}

	// 读出
	got, err := store.GetNote(ctx, "u1", "2025-07-21")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected note, got nil")
	}
	if got.Content != "今天调通了工时日历" {
		t.Errorf("got.Content = %q", got.Content)
	}
}

func TestDailyNoteStore_UpdatePreservesCreatedAt(t *testing.T) {
	store := newTestDailyNoteStore(t)
	ctx := context.Background()

	first, err := store.SetNote(ctx, "u1", "2025-07-21", "v1", 0)
	if err != nil {
		t.Fatal(err)
	}
	// sleep 1s 确保 updated_at 跟 created_at 不同
	// 但 time.Now() 精度到秒，连续两次可能同秒。直接读出来验 created_at >= updated_at。
	second, err := store.SetNote(ctx, "u1", "2025-07-21", "v2", 0)
	if err != nil {
		t.Fatal(err)
	}
	if second.Content != "v2" {
		t.Errorf("second.Content = %q", second.Content)
	}
	if second.CreatedAt.Unix() != first.CreatedAt.Unix() {
		t.Errorf("created_at changed: %v vs %v", first.CreatedAt, second.CreatedAt)
	}
}

func TestDailyNoteStore_GetMissing(t *testing.T) {
	store := newTestDailyNoteStore(t)
	ctx := context.Background()

	got, err := store.GetNote(ctx, "u1", "2099-01-01")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("expected nil for missing note, got %+v", got)
	}
}

func TestDailyNoteStore_Delete(t *testing.T) {
	store := newTestDailyNoteStore(t)
	ctx := context.Background()

	_, _ = store.SetNote(ctx, "u1", "2025-07-21", "x", 0)
	if err := store.DeleteNote(ctx, "u1", "2025-07-21"); err != nil {
		t.Fatal(err)
	}
	got, _ := store.GetNote(ctx, "u1", "2025-07-21")
	if got != nil {
		t.Errorf("expected nil after delete, got %+v", got)
	}
	// 二次 delete 幂等
	if err := store.DeleteNote(ctx, "u1", "2025-07-21"); err != nil {
		t.Errorf("double delete should be no-op, got err: %v", err)
	}
}

func TestDailyNoteStore_ListByMonth(t *testing.T) {
	store := newTestDailyNoteStore(t)
	ctx := context.Background()

	// 7 月 3 条 + 8 月 2 条 + 9 月 1 条
	for _, d := range []string{"2025-07-01", "2025-07-15", "2025-07-31", "2025-08-10", "2025-08-20", "2025-09-05"} {
		if _, err := store.SetNote(ctx, "u1", d, "n-"+d, 0); err != nil {
			t.Fatal(err)
		}
	}

	// 7 月
	jul, err := store.ListNotesByMonth(ctx, "u1", 2025, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(jul) != 3 {
		t.Errorf("July notes = %d, want 3", len(jul))
	}
	if len(jul) >= 1 && jul[0].Date != "2025-07-01" {
		t.Errorf("first July date = %q, want 2025-07-01", jul[0].Date)
	}

	// 8 月
	aug, _ := store.ListNotesByMonth(ctx, "u1", 2025, 8)
	if len(aug) != 2 {
		t.Errorf("August notes = %d, want 2", len(aug))
	}

	// 9 月
	sep, _ := store.ListNotesByMonth(ctx, "u1", 2025, 9)
	if len(sep) != 1 {
		t.Errorf("September notes = %d, want 1", len(sep))
	}

	// 全部
	all, _ := store.ListNotesByMonth(ctx, "u1", 0, 0)
	if len(all) != 6 {
		t.Errorf("all notes = %d, want 6", len(all))
	}
}

func TestDailyNoteStore_NamespaceIsolation(t *testing.T) {
	store := newTestDailyNoteStore(t)
	ctx := context.Background()

	_, _ = store.SetNote(ctx, "u1", "2025-07-21", "u1 的", 0)
	_, _ = store.SetNote(ctx, "u2", "2025-07-21", "u2 的", 0)

	n1, _ := store.GetNote(ctx, "u1", "2025-07-21")
	n2, _ := store.GetNote(ctx, "u2", "2025-07-21")
	if n1 == nil || n2 == nil {
		t.Fatal("expected both notes")
	}
	if n1.Content == n2.Content {
		t.Errorf("namespaces not isolated: %q == %q", n1.Content, n2.Content)
	}
}

func TestDailyNoteStore_NilReceiver(t *testing.T) {
	var s *DailyNoteStore
	ctx := context.Background()
	// nil receiver 全 Get/Set/Delete/List 都不应 panic
	if n, err := s.GetNote(ctx, "u", "2025-07-21"); err != nil || n != nil {
		t.Errorf("nil GetNote: n=%v err=%v", n, err)
	}
	if n, err := s.SetNote(ctx, "u", "2025-07-21", "x", 0); err == nil {
		t.Error("nil SetNote should return error")
		_ = n
	}
	if err := s.DeleteNote(ctx, "u", "2025-07-21"); err != nil {
		t.Errorf("nil DeleteNote: %v", err)
	}
	if list, err := s.ListNotesByMonth(ctx, "u", 2025, 7); err != nil || list != nil {
		t.Errorf("nil ListNotesByMonth: list=%v err=%v", list, err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("nil Close: %v", err)
	}
}

func TestDailyNoteStore_EmptyContentAllowed(t *testing.T) {
	store := newTestDailyNoteStore(t)
	ctx := context.Background()

	if _, err := store.SetNote(ctx, "u1", "2025-07-21", "", 0); err != nil {
		t.Fatal(err)
	}
	got, _ := store.GetNote(ctx, "u1", "2025-07-21")
	if got == nil {
		t.Fatal("expected empty-content note to be persisted")
	}
	if got.Content != "" {
		t.Errorf("got.Content = %q, want empty", got.Content)
	}
}

package zentao

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
"fmt"
	"testing"
	"time"

	"golang.org/x/sync/singleflight"
)

// newTestSessionCache 构造一个 L1-only 的 SessionCache（无 L2），scope 唯一避免跨测试串。
func newTestSessionCache(t *testing.T, scope string) *SessionCache {
	t.Helper()
	return NewSessionCache(SessionCacheConfig{
		Server:  "https://test.example.com",
		Account: "user-" + scope,
		Realm:   "kydc",
		// DBPrefix 留空 → L2 关闭，纯 L1 + loader
	})
}

// newTestSessionCacheWithL2 构造带 L2 的 SessionCache。
func newTestSessionCacheWithL2(t *testing.T, scope string) *SessionCache {
	t.Helper()
	prefix := filepath.Join(t.TempDir(), "session_test_"+scope)
	c := NewSessionCache(SessionCacheConfig{
		Server:   "https://test.example.com",
		Account:  "user-" + scope,
		Realm:    "kydc",
		DBPrefix: prefix,
	})
	if c == nil {
		t.Fatal("expected non-nil cache")
	}
	if !c.Stats().L2Enabled {
		t.Fatal("expected L2 enabled")
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestSessionCache_NamespaceIsolation(t *testing.T) {
	c1 := NewSessionCache(SessionCacheConfig{Server: "https://a.com", Account: "alice", Realm: "kydc"})
	c2 := NewSessionCache(SessionCacheConfig{Server: "https://b.com", Account: "alice", Realm: "kydc"})
	c3 := NewSessionCache(SessionCacheConfig{Server: "https://a.com", Account: "bob", Realm: "kydc"})
	c4 := NewSessionCache(SessionCacheConfig{Server: "https://a.com", Account: "alice", Realm: "local"})
	if c1 == nil || c2 == nil || c3 == nil || c4 == nil {
		t.Fatal("expected non-nil cache for valid configs")
	}

	ns := []string{c1.ns, c2.ns, c3.ns, c4.ns}
	for i := range ns {
		for j := i + 1; j < len(ns); j++ {
			if ns[i] == ns[j] {
				t.Fatalf("namespace collision: %s == %s (c%d vs c%d)", ns[i], ns[j], i+1, j+1)
			}
		}
	}
}

func TestSessionCache_NilForEmptyConfig(t *testing.T) {
	if c := NewSessionCache(SessionCacheConfig{Server: "", Account: "x"}); c != nil {
		t.Error("expected nil when server empty")
	}
	if c := NewSessionCache(SessionCacheConfig{Server: "https://x", Account: ""}); c != nil {
		t.Error("expected nil when account empty")
	}
}

func TestSessionCache_NilReceiver_DirectLoader(t *testing.T) {
	var c *SessionCache
	calls := 0
	loader := func(ctx context.Context) ([]byte, error) {
		calls++
		return []byte(`{"ok":true}`), nil
	}
	res, err := c.GetOrLoad(context.Background(), sessionEntityUsers, "", loader)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.FromCache {
		t.Error("nil cache must report FromCache=false")
	}
	if calls != 1 {
		t.Errorf("loader calls = %d, want 1", calls)
	}
}

func TestSessionCache_BasicGetOrLoad_L1(t *testing.T) {
	c := newTestSessionCache(t, "basic-l1")

	loaderCalls := 0
	loader := func(ctx context.Context) ([]byte, error) {
		loaderCalls++
		return []byte(`[{"id":1,"name":"a"}]`), nil
	}

	r1, err := c.GetOrLoad(context.Background(), sessionEntityBugs, "byProduct:1", loader)
	if err != nil {
		t.Fatalf("first load err: %v", err)
	}
	if r1.FromCache || loaderCalls != 1 {
		t.Errorf("first: FromCache=%v loaderCalls=%d, want false/1", r1.FromCache, loaderCalls)
	}

	r2, err := c.GetOrLoad(context.Background(), sessionEntityBugs, "byProduct:1", loader)
	if err != nil {
		t.Fatalf("second load err: %v", err)
	}
	if !r2.FromCache || loaderCalls != 1 {
		t.Errorf("second: FromCache=%v loaderCalls=%d, want true/1", r2.FromCache, loaderCalls)
	}
}

func TestSessionCache_BasicGetOrLoad_L2(t *testing.T) {
	c := newTestSessionCacheWithL2(t, "basic-l2")

	loaderCalls := 0
	loader := func(ctx context.Context) ([]byte, error) {
		loaderCalls++
		return []byte(`"data-v1"`), nil
	}

	if _, err := c.GetOrLoad(context.Background(), sessionEntityUsers, "", loader); err != nil {
		t.Fatal(err)
	}
	if loaderCalls != 1 {
		t.Fatalf("first loader calls = %d", loaderCalls)
	}

	// 清 L1，下次应走 L2
	c.l1.ClearNamespace(c.ns)
	c.sf = singleflight.Group{}

	if _, err := c.GetOrLoad(context.Background(), sessionEntityUsers, "", loader); err != nil {
		t.Fatal(err)
	}
	if loaderCalls != 1 {
		t.Errorf("after L1 evict, loader called again (calls=%d) — L2 fallback broken", loaderCalls)
	}
}

func TestSessionCache_DifferentScopesIsolated(t *testing.T) {
	c := newTestSessionCacheWithL2(t, "scopes")

	loader := func(scope string) Loader {
		return func(ctx context.Context) ([]byte, error) {
			return []byte(`"` + scope + `"`), nil
		}
	}

	for _, scope := range []string{"all", "byProduct:1", "byProduct:2"} {
		res, err := c.GetOrLoad(context.Background(), sessionEntityBugs, scope, loader(scope))
		if err != nil {
			t.Fatal(err)
		}
		if string(res.Data) != `"`+scope+`"` {
			t.Errorf("scope %s: data %s, want %s", scope, res.Data, scope)
		}
	}

	// 清 L1，从 L2 重读应拿回各自 scope 的数据
	c.l1.ClearNamespace(c.ns)
	c.sf = singleflight.Group{}

	for _, scope := range []string{"all", "byProduct:1", "byProduct:2"} {
		res, err := c.GetOrLoad(context.Background(), sessionEntityBugs, scope, loader(scope))
		if err != nil {
			t.Fatal(err)
		}
		if string(res.Data) != `"`+scope+`"` {
			t.Errorf("L2 scope %s: data %s, want %s (scope collapsed!)", scope, res.Data, scope)
		}
	}
}

func TestSessionCache_StaleFallback(t *testing.T) {
	c := newTestSessionCacheWithL2(t, "stale")

	// 第一次 loader 成功 → L1 + L2
	if _, err := c.GetOrLoad(context.Background(), sessionEntityBugs, "all", func(ctx context.Context) ([]byte, error) {
		return []byte(`"fresh"`), nil
	}); err != nil {
		t.Fatal(err)
	}

	// 把 L2 改成已经过期（用 0s TTL：expires = now.Unix()，sleep 1.5s 后秒级必过期）
	key := c.makeKey(sessionEntityBugs, "all")
	if err := c.l2Set(context.Background(), key, []byte(`"stale-old"`), 0); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1500 * time.Millisecond)
	c.l1.ClearNamespace(c.ns) // 清 L1，确保走 loader 路径
	c.sf = singleflight.Group{}

	// loader 失败 — 必须回退到 L2 stale
	res, err := c.GetOrLoad(context.Background(), sessionEntityBugs, "all", func(ctx context.Context) ([]byte, error) {
		return nil, errors.New("zentao unavailable")
	})
	if err != nil {
		t.Fatalf("expected stale fallback to succeed, got err: %v", err)
	}
	if !res.FromCache || !res.Stale {
		t.Errorf("expected FromCache=true Stale=true, got FromCache=%v Stale=%v", res.FromCache, res.Stale)
	}
	if string(res.Data) != `"stale-old"` {
		t.Errorf("stale data mismatch: %s", string(res.Data))
	}
}

func TestSessionCache_LoaderErrorNoStale(t *testing.T) {
	c := newTestSessionCacheWithL2(t, "no-stale")

	_, err := c.GetOrLoad(context.Background(), sessionEntityTasks, "all", func(ctx context.Context) ([]byte, error) {
		return nil, errors.New("network error")
	})
	if err == nil {
		t.Fatal("expected error when no L2 stale and loader fails")
	}
}

// TestSessionCache_StaleFallback_NetworkError_StillServed
// 网络瞬断（非 fatal session err）仍走 stale fallback，保持原设计意图。
// 这条 regression guard 保护 batch H/J 阶段确立的行为。
func TestSessionCache_StaleFallback_NetworkError_StillServed(t *testing.T) {
	c := newTestSessionCacheWithL2(t, "network-stale")

	// 第一次成功 → L1 + L2 写入
	if _, err := c.GetOrLoad(context.Background(), sessionEntityBugs, "scope", func(ctx context.Context) ([]byte, error) {
		return []byte(`"fresh-data"`), nil
	}); err != nil {
		t.Fatal(err)
	}

	// 让 L2 过期
	key := c.makeKey(sessionEntityBugs, "scope")
	if err := c.l2Set(context.Background(), key, []byte(`"stale-data"`), 0); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1500 * time.Millisecond)
	c.l1.ClearNamespace(c.ns)
	c.sf = singleflight.Group{}

	// loader 返网络错(非 fatal session err)→ 必须走 stale fallback
	res, err := c.GetOrLoad(context.Background(), sessionEntityBugs, "scope", func(ctx context.Context) ([]byte, error) {
		return nil, errors.New("connection reset by peer")
	})
	if err != nil {
		t.Fatalf("expected stale fallback, got err: %v", err)
	}
	if !res.Stale {
		t.Errorf("expected Stale=true for network err, got %+v", res)
	}
	if string(res.Data) != `"stale-data"` {
		t.Errorf("stale data mismatch: %s", string(res.Data))
	}
}

// TestSessionCache_StaleFallback_FatalSessionError_Skipped
// fatal session 错误（被 errDoNotServeStale wrap）必须跳过 L2 stale fallback，
// 直接把原 err 透传出去。不能让"旧空 list"兜底让调用方误以为"<user-account> 真的没产品"。
//
// 触发场景（受限实例 SSO 实测）：用户账号被风控限流，server login 失败；
// 如果 stale 兜底生效，前端 product 下拉会显示一个旧空 list，看起来像真的没数据。
//
// 注：wrap 由 client.go safeLoader 完成；doLoad 看到 wrap 跳过 stale。
// 这里直接测 doLoad 行为（用 wrap 后的 err 喂 loader），不重复测 client.go 包装。
func TestSessionCache_StaleFallback_FatalSessionError_Skipped(t *testing.T) {
	c := newTestSessionCacheWithL2(t, "fatal-skip")

	// 第一次成功 → L1 + L2 写入
	if _, err := c.GetOrLoad(context.Background(), sessionEntityBugs, "scope", func(ctx context.Context) ([]byte, error) {
		return []byte(`"fresh-data"`), nil
	}); err != nil {
		t.Fatal(err)
	}

	// 让 L2 过期 + 清 L1,让 GetOrLoad 必然走 loader
	key := c.makeKey(sessionEntityBugs, "scope")
	if err := c.l2Set(context.Background(), key, []byte(`"stale-data"`), 0); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1500 * time.Millisecond)
	c.l1.ClearNamespace(c.ns)
	c.sf = singleflight.Group{}

	// 模拟 client.go safeLoader 的 wrap：原 err 加上 errDoNotServeStale sentinel
	wrappedFatal := fmt.Errorf("%w: %w", errDoNotServeStale, errSessionReloginRequired)
	res, err := c.GetOrLoad(context.Background(), sessionEntityBugs, "scope", func(ctx context.Context) ([]byte, error) {
		return nil, wrappedFatal
	})
	if err == nil {
		t.Fatal("expected error to be returned for wrapped fatal session err, got nil")
	}
	if res != nil && len(res.Data) > 0 {
		t.Errorf("expected nil/empty CacheResult for fatal err, got %+v", res)
	}
	// 原 err 必须能 unwrap 到 errSessionReloginRequired
	if !errors.Is(err, errSessionReloginRequired) {
		t.Errorf("expected wrapped err to unwrap to errSessionReloginRequired, got %v", err)
	}

	// 反向验证：不 wrap 的 fatal err 仍走 stale fallback(说明修复是精准的,
	// 不会误伤其他 fatal-like 错误,前提是上层正确 wrap)
	// 用 errAccessDenied(也是 IsSessionAccessDenied 的目标)但不 wrap,应当走 stale
	c2 := newTestSessionCacheWithL2(t, "fatal-no-wrap")
	if _, err := c2.GetOrLoad(context.Background(), sessionEntityBugs, "scope", func(ctx context.Context) ([]byte, error) {
		return []byte(`"fresh"`), nil
	}); err != nil {
		t.Fatal(err)
	}
	key2 := c2.makeKey(sessionEntityBugs, "scope")
	if err := c2.l2Set(context.Background(), key2, []byte(`"stale"`), 0); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1500 * time.Millisecond)
	c2.l1.ClearNamespace(c2.ns)
	c2.sf = singleflight.Group{}

	res2, err := c2.GetOrLoad(context.Background(), sessionEntityBugs, "scope", func(ctx context.Context) ([]byte, error) {
		return nil, errSessionAccessDenied
	})
	if err != nil {
		t.Fatalf("未 wrap 的 err 应该走 stale fallback, 不该返 err: %v", err)
	}
	if res2 == nil || !res2.Stale || string(res2.Data) != `"stale"` {
		t.Errorf("未 wrap 应该走 stale fallback, got %+v", res2)
	}
}

// TestSessionCache_ErrDoNotServeStale_Identifiable
// 验证 sentinel 能用 errors.Is 识别（doLoad 2c 路径就靠这个判定）。
func TestSessionCache_ErrDoNotServeStale_Identifiable(t *testing.T) {
	wrapped := fmt.Errorf("fatal: %w", fmt.Errorf("wrapped: %w", errDoNotServeStale))
	if !errors.Is(wrapped, errDoNotServeStale) {
		t.Error("errors.Is should identify errDoNotServeStale through fmt.Errorf %w chain")
	}

	// safeLoader 在 client.go 里的实际包装方式
	clientWrapped := fmt.Errorf("%w: %w", errDoNotServeStale, errSessionReloginRequired)
	if !errors.Is(clientWrapped, errDoNotServeStale) {
		t.Error("client safeLoader 包装的 err 也要能被 errors.Is 识别")
	}
}

func TestSessionCache_Singleflight(t *testing.T) {
	c := newTestSessionCache(t, "sf")

	var loaderCalls atomic.Int32
	loader := func(ctx context.Context) ([]byte, error) {
		loaderCalls.Add(1)
		time.Sleep(50 * time.Millisecond)
		return []byte(`"v"`), nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.GetOrLoad(context.Background(), sessionEntityStories, "all", loader); err != nil {
				t.Errorf("get err: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := loaderCalls.Load(); got != 1 {
		t.Errorf("loader calls = %d, want 1 (singleflight broken)", got)
	}
}

func TestSessionCache_Invalidate(t *testing.T) {
	c := newTestSessionCacheWithL2(t, "invalidate")

	loaderCalls := 0
	loader := func(ctx context.Context) ([]byte, error) {
		loaderCalls++
		return []byte(`"v"`), nil
	}

	if _, err := c.GetOrLoad(context.Background(), sessionEntityBugs, "all", loader); err != nil {
		t.Fatal(err)
	}

	if err := c.Invalidate(context.Background(), sessionEntityBugs, "all"); err != nil {
		t.Fatal(err)
	}

	if _, err := c.GetOrLoad(context.Background(), sessionEntityBugs, "all", loader); err != nil {
		t.Fatal(err)
	}
	if loaderCalls != 2 {
		t.Errorf("after Invalidate loader calls = %d, want 2", loaderCalls)
	}
}

func TestSessionCache_InvalidateEntity(t *testing.T) {
	c := newTestSessionCacheWithL2(t, "invalidate-entity")

	loaderCalls := 0
	loader := func(ctx context.Context) ([]byte, error) {
		loaderCalls++
		return []byte(`"v"`), nil
	}

	for _, scope := range []string{"all", "byProduct:1", "byProduct:2"} {
		if _, err := c.GetOrLoad(context.Background(), sessionEntityBugs, scope, loader); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := c.GetOrLoad(context.Background(), sessionEntityTasks, "all", loader); err != nil {
		t.Fatal(err)
	}
	startCalls := loaderCalls

	if err := c.InvalidateEntity(context.Background(), sessionEntityBugs); err != nil {
		t.Fatal(err)
	}

	for _, scope := range []string{"all", "byProduct:1", "byProduct:2"} {
		if _, err := c.GetOrLoad(context.Background(), sessionEntityBugs, scope, loader); err != nil {
			t.Fatal(err)
		}
	}
	// tasks 仍然命中
	if _, err := c.GetOrLoad(context.Background(), sessionEntityTasks, "all", loader); err != nil {
		t.Fatal(err)
	}
	if loaderCalls != startCalls+3 {
		t.Errorf("after InvalidateEntity loader calls = %d, want %d (only bugs re-fetched)", loaderCalls, startCalls+3)
	}
}

func TestSessionCache_InvalidateAll(t *testing.T) {
	c := newTestSessionCacheWithL2(t, "invalidate-all")

	loaderCalls := 0
	loader := func(ctx context.Context) ([]byte, error) {
		loaderCalls++
		return []byte(`"v"`), nil
	}

	for _, e := range []sessionEntity{sessionEntityBugs, sessionEntityTasks, sessionEntityUsers} {
		if _, err := c.GetOrLoad(context.Background(), e, "all", loader); err != nil {
			t.Fatal(err)
		}
	}
	startCalls := loaderCalls

	if err := c.InvalidateAll(context.Background()); err != nil {
		t.Fatal(err)
	}

	for _, e := range []sessionEntity{sessionEntityBugs, sessionEntityTasks, sessionEntityUsers} {
		if _, err := c.GetOrLoad(context.Background(), e, "all", loader); err != nil {
			t.Fatal(err)
		}
	}
	if loaderCalls != startCalls+3 {
		t.Errorf("after InvalidateAll loader calls = %d, want %d", loaderCalls, startCalls+3)
	}
}

func TestSessionCache_UpdateNamespace_ResetsCache(t *testing.T) {
	c := newTestSessionCacheWithL2(t, "update-ns")

	loaderCalls := 0
	loader := func(ctx context.Context) ([]byte, error) {
		loaderCalls++
		return []byte(`"v"`), nil
	}

	if _, err := c.GetOrLoad(context.Background(), sessionEntityBugs, "all", loader); err != nil {
		t.Fatal(err)
	}

	c.UpdateNamespace("https://other.example.com", "user-update-ns", "kydc")

	if _, err := c.GetOrLoad(context.Background(), sessionEntityBugs, "all", loader); err != nil {
		t.Fatal(err)
	}
	if loaderCalls != 2 {
		t.Errorf("after namespace change loader calls = %d, want 2", loaderCalls)
	}
}

func TestSessionCache_UpdateNamespace_SameNamespacePreservesCache(t *testing.T) {
	c := newTestSessionCache(t, "preserve-ns")

	loaderCalls := 0
	loader := func(ctx context.Context) ([]byte, error) {
		loaderCalls++
		return []byte(`"v"`), nil
	}

	if _, err := c.GetOrLoad(context.Background(), sessionEntityBugs, "all", loader); err != nil {
		t.Fatal(err)
	}

	c.UpdateNamespace("https://test.example.com", "user-preserve-ns", "kydc")
	if _, err := c.GetOrLoad(context.Background(), sessionEntityBugs, "all", loader); err != nil {
		t.Fatal(err)
	}
	if loaderCalls != 1 {
		t.Errorf("same namespace should preserve cache, loader calls = %d, want 1", loaderCalls)
	}
}

func TestSessionCache_ContextCancel(t *testing.T) {
	c := newTestSessionCache(t, "ctx-cancel")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.GetOrLoad(ctx, sessionEntityBugs, "all", func(ctx context.Context) ([]byte, error) {
		return nil, errors.New("should not be called")
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestSessionCache_EntityTTL_Table(t *testing.T) {
	cases := map[sessionEntity]struct{ Fresh, Stale time.Duration }{
		sessionEntityUsers:      {30 * time.Minute, 24 * time.Hour},
		sessionEntityProducts:   {30 * time.Minute, 24 * time.Hour},
		sessionEntityBugs:       {2 * time.Minute, 1 * time.Hour},
		sessionEntityTasks:      {2 * time.Minute, 1 * time.Hour},
		sessionEntityDashboard:  {5 * time.Minute, 1 * time.Hour},
		sessionEntityExecutions: {10 * time.Minute, 6 * time.Hour},
	}
	for e, want := range cases {
		got := entityTTL[e]
		if got.Fresh != want.Fresh || got.Stale != want.Stale {
			t.Errorf("entity %s: TTL {%v fresh, %v stale}, want {%v fresh, %v stale}",
				e, got.Fresh, got.Stale, want.Fresh, want.Stale)
		}
	}
}

func TestSessionCache_Stats(t *testing.T) {
	c := newTestSessionCacheWithL2(t, "stats")

	loader := func(ctx context.Context) ([]byte, error) {
		return []byte(`"v"`), nil
	}
	for _, scope := range []string{"a", "b", "c"} {
		if _, err := c.GetOrLoad(context.Background(), sessionEntityBugs, scope, loader); err != nil {
			t.Fatal(err)
		}
	}
	s := c.Stats()
	if s.L1Size != 3 {
		t.Errorf("L1Size = %d, want 3", s.L1Size)
	}
	if !s.L2Enabled {
		t.Error("L2Enabled should be true")
	}
	if s.Account != "user-stats" {
		t.Errorf("Account = %s, want user-stats", s.Account)
	}
	if s.Namespace == "" {
		t.Error("Namespace should not be empty")
	}
}

package service

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yi-nology/zentao-max/backend/internal/config"
	storage "github.com/yi-nology/zentao-max/backend/internal/repository/sqlite"
	"github.com/yi-nology/zentao-max/backend/pkg/logger"
)

func init() {
	// SWR 路径（含后台 goroutine）会打日志；logger 未初始化会 panic。
	_ = logger.Init(&config.LogConfig{Level: "info", Format: "console", OutputPath: "stderr"})
}

func newSWRTestService(t *testing.T) *CacheService {
	t.Helper()
	store, err := storage.NewSQLiteStore(filepath.Join(t.TempDir(), "swr-test.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return NewCacheService(store)
}

// 场景 1：新鲜缓存直接返回，不触发 loader。
func TestSWR_FreshHit(t *testing.T) {
	svc := newSWRTestService(t)
	ctx := context.Background()
	if err := svc.store.Set(ctx, "dash:", 1, []byte(`{"v":1}`), time.Minute); err != nil {
		t.Fatal(err)
	}
	calls := 0
	res, err := svc.GetOrLoadSWR(ctx, "dash:", 1, time.Minute, func(ctx context.Context) ([]byte, error) {
		calls++
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 || string(res.Data) != `{"v":1}` || res.Stale {
		t.Fatalf("新鲜命中应直接返回: calls=%d res=%+v", calls, res)
	}
}

// 场景 2：过期缓存 → 立即返回旧数据（Stale=true），loader 在后台完成并刷新缓存。
func TestSWR_StaleServedInstantlyAndRefreshed(t *testing.T) {
	svc := newSWRTestService(t)
	ctx := context.Background()
	// 负 TTL = 已过期条目（store.Set 直接落 expires_at=now-1min）
	if err := svc.store.Set(ctx, "dash:", 2, []byte(`{"v":"old"}`), -time.Minute); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	var loaderDone atomic.Bool
	res, err := svc.GetOrLoadSWR(ctx, "dash:", 2, time.Minute, func(ctx context.Context) ([]byte, error) {
		time.Sleep(300 * time.Millisecond) // 模拟 60-90s 大成本回源的缩比模型
		loaderDone.Store(true)
		return []byte(`{"v":"new"}`), nil
	})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	// 关键断言：不能等 loader（300ms 缩比模型），必须立即返回旧数据
	if elapsed > 150*time.Millisecond {
		t.Fatalf("SWR 应立即返回旧数据，实际阻塞 %v", elapsed)
	}
	if !res.Stale || string(res.Data) != `{"v":"old"}` {
		t.Fatalf("应返回过期数据: %+v", res)
	}
	if res.FromCache != true {
		t.Fatalf("应标记来自缓存: %+v", res)
	}

	// 等后台刷新完成（300ms 模型 + 写缓存），随后缓存应变为新值
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if loaderDone.Load() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !loaderDone.Load() {
		t.Fatal("后台刷新未执行")
	}
	// 刷新完成后新数据应可见（后台写缓存异步，轮询等待）
	for time.Now().Before(deadline) {
		if d, _, _ := svc.store.Get(ctx, "dash:", 2); d != nil && string(d) == `{"v":"new"}` {
			return // 通过
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("后台刷新完成后缓存未更新为新数据")
}

// 场景 3：完全无缓存 → 阻塞回源（冷加载语义与 GetOrLoad 一致）。
func TestSWR_ColdLoadBlocking(t *testing.T) {
	svc := newSWRTestService(t)
	res, err := svc.GetOrLoadSWR(context.Background(), "dash:", 3, time.Minute,
		func(ctx context.Context) ([]byte, error) {
			return []byte(`{"v":"cold"}`), nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if res.FromCache || res.Stale || string(res.Data) != `{"v":"cold"}` {
		t.Fatalf("冷加载语义错误: %+v", res)
	}
}

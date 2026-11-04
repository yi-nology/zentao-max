package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oekit/concurrency/fanout"
)

// 场景：实例级列表空结果防毒化——isEmpty 命中时返回错误（不缓存空结果），
// 已有过期缓存时 SWR 回退旧数据（stale=true）且 loader 真跑过一次。
func TestSwrLoadJSON_RejectEmpty(t *testing.T) {
	svc := newSWRTestService(t)
	ctx := context.Background()
	// 预置过期缓存（旧数据 3 条）
	if err := svc.store.Set(ctx, "tasks:all", 0, []byte(`[{"id":1},{"id":2},{"id":3}]`), -time.Minute); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int64
	got, stale, err := swrLoadJSON[[]struct{ ID int }](svc, "tasks:all", 0, time.Minute,
		func(ctx context.Context) ([]struct{ ID int }, error) {
			calls.Add(1)
			return []struct{ ID int }{}, nil // 上游异常返回空
		},
		func(t []struct{ ID int }) bool { return len(t) == 0 },
		"任务全量遍历为空（疑似会话失效），不缓存空结果",
	)
	if err != nil {
		t.Fatalf("应回退旧缓存而非报错: %v", err)
	}
	if !stale || len(got) != 3 {
		t.Fatalf("应秒回旧缓存(stale=true 3条): stale=%v len=%d", stale, len(got))
	}
	// 后台刷新真跑过（SWR stale 语义：loader 异步执行）
	deadline := time.Now().Add(2 * time.Second)
	for calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if calls.Load() != 1 {
		t.Fatalf("后台刷新应执行一次 loader, calls=%d", calls.Load())
	}
	// 防毒化关键断言：空结果不得进缓存——过期旧数据经 GetStale 仍可取
	//（store.Get 对过期条目返回 nil，%v 打印 nil 切片形如 "[]"，勿据此误判）
	raw, _, err := svc.store.GetStale(ctx, "tasks:all", 0)
	if err != nil || raw == nil {
		t.Fatalf("过期旧数据应经 GetStale 保留: err=%v raw=%v", err, raw)
	}
	if string(raw) != `[{"id":1},{"id":2},{"id":3}]` {
		t.Fatalf("空结果被写入缓存(毒化)！缓存内容: %s", raw)
	}
}

// 场景：nil cache 直连（无缓存直通）。
func TestSwrLoadJSON_NilCacheDirect(t *testing.T) {
	got, stale, err := swrLoadJSON[[]int](nil, "x:all", 0, time.Minute,
		func(ctx context.Context) ([]int, error) { return []int{1, 2}, nil },
		nil, "")
	if err != nil || stale || len(got) != 2 {
		t.Fatalf("nil cache 应直连: got=%v stale=%v err=%v", got, stale, err)
	}
}

// 场景：fanout.Map（crawlAcrossExecutions 的底层原语）预算确定性收口——
// 1440 项 × 100ms/项 × 并发 8，预算 500ms：ctx 到点掐断排队与在途项，
// elapsed ≤ 预算 + 1s 余量；未跑的项以 context.Canceled 聚合进错误。
// （此前的手搓 sem/wg/select 曾因随机选择把 60s 预算拖成 137s。）
func TestCrawlBudget_FanoutEnforcement(t *testing.T) {
	deadline := time.Now().Add(500 * time.Millisecond)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	ids := make([]int, 1440)
	var count atomic.Int64
	start := time.Now()
	_, err := fanout.Map(ctx, ids, 8, func(ctx context.Context, _ int) ([]int, error) {
		time.Sleep(100 * time.Millisecond)
		count.Add(1)
		return []int{1}, nil
	})
	elapsed := time.Since(start)
	if elapsed > 1500*time.Millisecond {
		t.Fatalf("预算未确定性收口: elapsed=%v", elapsed)
	}
	got := count.Load()
	if got < 20 || got > 60 {
		t.Fatalf("预算内完成量应约 40（8并发×500ms/100ms）, got %d", got)
	}
	if err == nil {
		t.Fatalf("预算截断应产生部分失败聚合错误")
	}
	if !isCanceledAggregate(err) {
		t.Fatalf("聚合错误应可辨识 ctx 取消: %v", err)
	}
}

// isCanceledAggregate 判断 fanout 聚合错误是否含 ctx 取消/截止项。
func isCanceledAggregate(err error) bool {
	var fe *fanout.Error
	if !errors.As(err, &fe) {
		return false
	}
	for _, f := range fe.Failed {
		if errors.Is(f.Err, context.Canceled) || errors.Is(f.Err, context.DeadlineExceeded) {
			return true
		}
	}
	return false
}

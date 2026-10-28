package zentao

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// RatePerSec=0 是不限流的约定（限流相关测试用具体非零值）。
const noRate = 0

func newTestPool(t *testing.T, cfg SessionWorkerPoolConfig) *SessionWorkerPool {
	t.Helper()
	cfg.RatePerSec = noRate
	p := NewSessionWorkerPool(cfg)
	p.Start()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = p.Shutdown(ctx)
	})
	return p
}

func TestSessionPool_BasicSubmit(t *testing.T) {
	p := newTestPool(t, SessionWorkerPoolConfig{})
	called := 0
	fn := func(ctx context.Context) error {
		called++
		return nil
	}
	if err := p.SubmitRead(context.Background(), "test", fn, nil); err != nil {
		t.Fatalf("submit err: %v", err)
	}
	if called != 1 {
		t.Errorf("called = %d", called)
	}
}

func TestSessionPool_FnErrorPropagated(t *testing.T) {
	p := newTestPool(t, SessionWorkerPoolConfig{})
	wantErr := errors.New("fn failed")
	err := p.SubmitRead(context.Background(), "test",
		func(ctx context.Context) error { return wantErr },
		nil)
	if !errors.Is(err, wantErr) {
		t.Errorf("got %v, want %v", err, wantErr)
	}
}

func TestSessionPool_WriteSerialized(t *testing.T) {
	p := newTestPool(t, SessionWorkerPoolConfig{ReadConcurrency: 4})
	var (
		mu       sync.Mutex
		sequence []int
	)
	for i := 0; i < 5; i++ {
		i := i
		err := p.SubmitWrite(context.Background(), "write",
			func(ctx context.Context) error {
				mu.Lock()
				sequence = append(sequence, i)
				mu.Unlock()
				time.Sleep(20 * time.Millisecond)
				return nil
			})
		if err != nil {
			t.Fatalf("write submit err: %v", err)
		}
	}
	time.Sleep(300 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if len(sequence) != 5 {
		t.Errorf("sequence len = %d, want 5", len(sequence))
	}
	seen := map[int]bool{}
	for _, id := range sequence {
		if seen[id] {
			t.Errorf("duplicate id %d", id)
		}
		seen[id] = true
	}
}

func TestSessionPool_ReadConcurrent(t *testing.T) {
	// 关掉限流（rate=noRate）才能验证纯并发能力，限流是另测。
	p := newTestPool(t, SessionWorkerPoolConfig{ReadConcurrency: 4, JobBuf: 8, RatePerSec: noRate})
	var (
		active    atomic.Int32
		maxActive atomic.Int32
		wg        sync.WaitGroup
	)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := p.SubmitRead(context.Background(), "read",
				func(ctx context.Context) error {
					cur := active.Add(1)
					defer active.Add(-1)
					for {
						prev := maxActive.Load()
						if cur <= prev || maxActive.CompareAndSwap(prev, cur) {
							break
						}
					}
					time.Sleep(80 * time.Millisecond)
					return nil
				}, nil)
			if err != nil {
				t.Errorf("err: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := maxActive.Load(); got < 2 {
		t.Errorf("maxActive = %d, want >= 2", got)
	}
}

func TestSessionPool_CacheFallback(t *testing.T) {
	// JobBuf=2, ReadConcurrency=1。3 个 filler 并发 submit 让队列和 worker 短时间都满,
	// 第 4 个 submit 排队超时 → cacheFallback。
	p := NewSessionWorkerPool(SessionWorkerPoolConfig{
		JobBuf:          2,
		ReadConcurrency: 1,
	})
	p.Start()
	defer p.Shutdown(context.Background())

	slow := func(ctx context.Context) error { time.Sleep(500 * time.Millisecond); return nil }
	cacheCalled := 0
	cacheFn := func(ctx context.Context) error {
		cacheCalled++
		return nil
	}

	fillerDone := make(chan struct{}, 3)
	for i := 0; i < 3; i++ {
		go func() {
			if err := p.SubmitRead(context.Background(), "filler", slow, nil); err != nil {
				t.Errorf("filler err: %v", err)
			}
			fillerDone <- struct{}{}
		}()
	}
	time.Sleep(30 * time.Millisecond)

	if err := p.SubmitRead(context.Background(), "fallback",
		func(ctx context.Context) error { return nil },
		cacheFn); err != nil {
		t.Errorf("expected fallback success, got: %v", err)
	}
	if cacheCalled != 1 {
		t.Errorf("cacheFallback called %d times, want 1", cacheCalled)
	}
	for i := 0; i < 3; i++ {
		<-fillerDone
	}
}

func TestSessionPool_CacheFallbackAlsoFails_ReturnsBusy(t *testing.T) {
	p := NewSessionWorkerPool(SessionWorkerPoolConfig{
		JobBuf:          2,
		ReadConcurrency: 1,
	})
	p.Start()
	defer p.Shutdown(context.Background())

	slow := func(ctx context.Context) error { time.Sleep(500 * time.Millisecond); return nil }
	fillerDone := make(chan struct{}, 3)
	for i := 0; i < 3; i++ {
		go func() {
			if err := p.SubmitRead(context.Background(), "filler", slow, nil); err != nil {
				t.Errorf("filler err: %v", err)
			}
			fillerDone <- struct{}{}
		}()
	}
	time.Sleep(30 * time.Millisecond)

	err := p.SubmitRead(context.Background(), "fallback-fails",
		func(ctx context.Context) error { return nil },
		func(ctx context.Context) error { return errors.New("cache also down") })
	if !errors.Is(err, ErrPoolBusy) {
		t.Errorf("expected ErrPoolBusy, got: %v", err)
	}
	for i := 0; i < 3; i++ {
		<-fillerDone
	}
}

func TestSessionPool_PrefetchNonBlocking(t *testing.T) {
	p := NewSessionWorkerPool(SessionWorkerPoolConfig{JobBuf: 1, ReadConcurrency: 1})
	p.Start()
	defer p.Shutdown(context.Background())

	slow := func(ctx context.Context) error { time.Sleep(500 * time.Millisecond); return nil }
	fillerDone := make(chan struct{})
	go func() {
		if err := p.SubmitRead(context.Background(), "blocker", slow, nil); err != nil {
			t.Errorf("err: %v", err)
		}
		close(fillerDone)
	}()
	time.Sleep(20 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		p.SubmitPrefetch(context.Background(), "prefetch",
			func(ctx context.Context) error { return nil })
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("prefetch should be non-blocking")
	}
	<-fillerDone
}

func TestSessionPool_ContextCancel(t *testing.T) {
	p := newTestPool(t, SessionWorkerPoolConfig{JobBuf: 1})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := p.SubmitRead(ctx, "test",
		func(ctx context.Context) error { return nil },
		nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestSessionPool_ShutdownDrains(t *testing.T) {
	p := NewSessionWorkerPool(SessionWorkerPoolConfig{ReadConcurrency: 2})
	p.Start()

	var done atomic.Int32
	for i := 0; i < 3; i++ {
		err := p.SubmitRead(context.Background(), "test",
			func(ctx context.Context) error {
				time.Sleep(80 * time.Millisecond)
				done.Add(1)
				return nil
			}, nil)
		if err != nil {
			t.Fatalf("submit err: %v", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := p.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown err: %v", err)
	}
	if done.Load() != 3 {
		t.Errorf("done = %d, want 3", done.Load())
	}
}

func TestSessionPool_ShutdownRejectsNew(t *testing.T) {
	p := NewSessionWorkerPool(SessionWorkerPoolConfig{})
	p.Start()
	_ = p.Shutdown(context.Background())

	cacheCalled := 0
	cacheFn := func(ctx context.Context) error {
		cacheCalled++
		return nil
	}
	err := p.SubmitRead(context.Background(), "after-shutdown",
		func(ctx context.Context) error { return nil },
		cacheFn)
	if cacheCalled != 1 {
		t.Errorf("cacheFallback called %d times, want 1", cacheCalled)
	}
	if err != nil {
		t.Errorf("expected fallback success, got: %v", err)
	}
	err = p.SubmitRead(context.Background(), "after-shutdown-no-fallback",
		func(ctx context.Context) error { return nil }, nil)
	if !errors.Is(err, ErrPoolClosed) {
		t.Errorf("expected ErrPoolClosed, got: %v", err)
	}
}

func TestSessionPool_RateLimit(t *testing.T) {
	p := NewSessionWorkerPool(SessionWorkerPoolConfig{RatePerSec: 10})
	p.Start()
	defer p.Shutdown(context.Background())

	start := time.Now()
	for i := 0; i < 5; i++ {
		if err := p.SubmitRead(context.Background(), "rl",
			func(ctx context.Context) error { return nil },
			nil); err != nil {
			t.Fatalf("submit err: %v", err)
		}
	}
	elapsed := time.Since(start)
	// 5 jobs 在 rate=10/s 下，理论最小耗时 ≈ 4 * 100ms = 400ms
	if elapsed < 350*time.Millisecond {
		t.Errorf("rate limit not working: 5 jobs in %v, want >= 350ms", elapsed)
	}
}

func TestSessionPool_Stats(t *testing.T) {
	p := NewSessionWorkerPool(SessionWorkerPoolConfig{RatePerSec: 8})
	p.Start()
	defer p.Shutdown(context.Background())

	stats := p.Stats()
	if stats.RatePerSec != 8 {
		t.Errorf("RatePerSec = %v, want 8", stats.RatePerSec)
	}
	if stats.Closed {
		t.Error("Closed should be false")
	}
}

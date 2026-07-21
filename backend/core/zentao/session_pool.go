package zentao

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// SessionWorkerPool 是 Session 模式专属的请求调度中心。
//
// 设计目标：
//   - 限流：避免前端一次打开 dashboard 触发 5+ goroutine 同时打禅道触发受限实例 SSO 反爬。
//   - 写串行：写端点可能让服务端尚在 commit，并发读易读到旧值。
//   - 读并发 4：单 cookie 下受限实例 4 并发实测不会触发限流。
//   - 降级：pool 满 / 排队超时 → 自动走 cache fallback；彻底无可用 → ErrPoolBusy。
//   - 优雅停机：Shutdown 等待 in-flight 完成 / ctx 取消。
type SessionWorkerPool struct {
	nextAllowedUnixNano atomic.Int64
	ratePerSec           float64

	readJobs  chan *job
	writeJobs chan *job

	readConcurrency int

	closed atomic.Bool
	wg     sync.WaitGroup

	readActive  atomic.Int64
	writeActive atomic.Int64
	queueDepth  atomic.Int64

	now   func() time.Time
	sleep func(time.Duration)
}

// SessionWorkerPoolConfig 构造配置。
//   - RatePerSec：0 = 无限流；>0 = 限流到该速率。受限实例建议 4。
//   - JobBuf：0 = 64。
//   - ReadConcurrency：0 = 4。
//   - QueueTimeout：0 = 100ms。
type SessionWorkerPoolConfig struct {
	RatePerSec      float64
	JobBuf          int
	ReadConcurrency int
	QueueTimeout    time.Duration
}

type job struct {
	ctx     context.Context
	op      string
	isWrite bool
	prio    int
	fn      func(ctx context.Context) error
	done    chan error
}

var ErrPoolBusy = errors.New("session pool busy, queue full")
var ErrPoolClosed = errors.New("session pool closed")

func NewSessionWorkerPool(cfg SessionWorkerPoolConfig) *SessionWorkerPool {
	if cfg.RatePerSec < 0 {
		cfg.RatePerSec = 0
	}
	if cfg.JobBuf <= 0 {
		cfg.JobBuf = 64
	}
	if cfg.ReadConcurrency <= 0 {
		cfg.ReadConcurrency = 4
	}
	if cfg.QueueTimeout <= 0 {
		cfg.QueueTimeout = 100 * time.Millisecond
	}
	return &SessionWorkerPool{
		ratePerSec:      cfg.RatePerSec,
		readConcurrency: cfg.ReadConcurrency,
		readJobs:        make(chan *job, cfg.JobBuf),
		writeJobs:       make(chan *job, cfg.JobBuf),
		now:             time.Now,
		sleep:           time.Sleep,
	}
}

func (p *SessionWorkerPool) Start() {
	if p.closed.Load() {
		return
	}
	for i := 0; i < p.readConcurrency; i++ {
		p.wg.Add(1)
		go p.readWorker()
	}
	p.wg.Add(1)
	go p.writeWorker()
}

func (p *SessionWorkerPool) SubmitRead(
	ctx context.Context,
	op string,
	fn func(ctx context.Context) error,
	cacheFallback func(ctx context.Context) error,
) error {
	return p.submit(ctx, op, false, 0, fn, cacheFallback)
}

func (p *SessionWorkerPool) SubmitWrite(ctx context.Context, op string, fn func(ctx context.Context) error) error {
	return p.submit(ctx, op, true, 0, fn, nil)
}

func (p *SessionWorkerPool) SubmitPrefetch(ctx context.Context, op string, fn func(ctx context.Context) error) {
	_ = p.submit(ctx, op, false, 1, fn, nil)
}

func (p *SessionWorkerPool) submit(
	ctx context.Context,
	op string,
	isWrite bool,
	prio int,
	fn func(ctx context.Context) error,
	cacheFallback func(ctx context.Context) error,
) error {
	if p.closed.Load() {
		if cacheFallback != nil {
			if err := cacheFallback(ctx); err == nil {
				return nil
			}
		}
		return ErrPoolClosed
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	j := &job{
		ctx:     ctx,
		op:      op,
		isWrite: isWrite,
		prio:    prio,
		fn:      fn,
		done:    make(chan error, 1),
	}

	target := p.readJobs
	if isWrite {
		target = p.writeJobs
	}

	if prio == 0 {
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case target <- j:
			p.queueDepth.Add(1)
			timer.Stop()
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			if cacheFallback != nil {
				if err := cacheFallback(ctx); err == nil {
					return nil
				}
			}
			return ErrPoolBusy
		}
		// 等待结果
		select {
		case err := <-j.done:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// prio=1 (prefetch) — 非阻塞入队，不等 done
	select {
	case target <- j:
		p.queueDepth.Add(1)
	default:
		// 队列满，丢弃（预期行为）
		return nil
	}
	return nil
}

func (p *SessionWorkerPool) readWorker() {
	defer p.wg.Done()
	for j := range p.readJobs {
		p.queueDepth.Add(-1)
		p.readActive.Add(1)
		p.runJob(j)
		p.readActive.Add(-1)
	}
}

func (p *SessionWorkerPool) writeWorker() {
	defer p.wg.Done()
	for j := range p.writeJobs {
		p.queueDepth.Add(-1)
		p.writeActive.Add(1)
		p.runJob(j)
		p.writeActive.Add(-1)
	}
}

func (p *SessionWorkerPool) acquireToken(ctx context.Context) {
	if p.ratePerSec <= 0 {
		return
	}
	interval := time.Duration(float64(time.Second) / p.ratePerSec)
	for {
		now := p.now().UnixNano()
		next := p.nextAllowedUnixNano.Load()
		if now >= next {
			if p.nextAllowedUnixNano.CompareAndSwap(next, now+interval.Nanoseconds()) {
				return
			}
			continue
		}
		wait := time.Duration(next - now)
		if wait > interval {
			p.nextAllowedUnixNano.Store(now + interval.Nanoseconds())
			continue
		}
		t := time.NewTimer(wait)
		select {
		case <-t.C:
		case <-ctx.Done():
			t.Stop()
			return
		}
	}
}

func (p *SessionWorkerPool) runJob(j *job) {
	defer func() {
		if r := recover(); r != nil {
			j.done <- errors.New("session pool job panicked")
		}
	}()
	p.acquireToken(j.ctx)
	err := j.fn(j.ctx)
	j.done <- err
}

func (p *SessionWorkerPool) Shutdown(ctx context.Context) error {
	if !p.closed.CompareAndSwap(false, true) {
		return nil
	}
	close(p.readJobs)
	close(p.writeJobs)
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type SessionWorkerPoolStats struct {
	RatePerSec  float64
	QueueDepth  int64
	ReadActive  int64
	WriteActive int64
	Closed      bool
}

func (p *SessionWorkerPool) Stats() SessionWorkerPoolStats {
	return SessionWorkerPoolStats{
		RatePerSec:  p.ratePerSec,
		QueueDepth:  p.queueDepth.Load(),
		ReadActive:  p.readActive.Load(),
		WriteActive: p.writeActive.Load(),
		Closed:      p.closed.Load(),
	}
}

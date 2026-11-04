package service

import (
	"context"
	"fmt"
	"time"

	"github.com/oekit/concurrency/async"
	"github.com/oekit/concurrency/singleflight"

	"github.com/yi-nology/zentao-max/backend/pkg/logger"
	"github.com/yi-nology/zentao-max/backend/internal/repository/sqlite"
)

// DefaultCacheTTL 默认缓存有效期（5 分钟）
const DefaultCacheTTL = 5 * time.Minute

// CacheService 离线缓存读写服务
// 提供带 fallback 的 GetOrLoad 模式：缓存命中直接返回，未命中或过期调用 loader 回源
type CacheService struct {
	store storage.Store
	// inflight 回源击穿保护：并发同 key 只放行一个 loader
	inflight *singleflight.Flight[[]byte]
}

// NewCacheService 创建缓存服务
func NewCacheService(store storage.Store) *CacheService {
	return &CacheService{
		store:    store,
		inflight: singleflight.New[[]byte](),
	}
}

// CacheResult 缓存查询结果
type CacheResult struct {
	Data      []byte
	FromCache bool      // true = 来自本地缓存（可能过期），false = 来自回源
	CachedAt  time.Time // 缓存时间（FromCache=true 时有意义）
	Stale     bool      // true = 来自过期缓存（离线 fallback 场景）
}

// GetOrLoad 带缓存 fallback 的查询
//   - 优先查未过期的缓存，命中直接返回（FromCache=true）
//   - 未命中调用 loader 回源；成功后写入缓存，返回（FromCache=false）
//   - loader 失败（如禅道不可达）但有过期缓存时，返回过期缓存（Stale=true）
func (s *CacheService) GetOrLoad(
	ctx context.Context,
	entityType string,
	productID int,
	ttl time.Duration,
	loader func(ctx context.Context) ([]byte, error),
) (*CacheResult, error) {
	if ttl <= 0 {
		ttl = DefaultCacheTTL
	}

	// 1. 查未过期缓存
	cachedData, cachedAt, err := s.store.Get(ctx, entityType, productID)
	if err != nil {
		logger.Warn("cache read failed, will fall through to loader",
			"entityType", entityType,
			"productID", productID,
			"error", err)
	}
	if cachedData != nil {
		return &CacheResult{
			Data:      cachedData,
			FromCache: true,
			CachedAt:  cachedAt,
		}, nil
	}
	return s.blockingLoad(ctx, entityType, productID, ttl, loader)
}

// GetOrLoadSWR 带 stale-while-revalidate 语义的查询（大成本回源场景专用，
// 如 dashboard 的逐执行任务遍历——产品 1029 实测冷加载 60-90s）：
//   - 新鲜缓存命中 → 直接返回（与 GetOrLoad 一致）；
//   - 缓存已过期但有过期数据 → 立即返回过期数据（Stale=true）+ 后台异步回源刷新
//     （singleflight 防重复打禅道）——用户零等待，刷新完成后下一次请求拿到新数据；
//   - 完全无数据 → 阻塞回源（首次冷加载仍需等待，无法避免）。
func (s *CacheService) GetOrLoadSWR(
	ctx context.Context,
	entityType string,
	productID int,
	ttl time.Duration,
	loader func(ctx context.Context) ([]byte, error),
) (*CacheResult, error) {
	if ttl <= 0 {
		ttl = DefaultCacheTTL
	}

	// 1. 新鲜命中
	cachedData, cachedAt, err := s.store.Get(ctx, entityType, productID)
	if err == nil && cachedData != nil {
		return &CacheResult{Data: cachedData, FromCache: true, CachedAt: cachedAt}, nil
	}

	// 2. 过期数据 → SWR：立即返回 + 后台刷新
	if staleData, staleAt, serr := s.store.GetStale(ctx, entityType, productID); serr == nil && staleData != nil {
		logger.Info("SWR: serving stale while refreshing in background",
			"entityType", entityType,
			"productID", productID,
			"cachedAt", staleAt)
		s.triggerBackgroundRefresh(entityType, productID, ttl, loader)
		return &CacheResult{Data: staleData, FromCache: true, CachedAt: staleAt, Stale: true}, nil
	}

	// 3. 冷加载（阻塞，与 GetOrLoad 同路径）
	return s.blockingLoad(ctx, entityType, productID, ttl, loader)
}

// blockingLoad 冷加载：singleflight 回源 → 写缓存 → 失败时 stale 兜底。
func (s *CacheService) blockingLoad(
	ctx context.Context,
	entityType string,
	productID int,
	ttl time.Duration,
	loader func(ctx context.Context) ([]byte, error),
) (*CacheResult, error) {
	// 回源（singleflight 击穿保护：并发同 entityType+productID 的请求
	// 只有一个真正调 loader 打禅道，其余共享结果）
	key := fmt.Sprintf("%s:%d", entityType, productID)
	freshData, loadErr := s.inflight.Do(ctx, key, func(ctx context.Context) ([]byte, error) {
		return loader(ctx)
	})
	if loadErr == nil {
		// 回源成功，写缓存（异步避免阻塞）。
		// 关键：ctx 用调用方传入的 + 5s 超时（不是 Background），store 已 Close 时
		// 错误能及时返回 + 5s 内必退出，避免 goroutine 永久 hang。
		// GoSafe：panic 收编 + 统一 panic 日志，替代原手写 recover 块
		async.GoSafe(func() {
			writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.store.Set(writeCtx, entityType, productID, freshData, ttl); err != nil {
				logger.Warn("cache write failed",
					"entityType", entityType,
					"productID", productID,
					"error", err)
			}
		})
		return &CacheResult{
			Data:      freshData,
			FromCache: false,
		}, nil
	}

	// 回源失败，尝试用过期缓存兜底
	staleData, staleAt, staleErr := s.store.GetStale(ctx, entityType, productID)
	if staleErr == nil && staleData != nil {
		logger.Warn("loader failed, serving stale cache as fallback",
			"entityType", entityType,
			"productID", productID,
			"cachedAt", staleAt,
			"error", loadErr)
		return &CacheResult{
			Data:      staleData,
			FromCache: true,
			CachedAt:  staleAt,
			Stale:     true,
		}, nil
	}

	// 完全无数据，返回 loader 的原始错误
	return nil, fmt.Errorf("loader 失败且无可用缓存: %w", loadErr)
}

// swrRefreshBudget SWR 后台刷新的独立预算。要大于 fetchDashboard 内部的 90s
// 总预算，保证刷新能跑到预算边界而不是被外层先掐断。
const swrRefreshBudget = 150 * time.Second

// triggerBackgroundRefresh 后台单飞刷新。ctx 独立于 HTTP 请求（Background+超时），
// 请求结束后刷新继续进行；in-flight 时重复触发共享同一次回源，不会加倍打禅道。
func (s *CacheService) triggerBackgroundRefresh(entityType string, productID int, ttl time.Duration, loader func(context.Context) ([]byte, error)) {
	key := fmt.Sprintf("%s:%d", entityType, productID)
	async.GoSafe(func() {
		ctx, cancel := context.WithTimeout(context.Background(), swrRefreshBudget)
		defer cancel()
		data, err := s.inflight.Do(ctx, key, func(ctx context.Context) ([]byte, error) {
			// 双检：排队等 in-flight 期间可能已被前一次刷新写新鲜
			if d, _, gerr := s.store.Get(ctx, entityType, productID); gerr == nil && d != nil {
				return d, nil
			}
			return loader(ctx)
		})
		if err != nil {
			logger.Warn("SWR background refresh failed (keeping stale data)",
				"entityType", entityType,
				"productID", productID,
				"error", err)
			return
		}
		writeCtx, wcancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer wcancel()
		if werr := s.store.Set(writeCtx, entityType, productID, data, ttl); werr != nil {
			logger.Warn("SWR background refresh cache write failed",
				"entityType", entityType,
				"productID", productID,
				"error", werr)
		} else {
			logger.Info("SWR background refresh completed",
				"entityType", entityType,
				"productID", productID)
		}
	})
}

// GetOrLoadBypass 显式刷新：跳过缓存读取直接回源（成功后写缓存供后续请求命中；
// 失败仍走 stale 兜底）。供"刷新按钮"类语义使用。
func (s *CacheService) GetOrLoadBypass(
	ctx context.Context,
	entityType string,
	productID int,
	ttl time.Duration,
	loader func(ctx context.Context) ([]byte, error),
) (*CacheResult, error) {
	if ttl <= 0 {
		ttl = DefaultCacheTTL
	}
	return s.blockingLoad(ctx, entityType, productID, ttl, loader)
}

// Invalidate 使指定缓存失效
func (s *CacheService) Invalidate(ctx context.Context, entityType string, productID int) error {
	s.inflight.Forget(fmt.Sprintf("%s:%d", entityType, productID))
	return s.store.Invalidate(ctx, entityType, productID)
}

// ClearAll 清空所有缓存
func (s *CacheService) ClearAll(ctx context.Context) error {
	// 清空全部 in-flight（key 集合不可枚举，直接换新 Flight 实例）
	s.inflight = singleflight.New[[]byte]()
	return s.store.ClearAll(ctx)
}

// Status 返回缓存状态
func (s *CacheService) Status(ctx context.Context) (*storage.CacheStatus, error) {
	return s.store.Status(ctx)
}

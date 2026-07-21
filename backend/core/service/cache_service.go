package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/yi-nology/zentao-max/backend/core/logger"
	"github.com/yi-nology/zentao-max/backend/core/storage"
	"go.uber.org/zap"
)

// DefaultCacheTTL 默认缓存有效期（5 分钟）
const DefaultCacheTTL = 5 * time.Minute

// CacheService 离线缓存读写服务
// 提供带 fallback 的 GetOrLoad 模式：缓存命中直接返回，未命中或过期调用 loader 回源
type CacheService struct {
	store storage.Store
}

// NewCacheService 创建缓存服务
func NewCacheService(store storage.Store) *CacheService {
	return &CacheService{store: store}
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
			zap.String("entityType", entityType),
			zap.Int("productID", productID),
			zap.Error(err))
	}
	if cachedData != nil {
		return &CacheResult{
			Data:      cachedData,
			FromCache: true,
			CachedAt:  cachedAt,
		}, nil
	}

	// 2. 回源
	freshData, loadErr := loader(ctx)
	if loadErr == nil {
		// 回源成功，写缓存（异步避免阻塞）
		go func() {
			if err := s.store.Set(context.Background(), entityType, productID, freshData, ttl); err != nil {
				logger.Warn("cache write failed",
					zap.String("entityType", entityType),
					zap.Int("productID", productID),
					zap.Error(err))
			}
		}()
		return &CacheResult{
			Data:      freshData,
			FromCache: false,
		}, nil
	}

	// 3. 回源失败，尝试用过期缓存兜底
	staleData, staleAt, staleErr := s.store.GetStale(ctx, entityType, productID)
	if staleErr == nil && staleData != nil {
		logger.Warn("loader failed, serving stale cache as fallback",
			zap.String("entityType", entityType),
			zap.Int("productID", productID),
			zap.Time("cachedAt", staleAt),
			zap.Error(loadErr))
		return &CacheResult{
			Data:      staleData,
			FromCache: true,
			CachedAt:  staleAt,
			Stale:     true,
		}, nil
	}

	// 4. 完全无数据，返回 loader 的原始错误
	return nil, fmt.Errorf("loader 失败且无可用缓存: %w", loadErr)
}

// Invalidate 使指定缓存失效
func (s *CacheService) Invalidate(ctx context.Context, entityType string, productID int) error {
	return s.store.Invalidate(ctx, entityType, productID)
}

// ClearAll 清空所有缓存
func (s *CacheService) ClearAll(ctx context.Context) error {
	return s.store.ClearAll(ctx)
}

// Status 返回缓存状态
func (s *CacheService) Status(ctx context.Context) (*storage.CacheStatus, error) {
	return s.store.Status(ctx)
}

// GetJSON 通用 JSON 反序列化缓存读取（语法糖）
func GetJSON[T any](ctx context.Context, s *CacheService, entityType string, productID int, ttl time.Duration, loader func(ctx context.Context) (*T, error)) (*T, bool, error) {
	result, err := s.GetOrLoad(ctx, entityType, productID, ttl, func(ctx context.Context) ([]byte, error) {
		v, err := loader(ctx)
		if err != nil {
			return nil, err
		}
		return json.Marshal(v)
	})
	if err != nil {
		return nil, false, err
	}
	var v T
	if err := json.Unmarshal(result.Data, &v); err != nil {
		return nil, false, err
	}
	return &v, result.FromCache, nil
}

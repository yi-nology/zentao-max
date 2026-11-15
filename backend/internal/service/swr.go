package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// swrLoadJSON 服务层 SWR JSON 缓存的统一入口（2026-11-04 破坏性重构：
// 此前 tasks/stories/bugs/executions/projects 六处各写一遍 loader 包装 +
// nil 判断 + 空结果防毒化，遗漏一处就是一类故障）。
//
// 语义：
//   - cache 为 nil → 直连 load（无缓存直通）；
//   - 新鲜命中 → 秒回；过期 → 秒回旧数据 + 后台单飞刷新（SWR）；
//   - 完全无缓存 → 阻塞回源（singleflight 击穿保护）；
//   - isEmpty 非 nil 且 load 结果为空 → 返回错误（不缓存空结果），
//     SWR 据此回退旧缓存——防「会话失效期空列表被当合法数据固化」
//     的毒化类故障（实例级列表专用；产品级空列表合法，传 nil）。
//
// 返回 (数据, 是否来自缓存/过期数据, 错误)。
func swrLoadJSON[T any](
	cache *CacheService,
	entityType string,
	id int,
	ttl time.Duration,
	load func(ctx context.Context) (T, error),
	isEmpty func(T) bool,
	emptyMsg string,
) (T, bool, error) {
	return swrLoadJSONGuarded(cache, entityType, id, ttl, load, isEmpty, emptyMsg, nil)
}

// swrLoadJSONGuarded 与 swrLoadJSON 相同，额外支持 noCache 判定：
// 结果判定为不完整/不可缓存时只返回数据、不写缓存（含 SWR 后台刷新路径）。
// 用于「闲时能完整、忙时被预算截断」的爬取类数据（tasks:all / executions:*）：
// 截断结果若固化 30 分钟，会让数据缺失变成"稳定错误"。
func swrLoadJSONGuarded[T any](
	cache *CacheService,
	entityType string,
	id int,
	ttl time.Duration,
	load func(ctx context.Context) (T, error),
	isEmpty func(T) bool,
	emptyMsg string,
	noCache func(T) bool,
) (T, bool, error) {
	var zero T
	if cache == nil {
		v, err := load(context.Background())
		if err != nil {
			return zero, false, err
		}
		if isEmpty != nil && isEmpty(v) {
			return zero, false, fmt.Errorf("%s", emptyMsg)
		}
		return v, false, nil
	}

	var lastLoad T
	loader := func(ctx context.Context) ([]byte, error) {
		v, err := load(ctx)
		if err != nil {
			return nil, err
		}
		lastLoad = v
		if isEmpty != nil && isEmpty(v) {
			return nil, fmt.Errorf("%s", emptyMsg)
		}
		return json.Marshal(v)
	}
	cacheable := func([]byte) bool {
		return noCache == nil || !noCache(lastLoad)
	}
	if noCache == nil {
		// 无判定需求时走原路径，避免闭包开销差异带来的行为漂移
		cacheable = nil
	}
	res, err := cache.GetOrLoadSWRGuarded(context.Background(), entityType, id, ttl, loader, cacheable)
	if err != nil {
		return zero, false, err
	}
	var v T
	if err := json.Unmarshal(res.Data, &v); err != nil {
		return zero, false, fmt.Errorf("解析缓存 %s 失败: %w", entityType, err)
	}
	return v, res.Stale, nil
}

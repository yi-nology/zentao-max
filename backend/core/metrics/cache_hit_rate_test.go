package metrics

import (
	"testing"
)

// 注意：metrics 包使用全局单例，必须先 Init。
// 为了避免污染全局状态，每个测试用例使用唯一的 cacheType 名。
func TestGetCacheHitRate(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	t.Run("未访问的类型返回0", func(t *testing.T) {
		got := GetCacheHitRate("test-not-exists-" + t.Name())
		if got != 0 {
			t.Errorf("GetCacheHitRate() = %v, want 0", got)
		}
	})

	t.Run("100%命中", func(t *testing.T) {
		ct := "test-100-hit-" + t.Name()
		// 重置该类型的计数（如果之前有）
		cacheCountersMu.Lock()
		cacheCounters[ct] = &cacheCounter{}
		cacheCountersMu.Unlock()
		RecordCacheHit(ct)
		RecordCacheHit(ct)
		RecordCacheHit(ct)
		got := GetCacheHitRate(ct)
		if got != 1.0 {
			t.Errorf("GetCacheHitRate() = %v, want 1.0", got)
		}
	})

	t.Run("50%命中率", func(t *testing.T) {
		ct := "test-50-hit-" + t.Name()
		cacheCountersMu.Lock()
		cacheCounters[ct] = &cacheCounter{}
		cacheCountersMu.Unlock()
		RecordCacheHit(ct)
		RecordCacheHit(ct)
		RecordCacheMiss(ct)
		RecordCacheMiss(ct)
		got := GetCacheHitRate(ct)
		if got != 0.5 {
			t.Errorf("GetCacheHitRate() = %v, want 0.5", got)
		}
	})

	t.Run("25%命中率", func(t *testing.T) {
		ct := "test-25-hit-" + t.Name()
		cacheCountersMu.Lock()
		cacheCounters[ct] = &cacheCounter{}
		cacheCountersMu.Unlock()
		RecordCacheHit(ct)
		RecordCacheMiss(ct)
		RecordCacheMiss(ct)
		RecordCacheMiss(ct)
		got := GetCacheHitRate(ct)
		if got != 0.25 {
			t.Errorf("GetCacheHitRate() = %v, want 0.25", got)
		}
	})
}

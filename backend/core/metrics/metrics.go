package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/adaptor"
	"github.com/oekit/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	// httpMetrics HTTP 指标委托 oekit/middleware 的 PrometheusMetrics
	// （http_requests_total / http_request_duration_seconds /
	//  http_requests_in_flight，路径标签用路由模板防高基数）
	httpMetrics *mw.PrometheusMetrics

	CacheHits    *prometheus.CounterVec
	CacheMisses  *prometheus.CounterVec
	CacheLatency *prometheus.HistogramVec

	ZentaoAPIRequests    *prometheus.CounterVec
	ZentaoAPIDuration    *prometheus.HistogramVec
	ZentaoAPIErrors      *prometheus.CounterVec
	ZentaoTokenRefreshes prometheus.Counter

	BugsTotal    *prometheus.GaugeVec
	StoriesTotal *prometheus.GaugeVec
	TasksTotal   *prometheus.GaugeVec
	TimelogTotal *prometheus.CounterVec
}

var globalMetrics *Metrics

// cacheCounters 维护按 cacheType 的命中/未命中计数（原子读写），
// 供 GetCacheHitRate 计算实时命中率（prometheus client_golang 读计数器值不便）
var (
	cacheCountersMu sync.RWMutex
	cacheCounters   = make(map[string]*cacheCounter)
)

type cacheCounter struct {
	hits   uint64
	misses uint64
}

func getCacheCounter(cacheType string) *cacheCounter {
	cacheCountersMu.RLock()
	c, ok := cacheCounters[cacheType]
	cacheCountersMu.RUnlock()
	if ok {
		return c
	}
	cacheCountersMu.Lock()
	defer cacheCountersMu.Unlock()
	// 双重检查
	if c, ok := cacheCounters[cacheType]; ok {
		return c
	}
	c = &cacheCounter{}
	cacheCounters[cacheType] = c
	return c
}

func Init() error {
	// HTTP 指标由 oekit PrometheusMetrics 创建并注册（与业务指标同一 Registry，
	// /metrics 端点统一暴露）；status label 更名 status_code（Grafana 面板已同步）
	httpMetrics := mw.NewPrometheusMetrics(mw.PrometheusConfig{
		Registry:  prometheus.DefaultRegisterer,
		SkipPaths: []string{"/metrics"},
	})
	m := &Metrics{
		httpMetrics: httpMetrics,

		CacheHits: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cache_hits_total",
				Help: "Total number of cache hits",
			},
			[]string{"cache_type"},
		),
		CacheMisses: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cache_misses_total",
				Help: "Total number of cache misses",
			},
			[]string{"cache_type"},
		),
		CacheLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "cache_operation_duration_seconds",
				Help:    "Cache operation duration in seconds",
				Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1},
			},
			[]string{"cache_type", "operation"},
		),

		ZentaoAPIRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "zentao_api_requests_total",
				Help: "Total number of Zentao API requests",
			},
			[]string{"endpoint", "method"},
		),
		ZentaoAPIDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "zentao_api_duration_seconds",
				Help:    "Zentao API request duration in seconds",
				Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
			},
			[]string{"endpoint", "method"},
		),
		ZentaoAPIErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "zentao_api_errors_total",
				Help: "Total number of Zentao API errors",
			},
			[]string{"endpoint", "method", "error_type"},
		),
		// ZentaoTokenRefreshes: 会话模式无 token 刷新，保留字段以兼容旧引用但实际无递增。
		ZentaoTokenRefreshes: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "zentao_token_refreshes_total",
				Help: "Total number of Zentao token refreshes (no-op in session mode)",
			},
		),

		BugsTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "bugs_total",
				Help: "Total number of bugs",
			},
			[]string{"product", "project", "status"},
		),
		StoriesTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "stories_total",
				Help: "Total number of stories",
			},
			[]string{"product", "project", "status"},
		),
		TasksTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "tasks_total",
				Help: "Total number of tasks",
			},
			[]string{"project", "execution", "status"},
		),
		TimelogTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "timelog_hours_total",
				Help: "Total hours logged",
			},
			[]string{"user", "project"},
		),
	}

	prometheus.MustRegister(
		m.CacheHits,
		m.CacheMisses,
		m.CacheLatency,
		m.ZentaoAPIRequests,
		m.ZentaoAPIDuration,
		m.ZentaoAPIErrors,
		m.BugsTotal,
		m.StoriesTotal,
		m.TasksTotal,
		m.TimelogTotal,
	)

	globalMetrics = m
	return nil
}

func Get() *Metrics {
	if globalMetrics == nil {
		panic("metrics not initialized, please call Init() first")
	}
	return globalMetrics
}

func Middleware() app.HandlerFunc {
	return Get().httpMetrics.Middleware()
}

func Handler() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		stdReq, _ := adaptor.GetCompatRequest(&c.Request)
		stdResp := adaptor.GetCompatResponseWriter(&c.Response)
		promhttp.Handler().ServeHTTP(stdResp, stdReq)
	}
}

func RecordCacheHit(cacheType string) {
	Get().CacheHits.WithLabelValues(cacheType).Inc()
	c := getCacheCounter(cacheType)
	cacheCountersMu.Lock()
	c.hits++
	cacheCountersMu.Unlock()
}

func RecordCacheMiss(cacheType string) {
	Get().CacheMisses.WithLabelValues(cacheType).Inc()
	c := getCacheCounter(cacheType)
	cacheCountersMu.Lock()
	c.misses++
	cacheCountersMu.Unlock()
}

func RecordCacheOperation(cacheType, operation string, duration time.Duration) {
	Get().CacheLatency.WithLabelValues(cacheType, operation).Observe(duration.Seconds())
}

func RecordZentaoAPIRequest(endpoint, method string, duration time.Duration, err error) {
	m := Get()
	m.ZentaoAPIRequests.WithLabelValues(endpoint, method).Inc()
	m.ZentaoAPIDuration.WithLabelValues(endpoint, method).Observe(duration.Seconds())

	if err != nil {
		errorType := "unknown"
		if err != nil {
			errorType = "error"
		}
		m.ZentaoAPIErrors.WithLabelValues(endpoint, method, errorType).Inc()
	}
}

func UpdateBugsTotal(product, project, status string, count float64) {
	Get().BugsTotal.WithLabelValues(product, project, status).Set(count)
}

func UpdateStoriesTotal(product, project, status string, count float64) {
	Get().StoriesTotal.WithLabelValues(product, project, status).Set(count)
}

func UpdateTasksTotal(project, execution, status string, count float64) {
	Get().TasksTotal.WithLabelValues(project, execution, status).Set(count)
}

func RecordTimelog(user, project string, hours float64) {
	Get().TimelogTotal.WithLabelValues(user, project).Add(hours)
}

// GetCacheHitRate 返回指定缓存类型的命中率（0.0 ~ 1.0）
// 若该类型无任何访问，返回 0
func GetCacheHitRate(cacheType string) float64 {
	cacheCountersMu.RLock()
	c, ok := cacheCounters[cacheType]
	cacheCountersMu.RUnlock()
	if !ok || c == nil {
		return 0
	}
	total := c.hits + c.misses
	if total == 0 {
		return 0
	}
	return float64(c.hits) / float64(total)
}

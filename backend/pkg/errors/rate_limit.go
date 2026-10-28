package errors

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/oekit/concurrency/ratelimit"

	"github.com/yi-nology/zentao-max/backend/pkg/logger"
)

// RateLimitConfig 限流配置.
type RateLimitConfig struct {
	// RequestsPerMinute 每 IP 每分钟允许的请求数（令牌桶语义：填充速率
	// rpm/60 每秒，桶容量 rpm——允许瞬时突发，持续超速被拒）
	RequestsPerMinute int
	// Deprecated: 令牌桶语义下无"封禁期"（超限请求等令牌，桶不满即拒），
	// 字段仅为 config.yaml 向后兼容保留。
	BlockDurationMinutes int
}

// DefaultRateLimitConfig 默认限流配置.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{RequestsPerMinute: 600, BlockDurationMinutes: 1}
}

// exemptPaths 不计入限流的基础设施端点：限流它们会导致监控/心跳把服务本身"打死"。
var exemptPaths = map[string]bool{
	"/health":      true,
	"/api/healthz": true,
	"/metrics":     true,
	"/api/version": true,
}

// NewRateLimiter 创建按 IP 限流器：oekit KeyedLimiter（空闲 TTL 自动清理）
// + TokenBucket 内核，替代原手写固定窗口计数 + 封禁期 + 清理 goroutine。
func NewRateLimiter(config RateLimitConfig) *ratelimit.KeyedLimiter {
	rpm := config.RequestsPerMinute
	if rpm <= 0 {
		rpm = 600
	}
	return ratelimit.NewKeyedLimiter(
		func() ratelimit.Limiter {
			return ratelimit.NewTokenBucketFloat(float64(rpm)/60.0, rpm)
		},
		10*time.Minute,
	)
}

// RateLimitMiddleware 使用默认配置的限流中间件.
func RateLimitMiddleware() app.HandlerFunc {
	return RateLimitMiddlewareWithConfig(600, 0)
}

// RateLimitMiddlewareWithConfig 按配置创建限流中间件.
// blockDurationMinutes 已废弃（令牌桶语义），参数保留兼容既有调用点。
func RateLimitMiddlewareWithConfig(requestsPerMinute, blockDurationMinutes int) app.HandlerFunc {
	return rateLimitMiddleware(NewRateLimiter(RateLimitConfig{RequestsPerMinute: requestsPerMinute}))
}

func rateLimitMiddleware(limiter *ratelimit.KeyedLimiter) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		path := string(c.Request.URI().Path())
		if exemptPaths[path] {
			c.Next(ctx)
			return
		}

		// 限流 key 用直连地址（去端口）而非 ClientIP：hertz 默认信任任意网段的
		// X-Forwarded-For，攻击者每请求换一个伪造 XFF 即可让 ClientIP 生成
		// 无限个新桶（限流形同虚设）。与 oekit/middleware ratelimit 的
		// defaultIPKey 同款防护（#13）；代理拓扑下如需按真实客户端 IP 限流，
		// 须先配置可信网段再切回 ClientIP。
		ip, _, err := net.SplitHostPort(c.RemoteAddr().String())
		if err != nil {
			ip = c.RemoteAddr().String()
		}

		if !limiter.Allow(ip) {
			logger.Warn("rate limit blocked", "ip", ip, "path", path)
			c.JSON(http.StatusTooManyRequests, Response{
				Code:    http.StatusTooManyRequests,
				Message: "请求过于频繁，请稍后再试",
				Data: map[string]interface{}{
					"retryAfter": 1,
				},
			})
			c.Abort()
			return
		}

		c.Next(ctx)
	}
}

package errors

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
)

type RateLimiter struct {
	requests map[string]*ClientInfo
	mu       sync.RWMutex
	config   RateLimitConfig
}

type ClientInfo struct {
	Count     int
	ResetTime time.Time
	Blocked   bool
}

type RateLimitConfig struct {
	RequestsPerMinute int
	BlockDuration     time.Duration
	CleanupInterval   time.Duration
}

func DefaultRateLimitConfig() RateLimitConfig {
	// 默认 600 次/分钟：前端单页加载会并发请求多个接口，
	// 60 次/分钟在正常使用下极易触发，且触发后封禁 5 分钟会让应用完全不可用。
	requestsPerMinute := 600
	if val := os.Getenv("RATE_LIMIT_REQUESTS_PER_MINUTE"); val != "" {
		if num, err := strconv.Atoi(val); err == nil && num > 0 {
			requestsPerMinute = num
		}
	}

	// 触发后封禁 1 分钟（原 5 分钟过长，相当于把用户彻底踢出）。
	blockDuration := 1 * time.Minute
	if val := os.Getenv("RATE_LIMIT_BLOCK_DURATION_MINUTES"); val != "" {
		if num, err := strconv.Atoi(val); err == nil && num > 0 {
			blockDuration = time.Duration(num) * time.Minute
		}
	}

	return RateLimitConfig{
		RequestsPerMinute: requestsPerMinute,
		BlockDuration:     blockDuration,
		CleanupInterval:   1 * time.Minute,
	}
}

func NewRateLimiter(config RateLimitConfig) *RateLimiter {
	limiter := &RateLimiter{
		requests: make(map[string]*ClientInfo),
		config:   config,
	}

	go limiter.cleanupTask()

	return limiter
}

func (rl *RateLimiter) cleanupTask() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, info := range rl.requests {
			if now.After(info.ResetTime) {
				delete(rl.requests, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) Allow(ip string) (bool, int, time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	info, exists := rl.requests[ip]

	if !exists {
		rl.requests[ip] = &ClientInfo{
			Count:     1,
			ResetTime: now.Add(1 * time.Minute),
			Blocked:   false,
		}
		return true, rl.config.RequestsPerMinute - 1, rl.requests[ip].ResetTime
	}

	if info.Blocked {
		if now.Before(info.ResetTime) {
			return false, 0, info.ResetTime
		}
		info.Blocked = false
		info.Count = 0
		info.ResetTime = now.Add(1 * time.Minute)
	}

	if now.After(info.ResetTime) {
		info.Count = 0
		info.ResetTime = now.Add(1 * time.Minute)
	}

	if info.Count >= rl.config.RequestsPerMinute {
		info.Blocked = true
		info.ResetTime = now.Add(rl.config.BlockDuration)
		log.Printf("[RATE_LIMIT] IP %s blocked for %v due to exceeding rate limit", ip, rl.config.BlockDuration)
		return false, 0, info.ResetTime
	}

	info.Count++
	remaining := rl.config.RequestsPerMinute - info.Count
	return true, remaining, info.ResetTime
}

func RateLimitMiddleware() app.HandlerFunc {
	config := DefaultRateLimitConfig()
	limiter := NewRateLimiter(config)

	// 不计入限流的路径：健康检查、指标、初始化状态、版本号。
	// 这些是基础设施端点，限流它们会导致监控/心跳把服务本身"打死"，
	// 也会让前端路由守卫（getInitStatus）在正常浏览时被误伤。
	exemptPaths := map[string]bool{
		"/health":          true,
		"/api/healthz":     true,
		"/metrics":         true,
		"/api/init/status": true,
		"/api/version":     true,
	}

	log.Printf("[RATE_LIMIT] Rate limiter initialized: %d requests/minute, block duration: %v",
		config.RequestsPerMinute, config.BlockDuration)

	return func(ctx context.Context, c *app.RequestContext) {
		path := string(c.Request.URI().Path())
		if exemptPaths[path] {
			c.Next(ctx)
			return
		}

		ip := c.ClientIP()

		allowed, remaining, resetTime := limiter.Allow(ip)

		c.Header("X-RateLimit-Limit", strconv.Itoa(config.RequestsPerMinute))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(resetTime.Unix(), 10))

		if !allowed {
			log.Printf("[RATE_LIMIT] Request blocked from IP: %s, Path: %s", ip, string(c.Request.URI().Path()))
			c.JSON(http.StatusTooManyRequests, Response{
				Code:    http.StatusTooManyRequests,
				Message: "请求过于频繁，请稍后再试",
				Data: map[string]interface{}{
					"retryAfter": resetTime.Sub(time.Now()).Seconds(),
					"resetAt":    resetTime.Format(time.RFC3339),
				},
			})
			c.Abort()
			return
		}

		c.Next(ctx)
	}
}

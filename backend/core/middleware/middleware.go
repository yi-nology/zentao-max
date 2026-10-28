package middleware

import (
	"context"
	"time"

	"github.com/yi-nology/zentao-max/backend/core/logger"
	"github.com/yi-nology/zentao-max/backend/core/metrics"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/oekit/middleware/hmw"
)

// trace_id 与 panic 防护由 oekit/middleware 提供（routes.go 注册）：
//   - hmw.Middleware()：trace_id 生成/透传（X-Trace-Id，12 位 hex）+ 客户端
//     ID 白名单校验，同步注入 RequestContext 与标准 context（logx 经
//     traceid.FromContext 提取，Ctx* 日志自动携带）
//   - mw.RecoveryMiddleware()：panic 捕获 + logx 记录堆栈，响应格式经
//     WithRecoveryErrorHandler 定制为项目 errors.Response 结构
//
// 本包只保留访问日志与指标两个中间件。

func LoggerMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		start := time.Now()
		path := string(c.Request.URI().Path())
		method := string(c.Request.Method())

		c.Next(ctx)

		latency := time.Since(start)
		statusCode := c.Response.StatusCode()

		kv := []any{
			"method", method,
			"path", path,
			"status", statusCode,
			"latency", latency,
			"client_ip", c.ClientIP(),
			"trace_id", hmw.FromRequestContext(c),
		}

		if statusCode >= 500 {
			logger.Error("HTTP Request", kv...)
		} else if statusCode >= 400 {
			logger.Warn("HTTP Request", kv...)
		} else {
			logger.Info("HTTP Request", kv...)
		}
	}
}

func MetricsMiddleware() app.HandlerFunc {
	return metrics.Middleware()
}

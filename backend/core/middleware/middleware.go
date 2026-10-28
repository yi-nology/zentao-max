package middleware

import (
	"context"
	"runtime/debug"
	"time"

	"github.com/yi-nology/zentao-max/backend/core/errors"
	"github.com/yi-nology/zentao-max/backend/core/logger"
	"github.com/yi-nology/zentao-max/backend/core/metrics"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/google/uuid"
	)

func TraceIDMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		traceID := string(c.GetHeader("X-Trace-ID"))
		if traceID == "" {
			traceID = uuid.New().String()
		}

		c.Set(string(logger.TraceIDKey), traceID)

		c.Header("X-Trace-ID", traceID)

		c.Next(ctx)
	}
}

func LoggerMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		start := time.Now()
		path := string(c.Request.URI().Path())
		method := string(c.Request.Method())

		c.Next(ctx)

		latency := time.Since(start)
		statusCode := c.Response.StatusCode()
		traceID, _ := c.Get(string(logger.TraceIDKey))
		traceIDStr, _ := traceID.(string)

		kv := []any{
			"method", method,
			"path", path,
			"status", statusCode,
			"latency", latency,
			"client_ip", c.ClientIP(),
			"trace_id", traceIDStr,
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

func RecoveryMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		defer func() {
			if err := recover(); err != nil {
				traceID, _ := c.Get(string(logger.TraceIDKey))
				traceIDStr, _ := traceID.(string)

				logger.Error("Panic recovered",
					"error", err,
					"trace_id", traceIDStr,
					"method", string(c.Request.Method()),
					"path", string(c.Request.URI().Path()),
					"stack", string(debug.Stack()),
				)

				c.AbortWithStatusJSON(500, errors.Response{
					Code:    errors.CodeInternalError,
					Message: "服务器内部错误",
					Data:    nil,
				})
			}
		}()

		c.Next(ctx)
	}
}

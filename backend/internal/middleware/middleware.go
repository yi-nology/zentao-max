package middleware

import (
	"github.com/yi-nology/zentao-max/backend/pkg/metrics"

	"github.com/cloudwego/hertz/pkg/app"
)

// trace_id 与 panic 防护由 oekit/middleware 提供（routes.go 注册）：
//   - hmw.Middleware()：trace_id 生成/透传（X-Trace-Id，12 位 hex）+ 客户端
//     ID 白名单校验，同步注入 RequestContext 与标准 context（logx 经
//     traceid.FromContext 提取，Ctx* 日志自动携带）
//   - mw.RecoveryMiddleware()：panic 捕获 + logx 记录堆栈，响应格式经
//     WithRecoveryErrorHandler 定制为项目 errors.Response 结构
//   - mw.AccessLogMiddleware()：访问日志（method/path/status/latency/
//     client_ip/trace_id），/metrics、/health 噪音路径跳过
//
// 本包只保留指标中间件。

func MetricsMiddleware() app.HandlerFunc {
	return metrics.Middleware()
}

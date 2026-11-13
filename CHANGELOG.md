# zentao-max CHANGELOG

## [v0.10.0] — 2026-11-13

### **BREAKING**：MCP 短别名路由删除

删除 8 条早期集成的 MCP 短别名 POST 路由：`/mcp/products`、`/mcp/projects`、
`/mcp/executions`、`/mcp/bugs`、`/mcp/stories`、`/mcp/tasks`、`/mcp/users`、`/mcp/timelog`。
**下游 MCP 客户端必须改用工具全名端点**（如 `/mcp/get_bugs`）或统一入口
`POST /mcp`（`{"action":"get_bugs","params":{…}}`）/ `GET /mcp?action=get_bugs`。

### **BREAKING**：config.yaml 兼容字段删除

- `rate_limit.block_duration_minutes` 键从配置结构删除（旧固定窗口封禁期残留，
  令牌桶语义下从未生效）。yaml 解析为非严格模式，旧配置文件残留该键不影响启动，
  但应同步清理。环境变量 `ZENTAO_MAX_RATE_LIMIT_BLOCK_DURATION_MINUTES` 同步失效
- `errors.RateLimitMiddlewareWithConfig(rpm, blockDurationMinutes)` 签名收窄为
  `(requestsPerMinute int)`；`RateLimitConfig.BlockDurationMinutes` 字段、
  `RateLimitConfig.GetBlockDuration()`、零调用的 `DefaultRateLimitConfig()` /
  `RateLimitMiddleware()` 一并删除

### 其他兼容清理

- 删除互相成孤岛的 Deprecated 未导出方法 `getCasesByProductSession` / `getAllCasesSession`
  （连同仅其使用的 `sessionTestCaseBrowseResp`/`sessionCase` 实体与对应映射测试；
  生产路径早已走 `getCasesByProductV1`）
- 删除 `logger.Sync()` 幂等空实现与 `cmd/mcp` 的 `defer logger.Sync()` 兼容调用点
- 删除 `StdioTransport.Start()`（后台 goroutine 旧用法）：both 模式改为
  `go transport.Run(ctx)`；`Run` 不再隐式改写传输模式，stdio 入口（cmd/mcp）
  自行 `SetTransport(TransportStdio)`，与 MCPModeManager 设计注释一致
- 删除零递增零读取的 metrics 死字段 `ZentaoTokenRefreshes`（`zentao_token_refreshes_total`）
- 删除 `sdk.FlexibleString.String()` 兼容方法（与 common 仓同口径，无调用方）

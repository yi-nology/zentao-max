# zentao-max 架构说明

> 2026-10 起仓库完成两轮大改造：**纯 Web 单二进制**（Wails 桌面模式移除）+ **目录结构对齐
> octopus**（`backend/core` 平铺 → `internal/` + `pkg/` 分层）+ **oekit 生态深度接入**。
> 本文描述当前形态；历史重构（Wire DI、三入口）的说明已过时，均已被下文取代。

## 1. 总体形态

```
┌─────────────────────────── 单一二进制 zentao-max ───────────────────────────┐
│  backend/main.go（唯一入口）                                               │
│  ├─ 配置加载: 默认值 → config.yaml → env（oekit core/config）               │
│  ├─ 凭据解析: ldflags/config/env → auth.db（AES 加密, ResolveAuthDBPath）   │
│  ├─ oekit bootstrap: 组件化启动 + SIGINT/SIGTERM 优雅停机                   │
│  └─ HTTPApp (internal/svc)                                                 │
│      └─ Hertz server: gzip → accesslog → recovery → metrics → rate_limit   │
│                      → auth middleware → 路由（biz/hz + internal 手写路由） │
│          ├─ /            嵌入式 Vue 3 SPA（embed static/*，或 -static 磁盘）│
│          ├─ /api/*       REST 业务 API（读匿名开放、写需平台登录）           │
│          ├─ /mcp/*       MCP HTTP 入口（独立 token/只读控制）               │
│          ├─ /metrics     Prometheus 指标                                   │
│          └─ /health      liveness                                          │
└─────────────────────────────────────────────────────────────────────────────┘
              │ Session/Token 双模式（realm 决定）+ 熔断 + 池化 + 二级缓存
              ▼
        禅道实例（标准开源版 REST / 受限 biz8.x *.json 会话端点）
```

另有一个独立子进程入口 `backend/cmd/mcp`（MCP stdio 模式，供 AI 工具作为子进程拉起）。

## 2. 目录分层（对齐 octopus）

```
backend/
├── main.go                 入口：flag 解析(-config/-env/-static) → config 加载
│                           → svc.InitializeHTTPApp（手写装配）→ bootstrap.Run
├── cmd/mcp/                MCP stdio 入口
├── biz/                    hz 生成层（IDL → handler/model/router），业务实现在 internal
├── internal/               私有业务代码
│   ├── svc/                AppConfig + Dependencies + HTTPApp 生命周期（手写装配；
│   │                       assembly.go（google/wire 已移除）
│   ├── config/             oekit core/config：DefaultConfig() → yaml → env(ZENTAO_MAX_)
│   ├── routes/             手写路由补充（v1 efforts/mcp 管理/logs/scheduler 等）
│   ├── middleware/         业务 MetricsMiddleware（其余中间件由 oekit 提供）
│   ├── handlers/           HTTP handlers + HandlerRegistry（单例装配）
│   ├── service/            业务服务：CacheService（业务层缓存）、dashboard 聚合
│   ├── initialization/     InitService（auth.db 凭据）、ConfigStore（cron.db）、
│   │                       ResolveAuthDBPath（auth.db 路径统一解析）
│   ├── auth/               平台访问控制：匿名只读/登录读写、SSO 短信登录、HMAC token
│   ├── mcp/                MCP 子系统：24 工具、JSON-RPC 2026-07-28、热重载、stdio
│   ├── repository/sqlite/  业务层 SQLite 缓存（modernc.org/sqlite，pure Go）
│   ├── infra/zentao/       禅道 client/SDK（双认证、SessionWorkerPool、SessionCache、
│   │                       breaker 熔断、SSO 登录限流）
│   └── dto/ vo/ models/ event/ version/
└── pkg/                    logger(logx/slog + ring buffer)、errors、constants、
                            metrics、utils
```

依赖方向：`main → svc → handlers → service → infra/zentao`；`pkg` 不依赖 `internal`。

## 3. oekit 生态接入清单

| 领域 | oekit 组件 | 替代的原手写实现 |
|------|-----------|------------------|
| 日志 | `logx`（slog 后端）+ 自定义 fanout/ring handler | zap（zap.Field 全量改 kv） |
| 配置 | `core/config`（LoadFile + ApplyEnvOverlay） | viper + godotenv |
| 生命周期 | `bootstrap`（Component 化启动/停机） | 手写 signal 处理 |
| trace | `middleware/hmw` + `tracing/traceid`（12-hex trace_id，X-Trace-Id 透传） | 手写 TraceIDMiddleware |
| 恢复 | `mw.RecoveryMiddleware`（保留项目 50000 错误体） | 手写 recover |
| 访问日志 | `mw.AccessLogMiddleware`（/metrics、/health 跳过） | 手写 LoggerMiddleware |
| HTTP 指标 | `mw.PrometheusMetrics`（路由模板 path 标签，`status_code` label） | 手写高基线 raw-path 指标 |
| 并发安全 | `async.GoSafe`（全部业务 goroutine） | 手写 recover goroutine |
| 击穿 | `concurrency/singleflight`（CacheService + SessionCache） | 手写 flight |
| 熔断 | `concurrency/breaker`（SessionTransport 出站） | 无 |
| 限流 | `concurrency/ratelimit` KeyedLimiter + TokenBucket | 手写固定窗口+封禁 |
| 并发组 | `concurrency/group`（healthz 并行检查）、`fanout`（dashboard） | 手写 WaitGroup/fanout |
| 出站 HTTP | `pkg/httpjson.DoRaw`（webhook） | 手写 POST |
| 工具 | `pkg/slicex`（filter/convert 委托） | 手写 filter |
| 压缩 | hertz-contrib `gzip`（/metrics 除外） | 无 |
| ID | `kitx/uidgen.RandomHex`（trace_id） | uuid |

**刻意保留手写**（评估结论记录在 git 历史）：禅道协议状态机（sid 过期→relogin→replay，
SSO 节流规避核心资产）、SessionWorkerPool 三队列（读并发/写串行/预取 + 令牌信号量）、
业务错误码体系、业务 metrics、robfig/cron（多任务同表达式语义）、分页 start/end 契约。

## 4. 请求链路（以 GET /api/bugs 为例）

```
Hertz
 → gzip.Gzip                     响应压缩（/metrics 除外）
 → mw.AccessLogMiddleware        访问日志（trace_id/method/path/status/latency）
 → mw.RecoveryMiddleware         panic → 50000 错误体
 → mw.PrometheusMetrics          http_requests_total{status_code} 等
 → RateLimitMiddleware           oekit 令牌桶，RemoteAddr 主机维度
 → auth.Middleware               匿名 GET 放行；写操作要求平台 token
    （豁免：/mcp*、/api/auth/*、/api/scheduler/preview、基础设施路径）
 → biz/hz 或 internal/handlers   HandlerRegistry 单例
 → service（业务缓存 GetOrLoad，singleflight 合并同 key 并发）
 → infra/zentao.Client           Token 或 Session 模式分发
     └─ SessionTransport → breaker → SessionWorkerPool（限流/串行写/降级）
                          → SessionCache L1/L2（TTL + singleflight + 写后失效）
                          → HTTP(s) → 禅道实例
```

错误约定：业务错误统一 `pkg/errors` 错误码（50002 = 禅道服务调用失败，消息附带根因；
429/401/503 语义见 `pkg/errors` 各文件）。

## 5. 双缓存层

| 层 | 位置 | key | 用途 |
|----|------|-----|------|
| 业务层 `service.CacheService` | `internal/service` + `repository/sqlite`（`~/.zentao-max/cache.db`） | `(entityType, productID)` | VO 级缓存，dashboard/列表页加速；singleflight 防击穿 |
| client 层 `SessionCache` | `internal/infra/zentao`（`./cache-session.db`） | `(ns, entity, scope)` 命名空间含 server/account/realm | 原始端点响应缓存，per-entity TTL + stale 降级；翻页 pages 2+ 强制绕过（防同 scope 重复数据） |

两层职责不同，同时存在。写操作成功后按 `writeOpToEntity` 映射失效对应实体。

## 6. 装配与生命周期

`svc.InitializeHTTPApp`（`internal/svc/assembly.go`，手写）：

1. `initialization.NewInitService(authDBPath, encryptionKey)` — auth.db 凭据服务；
   `authDBPath` 为空时经 `ResolveAuthDBPath` 解析（显式 → `ZENTAO_AUTH_DB_PATH` →
   已存在候选 → 默认 `./data/auth.db`）
2. `initialization.NewConfigStore` — 定时任务存储（`./data/cron.db`）
3. `provideZentaoClient` — 凭据优先级 ldflags/config → env → auth.db；按 realm
   决定 Token/Session 模式，装配缓存前缀与池化参数
4. `NewDependencies` + `handlers.NewHandlerRegistry` — handler 单例
5. `NewHTTPApp` — Hertz 引擎 + 中间件栈 + 路由 + 静态伺服；注册进 oekit `bootstrap`，
   与定时任务、session 保活等 Component 一起启动；SIGTERM 走统一优雅停机链

## 7. 平台认证与 MCP

- **平台认证**（`internal/auth`）：匿名只读（GET/HEAD/OPTIONS），管理员读写。
  SSO（麒麟统一认证：RSA 密码 + 短信验证码 + doLogin）为默认登录入口，
  本地 admin 为备用；HMAC 无状态 token（`exp:username.签名`），Cookie + Bearer 双通道。
  细节与端点见 [AGENTS.md](../AGENTS.md)。
- **MCP**（`internal/mcp`）：24 工具（16 查询：列表 9 + 详情/个人 5 + 配置诊断 2；8 受控写：
  record_effort / add_bug_comment / transition_bug / create_bug / transition_task +
  update_zentao_config / send_verify_code / sso_login——配置变更与认证操作同属
  read_only 拦截范围）；HTTP（`POST /mcp/{action}` 简化协议 + JSON-RPC 2026-07-28）与
  stdio 双入口（stdio 日志强制 stderr，防污染协议流）；token 哈希鉴权、enabled 开关、
  read_only 拦截、action 白名单，均可经 `/api/v1/mcp/config` 热重载。
  `WithConfigDeps` 向 MCP 注入 InitService（update_zentao_config 的 auth.db 持久化）
  与 SSOClient（麒麟短信登录两步），HTTP（routes.go）与 stdio（cmd/mcp）两入口接线。

## 8. 配置加载链

```
DefaultConfig()                    internal/config/config.go（结构体化默认值，port 12345 等）
  → config.yaml                    oekit core/config LoadFile（缺失 OK，解析失败 fatal）
  → 环境变量覆盖                    ApplyEnvOverlay，ZENTAO_MAX_ 前缀（server.port →
                                   ZENTAO_MAX_SERVER_PORT）
凭据单独走: ldflags/config/env（ZENTAO_SERVER/... 与 ZENTAO_MAX_ 前缀双形式）
           → auth.db AES 加密存储（密钥: security.encryption_key / ZENTAO_ENCRYPTION_KEY，
             未设置时用托管密钥文件 data/encryption.key——历史 hostname 派生方案
             因 hostname 随网络漂移致 auth.db 反复解密失败，已改为回退兼容）
```

## 9. 构建与发布

- 本地：`make release`（前端 build → copy-static → trimpath 单二进制）
- 交叉编译：纯 Go（hertz + modernc sqlite 无 CGO），`CGO_ENABLED=0 GOOS=... go build ./backend`
- CI（`.github/workflows/build.yml`「Build zentao-max」）：单 ubuntu runner 构建
  linux amd64/arm64 / windows amd64 / darwin arm64 四目标；`v*.*.*` tag 产物附加到 Release
- ⚠️ 嵌入构建前必须先 `./scripts/copy-static.sh` 且先清空 `backend/static`（cp 非 mirror）

## 10. 已知限制

- 受限 biz8.x 实例的 stories 分页 pages 2+ 返回空（分页 URL 语义待权威抓包确认）
- 任务/工时跨执行遍历（无产品级任务端点）：60s 遍历门控 + dashboard 90s 总超时，
  超时返回部分数据；首查仍慢
- auth.db 解密失败（机器派生密钥不一致/密钥文件丢失）→ 固定 `ZENTAO_ENCRYPTION_KEY`
  或连同 `encryption.key` 一起搬迁；legacy 回退可救历史文件

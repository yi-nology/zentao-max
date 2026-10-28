# AGENTS.md — zentao-max

## Project Overview

**Wails v3 alpha** desktop app (Go + Vue 3) for Zentao (禅道) project management. Also deployable as a standalone HTTP server or embedded app with static frontend.

**Go module name is `github.com/yi-nology/zentao-max`**. Import paths use `github.com/yi-nology/zentao-max/backend/...`.

> ⚠️ **Wails v3 still in alpha** (`v3.0.0-alpha2.117` as of 2026-07). API may churn between versions. Pin the version in `go.mod` rather than chasing `@latest`.

## Architecture

```
main.go + app.go          → Wails v3 entrypoint
                             - application.New + Service registration
                             - SystemTray (native, v3 新增)
                             - GlobalShortcut (CmdOrCtrl+Shift+Z 唤起)
                             - EventBus subscription → app.Event.Emit
backend/
  cmd/server/main.go      → HTTP server entrypoint (config-driven)
  cmd/app/main.go         → Embedded app entrypoint (static files baked in via ldflags)
  core/
    app/                  → Application interface, Wire DI
    event/                → In-process event bus (pub/sub)
    handlers/             → HTTP handlers (cache.go / logs.go)
    service/              → Business logic (cache_service / dashboard_service)
    storage/              → SQLite offline cache (modernc.org/sqlite, pure Go)
    logger/               → zap + ring_buffer
    metrics/              → Prometheus + cache hit rate
frontend/
  src/                    → Vue 3 + Element Plus + Chart.js
    composables/          → useTableColumns / useTheme / useDesktopNotification / useExternalLink
    utils/export.ts       → Excel/CSV/PDF exporter
  bindings/               → wails3 generate 输出（不要手改）
build/                    → v3 构建系统（Taskfile + 平台子目录）
Taskfile.yml              → v3 主构建入口
docs/grafana/             → Grafana dashboard JSON
```

Three runtime modes:
- **Wails desktop**: `wails3 task dev` / `make run`（启用 SystemTray / 全局快捷键 / 桌面通知）
- **HTTP server**: `cd backend && go run cmd/server/main.go`
- **Embedded app**: `cd backend && go run cmd/app/main.go`

## Commands

### Development
```bash
make run                  # wails3 task dev (hot reload frontend + Go)
make frontend-dev         # Vite dev server only
cd backend && make run    # HTTP server only
```

### Build
```bash
make build                # wails3 task build（当前平台）
make release              # wails3 task package（带打包）
wails3 task linux:build   # 指定平台（需在该平台环境或 Docker）
make wails-generate       # 重新生成前端 bindings（改 App 方法后必须）
```

### Test & Lint
```bash
cd backend && make check              # fmt + lint + test
cd backend && make test               # go test
cd frontend && npm run type-check     # vue-tsc --noEmit
cd frontend && npm run build          # vite build
```

### Cross-compilation
v3 + CGO 不再支持 macOS 上交叉编译 Linux/Windows。三个选项：
- **CI**: 推送到 master/tag，GitHub Actions 在对应平台 runner 构建
- **Docker**: `wails3 task setup:docker && wails3 task linux:build`
- **server-only**（无 GUI）: `cd backend && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/server`

## Key Conventions & Gotchas

- **Wire DI**: `backend/core/app/wire.go` 定义注入；改 providers 后 `wire ./backend/core/app/`
- **Frontend env switching**: 构建脚本 `cp .env.wails .env`，`.env` gitignored
- **Config priority**: AppConfig > database stored config > env vars (`ZENTAO_MAX_` prefix)
- **Default HTTP port**: `12345`
- **Generated code**: `frontend/bindings/` 是 wails3 generate 自动生成的，不要手改。改 `app.go` 的 App 方法后必须运行 `wails3 generate bindings -d frontend/bindings` 或 `make wails-generate`
- **Wails v3 Service 模式**: App 实现 `ServiceName/ServiceStartup/ServiceShutdown` 接口，通过 `application.NewService(app)` 包装注入到 `application.Options.Services`
- **Static files for embedded app**: `scripts/copy-static.sh` 复制 `frontend/dist/` → `backend/cmd/app/static/`
- **CI**: push to `master` 或 `v*.*.*` tag 触发，三平台 artifacts

## Feature Behaviors (mode-specific)

| Feature | Wails desktop | HTTP / Embedded |
|---------|--------------|-----------------|
| SystemTray（原生托盘）| ✅ v3 原生 | ❌ |
| GlobalShortcut（全局快捷键 CmdOrCtrl+Shift+Z）| ✅ v3 原生 | ❌ |
| Desktop notifications | ✅ app.Event.Emit | ❌（EventBus 无订阅者）|
| Hide-to-tray | ✅ | N/A |
| Offline SQLite cache | ✅ | ✅ |
| Log viewer / Grafana | ✅ | ✅ |

## Platform Auth (平台访问控制, 自 zentao-mini 同步)

`backend/core/auth`：匿名只读（GET/HEAD/OPTIONS 放行），登录管理员读写。

- **SSO 登录（默认入口）**：`/api/auth/sso/send-code` + `/api/auth/sso/login`——麒麟统一认证
  （sso.example.com）用户名+密码+短信验证码：密码用登录页硬编码 RSA 公钥加密（PKCS#1 v1.5），
  POST /mobiles 拿脱敏手机号 → /verifyCode 触发短信 → /sso/doLogin 验证码登录（移植自
  freedom/ky-work-mcp 的实测实现）。成功后签发平台会话令牌（与本地登录同一会话体系）。
  配置：`auth.sso.enabled`（默认 true）+ `auth.sso.base_url`。
- **本地管理员（备用入口）**：`auth.admin.username/password`（默认 admin/admin，env
  `ZENTAO_MAX_AUTH_ADMIN_USERNAME/PASSWORD`）。登录页底部链接切换两种方式。
- **会话**: HMAC 无状态 token，格式 `exp[.username]分段签名`——令牌内记录登录者身份
  （SSO 账号或本地管理员名），`/api/auth/status` 返回该身份；Cookie `zentao_max_session`
  + `Authorization: Bearer` 双通道；`SecretFromEncryptionKey` 派生签名密钥，重启后有效。
  登录失败防爆破（10 次/分钟/IP，SSO 两步共用计数）。
- **豁免**: `/mcp*`（MCP 有独立 token/只读控制）；`/api/auth/*`（登录/登出/SSO 两步）；
  `/api/scheduler/preview`（纯读）；首次启动时 `/api/init/login`、`/api/v1/init/login`、
  `/api/init/zentaosid` 匿名放行（否则全新部署死锁），初始化完成后同样需要登录。
- **前端**: `/login` 页默认 SSO 表单（账号/密码/验证码+60s 倒计时重发），底部可切本地管理员；
  头部不展示模式徽章——登录后只显示用户名+退出，未登录只显示登录链接（模式守卫由
  后端 401 层承担）；`api.ts` 对 401 分流——`/auth/*` 自身 401 表单自展示，含"管理员登录"
  字样跳 `/login?redirect=...`，其余（禅道配置失效）跳 `/init-guide`。
- **注意**: 初始化完成后再走 `/init-guide` 重新配置禅道连接也需要先登录。限流为配置驱动
  （`errors.RateLimitMiddlewareWithConfig`，默认 600 次/分钟）。

## Skills (Agent 技能包, `skills/`)

仓库内置两个可分发的 Agent 技能（直连禅道实例的 session 方式，支持 realm 统一认证域，
凭证走 `~/.zentao-credentials.env`，模板见 `skills/config.example.env`；配置含 SSH 跳板机
隧道支持）。改这两个技能时同步更新其 SKILL.md：

- **`skills/zentao-timelog/`**：Excel 批量填报禅道工时/任务日志（`scripts/zt.py`：whoami/tasks/plan/submit/verify）。核心红线：先 plan 再 submit、`left[1]` 必发且为绝对值、AJAX 外壳 `{"status":"success"}` 必须穿透 `data` 判定且写后必回读、失败行不入台账可安全重跑、工时被外部系统同步后只能网页改。
- **`skills/zentao-bug/`**：BUG 查询（`get_bug.py`，`bug-view-{id}.json` 外层 data 需二次解析）、解决/关闭/编辑（`close_bug.ps1`）与质量分析五字段批量填写（`set_quality_fields.ps1`：fixedPackageVersion + reasonAnalysis/advice/implementation/testAdvise 一次 `bug-edit` 全量写全，必须带 openedBuild/execution/title 原值）。

与后端 API 的关系：Bug 流转、批量工时（`POST /api/v1/efforts/batch`）等能力应用本身已提供
REST/MCP 入口；技能包覆盖的是应用 API 未暴露的实例侧细节（质量分析字段、Excel 工作流、
biz8.x 全量更新语义、跳板机隧道）。

**应用页面已吸收的技能能力**（2026-10-28，页面内即可用，无需 CLI）：

- **Bug 质量分析字段**：Bugs 页详情弹窗「编辑 / 质量字段」对话框，一次 `bug-edit` 全量写全
  fixedPackageVersion/reasonAnalysis/advice/implementation/testAdvise（+可选 resolution/
  resolvedBuild/comment）。`BugVO.Execution` 已透出（bug-edit 全量更新必带 execution 原值），
  提交前 title 的 `&#039;` 自动还原（防二次转义）。对应技能踩坑 #13/#17/#22。
- **工时 Excel 导入**：EffortCalendar 批量弹窗「导入 Excel」，复刻 zt.py 语义——表头前 10 行
  自动定位、日期归一（`2026/1/15`、`1.15`、`今天/昨天`、Excel 序列号）、对象号归一
  （`任务#123`/`BUG#123`/`123`）、剩余工时留空按任务当前剩余累计扣减（手填=绝对值）、
  台账式去重（日历已有同日同对象同消耗即跳过）、`#` 注释行跳过、消耗必须 >0；
  提交按行日期分组逐天调 `/api/v1/efforts/batch`，完成后回读日历核对。
  红线继承：剩余=0 会把任务置完成（弹窗内有警告提示）；接口成功 ≠ 落库，以回读为准。

## Authentication Modes (双认证模式)

应用支持两种禅道认证与数据访问模式，按配置的 `realm` 字段自动切换：

| 模式 | 认证 | 数据访问 | 适用 |
|------|------|----------|------|
| **Token 模式**（默认） | 用户名/密码换 REST Token | `/api.php/v1/*` REST 端点（Token 头） | 标准禅道开源版/企业版 |
| **Session 模式** | `/user-login.html` PHP 会话 | `*.json` 端点（zentaosid cookie） | 禁用 REST API 的禅道实例（如受限实例 your-zentao-server.com） |

**配置方式**：
- `~/.zentao-max/auth.db` 里 `AuthConfig.Realm` 非空（如 `"kydc"`）→ Session 模式
- 前端登录表单选择认证域：`kydc`（受限实例 SSO，密码明文）/ `local`（禅道本地，md5 混淆）
- 环境变量：`ZENTAO_MAX_REALM` / `ZENTAO_REALM`

**实现关键文件**（`backend/core/zentao/`）：
- `client_auth_mode.go` — `AuthMode` / `RealmKySSO` 常量
- `client_session.go` — `SessionTransport`（登录、cookie jar、双层 JSON 解包、re-login 重试）
- `client_session_*.go` — 各实体的 `.json → SDK` 映射（bugs/stories/tasks/products/projects/executions/users/builds/efforts/cases/plans/programs/releases/testtasks）
- `client_writes*.go` — 写操作（bug/task/story/... 的 create/update/delete/resolve/close/start/finish 等），token + session 双模式
- `client.go` 的 `NewSessionClient` / `UpdateSessionConfig` / `doSessionJSON` / `doSessionPost` 做模式分发

**Gotchas**：
- Session 模式下受限实例 SSO 短时间内频繁登录会被临时封账号（运营层限制），代码已加 30s 登录限流
- `.json` 端点每个实体的**分页 URL 段格式不同**（bug/project/execution 互不一致），见各 `client_session_*.go` 的注释
- 数值字段用 `json.Number` + `UseNumber()` 解码（兼容 int/string 混用）
- 未认证返回 HTTP 200 + `data.locate` 指向 `user-login`（不是 401），`IsSessionAuthError` 专门识别

## API Endpoints

**读**（GET）：`/api/{products,projects,executions,bugs,builds/{project,execution},stories,tasks,users,users/all,users/current,cases,plans,programs,releases,testtasks,timelog/{analysis,dashboard,efforts},dashboard,project/overview,personal/timelog,search}`

**写**（POST/PUT/DELETE）：
- Bug: `/api/bugs`（POST）、`/api/bugs/:id`（PUT/DELETE）、`/api/bugs/:id/{resolve,close,assign,confirm,activate}`（POST）
- Task: `/api/tasks`（POST）、`/api/tasks/:id`（PUT/DELETE）、`/api/tasks/:id/{start,finish,pause,assign,activate,effort}`（POST）
- Story: `/api/stories`（POST）、`/api/stories/:id`（PUT/DELETE）、`/api/stories/:id/change`（POST）
- Plan: `/api/plans`（POST）、`/api/plans/:id`（PUT/DELETE）、`/api/plans/:id/{link-stories,unlink-stories,link-bugs,unlink-bugs}`（POST）
- Case: `/api/cases`（POST）、`/:id`（PUT/DELETE）
- Product/Project/Program/Execution/Build/User: `/api/{products,projects,programs,executions,builds,users}`（POST）、`/:id`（PUT/DELETE）

## Dependencies

- Go 1.25+（推荐 1.26），Node 24+
- Wails v3 CLI: `go install github.com/wailsapp/wails/v3/cmd/wails3@latest`
- `@wailsio/runtime` npm 包（已写入 package.json）
- golangci-lint
- Linux 完整构建需要: `libgtk-3-dev libwebkit2gtk-4.1-dev libayatana-appindicator3-dev pkg-config` 等
- macOS: Xcode Command Line Tools
- Windows: WebView2 Runtime（Win10/11 默认自带）

## Data files

- `~/.zentao-max/cron.db` — JSON 存储定时任务和执行日志
- `~/.zentao-max/cache.db` — 业务层 SQLite 离线缓存（service.CacheService 用），可安全删除
- `~/.zentao-max/cache-session.db` — Session 模式 client 层 SQLite 缓存（SessionCache L2 用），可安全删除

## SessionCache / SessionWorkerPool（Session 模式专属架构）

Session 模式（cookie 鉴权、禁用 REST API 的禅道实例，如受限实例 your-zentao-server.com）下，
单端点拉取成本高（200ms~2s），且受限实例 SSO 限流严格（30s 登录限频 + 高频并发
触发反爬）。client 层引入两个组件解决：

### SessionCache（`backend/core/zentao/session_cache.go`）

二级 cache，所有 key 带 `(server, account, realm)` 命名空间：

- **L1**：in-memory，固定容量 1000，FIFO 简化淘汰（数量级 ~ 几十到几百，不需要完整 LRU）
- **L2**：SQLite 持久化（自管表 `session_cache`，复用 `modernc.org/sqlite` 无 CGO），
  key 完整 `(ns, entity, scope)` 三元组
- **singleflight**：同 key 并发请求只触发一次 loader，防击穿
- **per-entity TTL**（`entityTTL` map）：
  - users / products / projects / programs / releases: 30 min fresh / 24 h stale
  - executions / builds / plans: 10 min fresh / 6 h stale
  - bugs / stories / tasks: 2 min fresh / 1 h stale
  - cases / testtasks / efforts / dashboard: 5 min fresh / 1 h stale
- **写后失效**：`Invalidate / InvalidateEntity / InvalidateAll` 接口；
  `doSessionPost` 成功后按 `writeOpToEntity` 映射调 `InvalidateEntity`
- **nil-safe**：`server` 或 `account` 为空时返回 nil，调用方 nil 检查后走直连

**为什么不复用 `service.CacheService`**：那个在业务层（VO 序列化后），
per `(entityType, productID)`；这个在 client 层（原始端点响应），per `(entity, scope)`。
两层职责不同，都存在。

**为什么 L2 自管表不复用 `storage.Store`**：storage.Store 的 key 是
`(entityType, productID)` 复合，把 scope 编进 productID 会冲突，编进
entityType 又会让所有 scope 共享一行 → 语义错。

### SessionWorkerPool（`backend/core/zentao/session_pool.go`）

请求调度中心，`doSessionJSON / doSessionPost` 必经之路：

- **限流**：全局共享 token bucket（`nextAllowedUnixNano atomic.CAS`），
  受限实例默认 8/s，开源禅道 20/s；0 = 无限流
- **写串行**：写 worker 永远 1 个（避免受限实例 SSO 把写端点标记异常）
- **读并发 4**：单 cookie 下受限实例 4 并发实测不触发限流
- **降级**：pool 满 / 100ms 排队超时 → 自动走 `cacheFallback`（stale 也算）；
  cache 也失败 → `ErrPoolBusy`
- **预热**：`SubmitPrefetch` 优先级 1，非阻塞入队，队列满即丢
- **优雅停机**：`Shutdown` 关闭队列 + 等所有 worker 退出 / ctx 取消
- **nil-safe**：poolConfig 为 nil 时不启用，doSessionJSON/Post 直连

### 集成（commit: e6f8602 / 8a7bed1 / d10f98f / be4993d）

- `NewSessionClient(server, account, password, realm, cacheDBPrefix, poolConfig)` 6 参数构造
- 切账号：`UpdateSessionConfig` → `cache.UpdateNamespace`（L1 自动清，L2 留作快速回切）
- 上层 service 调用 `c.client.GetAllBugsContext` 等已自动走 cache + pool，
  不需要散写 goroutine 改造

### 已知未完成（待后续 commit）

- prometheus metrics 注入：SessionCache / SessionWorkerPool 已留 `SetMetricsCallbacks`
  注入点但尚未在 metrics 包实现具体 adapter。`Client.AttachMetrics`
  也已存在接口签名，需要 `metrics.SessionMetricsAdapter` 实现。
  详见 backend/core/metrics/metrics.go 后续 patch。

## Known Issues / Migration Notes

- v3 alpha 阶段 API 偶有变化，升级版本前先 `wails3 doctor` 检查
- `WebviewWindowOptions.Hidden:true` 在 Windows 上有 bug ([#4498](https://github.com/wailsapp/wails/issues/4498))
- 全局快捷键在某些 Linux 桌面环境（如 Wayland）可能不工作
- 若 wails3 generate 失败，尝试 `wails3 generate bindings -clean -d frontend/bindings`



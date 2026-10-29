# AGENTS.md — zentao-max

## Project Overview

**纯 Web 应用**（Go Hertz + Vue 3 单二进制：HTTP server + 嵌入式前端）for Zentao (禅道) project management。Wails 桌面模式已于 v0.4.0 移除（浏览器访问 `:port` 即用）。

**Go module name is `github.com/yi-nology/zentao-max`**。业务代码在 `backend/` 下，import
路径形如 `github.com/yi-nology/zentao-max/backend/internal/...`（2026-10 布局重构：
原 `backend/core/*` 平铺 20 包已迁至 `backend/internal/` + `backend/pkg/`，**旧路径全部失效**）。

## Architecture

```
backend/
  main.go                 唯一服务入口（-config/-env/-static；默认嵌入静态，
                          -static <dir> 磁盘伺服，对齐 octopus 根 main 布局）
  cmd/mcp/main.go         MCP stdio 独立入口（AI 工具子进程二进制）
  biz/                    hz 生成层（handler/model/router，由 idl/ 生成，不要手改生成的骨架）
  idl/                    hz IDL 定义
  internal/               私有业务代码（对齐 octopus 布局）
    svc/                  装配 + HTTPApp 生命周期（手写装配，对齐 octopus svc 模式；
                          assembly.go，google/wire 依赖已移除）
    config/               oekit core/config 加载（默认值→yaml→env 覆盖，ZENTAO_MAX_ 前缀）
    routes/               中间件栈与路由注册
    middleware/           业务指标中间件（accesslog/recovery/hmw/prometheus 由 oekit 接管）
    handlers/             HTTP handlers（biz 生成层的实现目标）+ HandlerRegistry
    service/              Business logic（cache_service / dashboard_service）
    initialization/       启动装配、auth.db 凭据存取、ResolveAuthDBPath 路径解析
    auth/                 平台访问控制（匿名只读/登录读写、SSO、token 签发校验）
    mcp/                  MCP 子系统（24 工具定义、JSON-RPC 2026-07-28、热重载、stdio；
                          WithConfigDeps 注入 initService/SSO——HTTP 与 stdio 两入口都要接）
    repository/sqlite/    业务层 SQLite 离线缓存（modernc.org/sqlite, pure Go）
    infra/zentao/         禅道 client/SDK（Token+Session 双模式、熔断、池化、二级缓存）
    models/ dto/ vo/ event/ version/
  pkg/                    可复用基础库
    logger/               oekit logx（slog 后端）+ ring_buffer（日志页 /api/logs 数据源）
    errors/ constants/ metrics/ utils/
frontend/
  src/                    Vue 3 + Element Plus + Chart.js
    views/                Dashboard/Bugs/Stories/Tasks/Cases/Timelog/EffortCalendar/
                          Scheduler/HealthCheck/Logs/MCPGuide/Settings/Login/Layout
    composables/          useTableColumns / useTheme / useDesktopNotification（Web 通知）/
                          useExternalLink
    utils/export.ts       Excel/CSV/PDF exporter
  tests/ui/               Playwright UI 测试套件（node tests/ui/run-all.js）
docs/grafana/             Grafana dashboard JSON + 指标说明
skills/                   zentao-bug / zentao-timelog Agent 技能包
```

运行形态由 `-static` 决定（单入口 `cd backend && go run .`）：
- 默认 embedded：前端产物嵌入单二进制（CI 构建 `CGO_ENABLED=0 go build ./backend`）
- `-static <dir>`：磁盘伺服前端（本地联调）

## Commands

### Development
```bash
make run                          # 根 Makefile：cd backend && go run .
cd frontend && npm run dev        # vite dev server（/api 代理到后端）
cd backend && go run . -static ../frontend/dist   # 磁盘伺服模式联调
```

### Build
```bash
make release                      # frontend build + copy-static + trimpath 单二进制
make backend-build                # 仅后端：backend/zentao-max
cd backend && make build-mcp      # MCP stdio 入口：backend/zentao-max-mcp
./build-linux.sh [amd64|arm64]    # Linux 交叉编译（CGO_ENABLED=0，同 CI）
./build-windows-x64.sh            # Windows 交叉编译
```

### Test & Lint
```bash
cd backend && make check              # fmt + lint + test
cd backend && make test               # go test
cd backend && make test-coverage-html # HTML 覆盖率报告
cd frontend && npm run type-check     # vue-tsc --noEmit
cd frontend && npm run build          # vue-tsc + vite build
cd frontend && node tests/ui/run-all.js   # Playwright UI 套件（需 npm i --no-save playwright）
```

## Key Conventions & Gotchas

- **装配**：`backend/internal/svc/assembly.go` 是**手写**装配——凭据解析
  （ldflags/config/env → auth.db 回退）→ 禅道 client → Dependencies → HTTPApp。
  google/wire 已移除，改依赖关系直接编辑该函数
- **hz 生成层**：`backend/biz/` 由 `idl/` 生成；业务实现在 `internal/handlers/`，
  biz handler 只是转发。不要手改 biz 生成骨架
- **Static assets**: `scripts/copy-static.sh` 复制 `frontend/dist/` → `backend/static/`。
  ⚠️ 脚本是普通 `cp -r` 非 mirror——先 `rm -rf backend/static` 再跑，否则已删除页面的
  旧 chunk 会残留在嵌入产物里
- **Config priority**: 设置页热更新 / auth.db > env（凭据走 `ZENTAO_SERVER/ACCOUNT/PASSWORD`
  及 `ZENTAO_MAX_` 前缀双形式）> config.yaml > 内置默认值
- **Default HTTP port**: `12345`（`server.port`）
- **前端 env**：本地 `.env` 需 `VITE_API_BASE_URL=/api`（vite 代理同源 cookie）；
  `.env.wails` 为历史遗留，无脚本引用
- **CI**: push `main` 或 `v*.*.*` tag 触发「Build zentao-max」——npm build → copy-static →
  `CGO_ENABLED=0` 交叉编译 4 目标（linux amd64/arm64 / windows amd64 / darwin arm64）
- **端口占用排查**：杀父进程不杀子进程会造成"看起来重启了实际还是旧进程"——
  `kill -9 $(lsof -ti :12345)` 后确认启动日志时间戳再验证

## Platform Auth (平台访问控制, 自 zentao-mini 同步)

`backend/internal/auth`：匿名只读（GET/HEAD/OPTIONS 放行），登录管理员读写。

- **SSO 登录**：`/api/auth/sso/send-code` + `/api/auth/sso/login`——统一认证平台
  （地址全部来自部署配置，代码不内置默认值）用户名+密码+短信验证码：密码用登录页 RSA 公钥
  加密（PKCS#1 v1.5），POST /mobiles 拿脱敏手机号 → /verifyCode 触发短信 → /sso/doLogin
  验证码登录（移植自 freedom/ky-work-mcp 的实测实现）。成功后签发平台会话令牌（与本地登录
  同一会话体系）。配置：`auth.sso.enabled`（默认 false）+ `auth.sso.base_url`（启用时必填，
  未配置则 SSO 入口自动关闭并启动告警）+ `auth.sso.auth_page_url`（Referer，留空回退
  `{base_url}/sso/auth`）。`GET /api/auth/sso/status` 恒注册（匿名 GET），返回 `{enabled}`
  供登录页选择默认表单。
- **本地管理员（备用入口）**：`auth.admin.username/password`（默认 admin/admin，env
  `ZENTAO_MAX_AUTH_ADMIN_USERNAME/PASSWORD`）。登录页底部链接切换两种方式。
- **会话**: HMAC 无状态 token，格式 `exp:username.签名`（分隔符 `:` 为 cookie 合法字符，
  不会被浏览器 percent-encode；verify 时归一化历史 `%7C` 残留）——令牌内记录登录者身份
  （SSO 账号或本地管理员名），`/api/auth/status` 返回该身份；Cookie `zentao_max_session`
  + `Authorization: Bearer` 双通道；`SecretFromEncryptionKey` 派生签名密钥，重启后有效。
  登录失败防爆破（10 次/分钟/IP，SSO 两步共用计数）。
- **豁免**: `/mcp*`（MCP 有独立 token/只读控制）；`/api/auth/*`（登录/登出/SSO 两步）；
  `/api/scheduler/preview`（纯读）。
- **前端**: `/login` 页按 `GET /api/auth/sso/status` 选择默认表单（SSO 启用=账号/密码/验证码
  +60s 倒计时重发；否则本地管理员），底部可手动切换；
  头部不展示模式徽章——登录后侧边栏底部显示用户名+退出，未登录只显示登录链接（模式守卫由
  后端 401 层承担）；`api.ts` 对 401 分流——`/auth/*` 自身 401 表单自展示，含"管理员登录"
  字样跳 `/login?redirect=...`，其余（禅道配置失效）仅弹错误提示不跳转。
- **限流**: oekit KeyedLimiter + TokenBucket（按 IP 直连地址 key，防 XFF 伪造；
  平滑令牌桶，超限即拒 429 无封禁期；默认 600 次/分钟）。
  `BlockDurationMinutes` 字段为 yaml 兼容保留但不参与。基础设施路径（/health、
  /api/healthz、/metrics、/api/version）豁免。
- **MCP 侧配置/认证工具**：`update_zentao_config`（TCP 5s 预检 → 同步登录验证 →
  StoreAuthConfigFromStruct 持久化 → ClearAllCaches 清业务缓存，与设置页同语义）、
  `send_verify_code`/`sso_login`（麒麟 SSO 短信两步，依赖 `auth.sso.enabled`；
  send 有 **60s 冷却窗**——SSO 平台频繁发送会频控/临时封号，防 AI 循环重试）、
  `get_zentao_config`/`test_connection`（只读诊断）。前三个归入 writeTools——只读令牌
  不得重配连接或触发认证。
  auth.db 解密失败（启动日志 `invalid character ...`）后禅道配置为空的场景，
  现在可由 AI 直接调 `update_zentao_config` 自愈，无需手工重新生成文件。
- **MCP 列表瘦身**：`get_bugs/get_stories/get_tasks/get_effort_calendar` 的长文本字段
  （steps/spec/verify/desc/work）经 `handlers_slim.go` 截断到 200 rune 并带截断标记
  （100 条 Bug 的 steps 全文数十 KB，对 AI 是纯 token 浪费）；全文用 `get_*_detail`。
  前端不走 MCP，无 UI 影响。

## Skills (Agent 技能包, `skills/`)

仓库内置两个可分发的 Agent 技能（直连禅道实例的 session 方式，支持 realm 统一认证域，
凭证走 `~/.zentao-credentials.env`，模板见 `skills/config.example.env`；配置含 SSH 跳板机
隧道支持）。改这两个技能时同步更新其 SKILL.md：

- **`skills/zentao-timelog/`**：Excel 批量填报禅道工时/任务日志（`scripts/zt.py`：whoami/tasks/plan/submit/verify）。核心红线：先 plan 再 submit、`left[1]` 必发且为绝对值、AJAX 外壳 `{"status":"success"}` 必须穿透 `data` 判定且写后必回读、失败行不入台账可安全重跑、工时被外部系统同步后只能网页改。
- **`skills/zentao-bug/`**：BUG 查询（`get_bug.py`，`bug-view-{id}.json` 外层 data 需二次解析）、解决/关闭/编辑（`close_bug.ps1`）与质量分析五字段批量填写（`set_quality_fields.ps1`：fixedPackageVersion + reasonAnalysis/advice/implementation/testAdvise 一次 `bug-edit` 全量写全，必须带 openedBuild/execution/title 原值）。

与后端 API 的关系：Bug 流转、批量工时（`POST /api/v1/efforts/batch`）等能力应用本身已提供
REST/MCP 入口；技能包覆盖的是应用 API 未暴露的实例侧细节（质量分析字段、Excel 工作流、
biz8.x 全量更新语义、跳板机隧道）。

**应用页面已吸收的技能能力**（页面内即可用，无需 CLI）：

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

## Authentication Modes (禅道双认证模式)

应用支持两种禅道认证与数据访问模式，按配置的 `realm` 字段自动切换：

| 模式 | 认证 | 数据访问 | 适用 |
|------|------|----------|------|
| **Token 模式**（默认） | 用户名/密码换 REST Token | `/api.php/v1/*` REST 端点（Token 头） | 标准禅道开源版/企业版 |
| **Session 模式** | `/user-login.html` PHP 会话 | `*.json` 端点（zentaosid cookie） | 禁用 REST API 的禅道实例（如受限实例 biz8.x） |

**配置方式**：
- 禅道连接凭据存于 auth.db。实际路径由 `initialization.ResolveAuthDBPath` 统一解析
  （`backend/internal/initialization/auth_path.go`），优先级：显式配置（`auth.db_path` /
  入口 ldflags 变量）→ 环境变量 `ZENTAO_AUTH_DB_PATH` → 已存在候选（`./data/auth.db` 规范
  位置 → `./auth.db` 历史 → `~/.zentao-max/auth.db` 最早）→ 都没有则首次写入落
  `./data/auth.db`（`cd backend && go run .` 时即 `backend/data/auth.db`）。
  `AuthConfig.Realm` 非空（如 `"kydc"`）→ Session 模式
- ⚠️ auth.db 加密密钥优先级：显式 `ZENTAO_ENCRYPTION_KEY`（生产推荐，跨机器搬迁
  auth.db 前先固定它）→ **托管密钥文件** `<auth.db 同目录>/encryption.key`
  （0600，首次生成后稳定）。历史方案按 hostname+homeDir 派生机器密钥——macOS 的
  kern hostname 会随网络/VPN 漂移，是 auth.db 反复解密失败（日志
  `invalid character ...`，症状为禅道接口全 50002「禅道服务器地址为空」）的根因，
  已弃用；现仅作 legacy 回退解密（命中时日志提示 re-save 迁移）。解密仍失败时
  重新生成 auth.db（`initialization.InitService.StoreAuthConfigFromStruct`）或
  由 AI 调 MCP `update_zentao_config` 自愈。注意 quirk：密钥派生里 salt 因 32 字节
  截断实际不参与有效密钥（历史 shell 兼容遗留，修复会破坏现有文件）
- init 引导页已删除（v0.4.0 起），凭据只经 auth.db / 环境变量 / 设置页「连接配置」卡片
  （`POST /api/v1/zentao-config`，平台登录后可用，保存成功自动清空业务缓存）进入系统；
  路径解析日志见启动输出 `Auth DB path: ...`
- 前端禅道登录表单选择认证域：`kydc`（受限实例 SSO，密码明文）/ `local`（禅道本地，md5 混淆）
- 环境变量：`ZENTAO_MAX_REALM` / `ZENTAO_REALM`

**实现关键文件**（`backend/internal/infra/zentao/`）：
- `client_auth_mode.go` — `AuthMode` / `RealmKySSO` 常量
- `client_session.go` — `SessionTransport`（登录、cookie jar、双层 JSON 解包、re-login 重试、熔断）
- `client_session_*.go` — 各实体的 `.json → SDK` 映射（bugs/stories/tasks/products/projects/executions/users/builds/efforts/cases/plans/programs/releases/testtasks）
- `client_writes*.go` — 写操作（bug/task/story/... 的 create/update/delete/resolve/close/start/finish 等），token + session 双模式
- `client.go` 的 `NewSessionClient` / `UpdateSessionConfig` / `doSessionJSON` / `doSessionPost` 做模式分发

**Gotchas**：
- Session 模式下受限实例 SSO 短时间内频繁登录会被临时封账号（运营层限制），代码已加 30s 登录限流
- `.json` 端点每个实体的**分页 URL 段格式不同**（bug/project/execution 互不一致），见各 `client_session_*.go` 的注释
- 数值字段用 `json.Number` + `UseNumber()` 解码（兼容 int/string 混用）
- 未认证返回 HTTP 200 + `data.locate` 指向 `user-login`（不是 401），`IsSessionAuthError` 专门识别
- **分页 pages 2+ 必须绕过 SessionCache**（`doSessionJSONUncached`）——同 scope 命中
  同一缓存行会导致翻页拿重复数据、统计口径虚高

## API Endpoints

**读**（GET）：`/api/{products,projects,executions,bugs,builds/{project,execution},stories,tasks,users,users/all,users/current,cases,plans,programs,releases,testtasks,timelog/{analysis,dashboard,efforts},dashboard,project/overview,personal/timelog,search,healthz,logs,version}`

**写**（POST/PUT/DELETE）：
- Bug: `/api/bugs`（POST）、`/api/bugs/:id`（PUT/DELETE）、`/api/bugs/:id/{resolve,close,assign,confirm,activate}`（POST）。
  （Bug 评论无独立 REST 端点，走 MCP `add_bug_comment` 工具，内部复用 bug-edit 全量更新通道）
- Task: `/api/tasks`（POST）、`/api/tasks/:id`（PUT/DELETE）、`/api/tasks/:id/{start,finish,pause,assign,activate,effort}`（POST）
- Story: `/api/stories`（POST）、`/api/stories/:id`（PUT/DELETE）、`/api/stories/:id/change`（POST）
- Plan: `/api/plans`（POST）、`/api/plans/:id`（PUT/DELETE）、`/api/plans/:id/{link-stories,unlink-stories,link-bugs,unlink-bugs}`（POST）
- Case: `/api/cases`（POST）、`/:id`（PUT/DELETE）
- Product/Project/Program/Execution/Build/User: `/api/{products,projects,programs,executions,builds,users}`（POST）、`/:id`（PUT/DELETE）
- 工时: `POST /api/v1/efforts/batch`（批量写日志；前端路径必须 `/v1/efforts/batch`，写错 404）
- 连接配置: `POST /api/v1/zentao-config`（平台登录后热更新禅道地址/账号，成功自动清业务缓存）
- MCP 管理: `GET/PUT /api/v1/mcp/{status,config}`（运行时热重载）
- 平台认证: `/api/auth/{login,logout,status}` + `/api/auth/sso/{send-code,login}`

## Dependencies

- Go 1.25+（go.mod 要求 1.26.0，`GOTOOLCHAIN=auto` 自动下载）
- Node 24+（前端构建）
- golangci-lint（后端 lint，可选）
- 纯 Go 依赖（hertz + modernc.org/sqlite 无 CGO），交叉编译无需任何平台系统库

## Data files

- `./data/auth.db` — 禅道连接凭据（AES 加密；路径经 `ResolveAuthDBPath` 解析，见上）
- `./data/encryption.key` — 托管加密密钥（0600，无显式 ZENTAO_ENCRYPTION_KEY 时自动生成；
  删除后下次生成新密钥，旧 auth.db 需重新配置；与 auth.db 同目录随其备份/搬迁）
- `./data/cron.db` — 定时任务与执行日志（JSON 存储；进程工作目录 `data/` 下）
- `~/.zentao-max/cache.db` — 业务层 SQLite 离线缓存（service.CacheService 用），可安全删除
- `./cache-session.db` — Session 模式 client 层 SQLite 缓存（SessionCache L2 用，
  进程工作目录），可安全删除

## SessionCache / SessionWorkerPool（Session 模式专属架构）

Session 模式（cookie 鉴权、禁用 REST API 的禅道实例）下，
单端点拉取成本高（200ms~2s），且受限实例 SSO 限流严格（30s 登录限频 + 高频并发
触发反爬）。client 层引入两个组件解决：

### SessionCache（`backend/internal/infra/zentao/session_cache.go`）

二级 cache，所有 key 带 `(server, account, realm)` 命名空间：

- **L1**：in-memory，固定容量 1000，FIFO 简化淘汰（数量级 ~ 几十到几百，不需要完整 LRU）
- **L2**：SQLite 持久化（自管表 `session_cache`，复用 `modernc.org/sqlite` 无 CGO），
  key 完整 `(ns, entity, scope)` 三元组
- **singleflight**：同 key 并发请求只触发一次 loader，防击穿
  （oekit `concurrency/singleflight` 泛型实现）
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

**为什么 L2 自管表不复用 `repository/sqlite`**：storage.Store 的 key 是
`(entityType, productID)` 复合，把 scope 编进 productID 会冲突，编进
entityType 又会让所有 scope 共享一行 → 语义错。

### SessionWorkerPool（`backend/internal/infra/zentao/session_pool.go`）

请求调度中心，`doSessionJSON / doSessionPost` 必经之路：

- **限流**：全局共享令牌桶（oekit `concurrency/ratelimit.TokenBucket.Wait`，
  ctx 取消不消耗配额），受限实例默认 8/s，开源禅道 20/s；0 = 无限流
- **写串行**：写 worker 永远 1 个（避免受限实例 SSO 把写端点标记异常）
- **读并发 4**：单 cookie 下受限实例 4 并发实测不触发限流
- **熔断**：`SessionTransport` 出站统一走 oekit breaker——仅网络层错误计数
  （连拒/DNS/超时；拿到任何 HTTP 响应包括禅道 4xx 均视为可达），连续 5 次网络失败
  open 30s，拒绝错误带可读中文并保留 `errors.Is(err, breaker.ErrCircuitOpen)`
- **降级**：pool 满 / 100ms 排队超时 → 自动走 `cacheFallback`（stale 也算）；
  cache 也失败 → `ErrPoolBusy`
- **预热**：`SubmitPrefetch` 优先级 1，非阻塞入队，队列满即丢
- **优雅停机**：`Shutdown` 关闭队列 + 等所有 worker 退出 / ctx 取消
- **nil-safe**：poolConfig 为 nil 时不启用，doSessionJSON/Post 直连

### 集成

- `NewSessionClient(server, account, password, realm, cacheDBPrefix, poolConfig)` 6 参数构造
- 切账号：`UpdateSessionConfig` → `cache.UpdateNamespace`（L1 自动清，L2 留作快速回切）
- 上层 service 调用 `c.client.GetAllBugsContext` 等已自动走 cache + pool，
  不需要散写 goroutine 改造

## oekit 组件覆盖（2026-10 迁移完成）

已深度接入：logx（slog）、core/config、bootstrap（生命周期）、concurrency
（async.GoSafe / singleflight / breaker / ratelimit / group / fanout）、middleware
（hmw trace / recovery / accesslog / prometheus）、pkg（slicex / httpjson）、
tracing/traceid、gzip（hertz-contrib）。

保留手写的部分（均有记录的评估结论）：禅道协议状态机（sid 过期→relogin→replay）、
SSO 节流规避、SessionWorkerPool 三队列设计、业务错误码体系、业务 metrics、
robfig/cron（多任务同表达式）、分页 start/end 契约（禅道 API 外部约定）。

## Known Issues / 限制

- 需求（stories）分页 pages 2+ 在受限 biz8.x 实例返回空数组（browseType 语义未摸清，
  当前取第 1 页全量；修复需有效 web 会话抓取权威分页格式）
- 大实例上 /api/tasks 与 timelog 跨执行遍历慢（禅道无产品级任务端点）：遍历循环已有
  60s 截止门控返回部分数据，dashboard 聚合 90s 总超时；首查仍慢
- auth.db 解密失败仅剩两种情形：`encryption.key` 被删（重新保存配置即可恢复）与
  跨机器搬迁未带密钥文件（连同 `encryption.key` 一起拷贝，或固定 `ZENTAO_ENCRYPTION_KEY`）；
  历史 hostname 漂移致败的问题已由托管密钥文件根治（legacy 回退兼容旧文件）
- `go:embed static/*` 要求嵌入构建前先跑 copy-static，且先清空 `backend/static`

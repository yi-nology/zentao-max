# zentao-max

禅道（ZenTao）项目管理系统辅助工具。**纯 Web 应用**：Go（Hertz）+ Vue 3 编译为**单一二进制**（HTTP 服务 + 内嵌前端），浏览器访问即用。Wails 桌面模式已于 v0.4.0 移除。

官网: https://murphyyi.com

## 功能特性

- **数据看板**：Bug/任务/需求汇总卡片与图表；大实例聚合设 90s 总超时门控，超时返回已收集的部分数据
- **Bug 管理**：查询/筛选/导出 Excel、详情查看、编辑与质量分析五字段批量填写、流转（confirm/resolve/close/activate/assign）、评论、Bug 详情页直接「记录工时」
- **需求 / 任务 / 用例 / 版本计划**查询与详情；任务详情「记录工时」
- **工时日历**（EffortCalendar）：日历视图 + 批量写日志（手工 + Excel 导入）+ 日备注「同步为工时」挂任务——共 4 个写日志入口
- **工时统计**：分析 / 看板 / 明细流水
- **定时任务**：cron 调度 + webhook 通知（钉钉/企业微信）
- **健康检查**：`/health-check` 页面 + `/api/healthz` 实时探针（禅道连通性自愈）
- **日志页**：环形缓冲实时日志（含 trace_id）
- **MCP 服务**：24 个工具（16 查询 + 8 写），JSON-RPC `2026-07-28` 协议，HTTP 与 stdio 双入口，运行时热重载；stdio 入口日志走 stderr 保证协议流纯净；支持 AI 自助完成连接配置（`update_zentao_config`）与登录排错（SSO 验证码两步）
- **平台认证**：匿名只读 / 登录读写；UI 登录统一走 SSO（麒麟统一认证，用户名+密码+短信验证码），本地管理员仅保留 API 入口（脚本/自动化）
- **版本检查与更新**：启动注册检查（携带 OS/arch）+ 定期复查 + 设置页一键下载/应用/重启（单二进制自更新，Windows 下载后手动替换）
- **系统服务安装**：设置页一键注册开机自启（macOS launchd / Linux systemd user / Windows 计划任务，全部用户级免管理员），环境变量快照固化进服务定义
- **可观测性**：Prometheus `/metrics` + Grafana 仪表盘（`docs/grafana/`）
- **Agent 技能包**：`skills/` 内置 zentao-bug / zentao-timelog 两个可分发技能（直连禅道实例）

## 系统要求

- Go 1.25+（go.mod 要求 1.26.0，`GOTOOLCHAIN=auto` 默认自动下载）
- Node.js 24+（仅构建前端时需要；使用发布产物则无需）
- 可选：golangci-lint（后端 lint）

## 快速开始

### 开发运行

```bash
# 1. 前端构建产物（嵌入后端；开发时可改用 vite dev + 后端 -static 模式）
cd frontend && npm install && npm run build
cd .. && ./scripts/copy-static.sh

# 2. 启动后端（单入口，默认读取 backend/ 下 config.yaml，无则用内置默认值）
cd backend && go run .        # 或 make run
# 浏览器访问 http://localhost:12345
```

前端热更新联调：

```bash
cd frontend && npm run dev    # vite dev server，/api 代理到后端
cd backend && go run . -static ../frontend/dist
```

**首次配置**：部署时通过 `auth.db` 加密文件或环境变量提供禅道凭据（已无初始化引导页）；
运行后在「设置」页的「连接配置」卡片可热更新禅道地址/账号（需平台登录）。

### 发布构建

```bash
make release                  # 前端 build + copy-static + 单二进制 backend/zentao-max
PORT=8080 ./zentao-max        # 运行（默认端口 12345）
```

交叉编译（纯 Go + modernc sqlite，无 CGO，与 CI 同流程）：

```bash
./build-linux.sh [amd64|arm64]   # Linux
./build-windows-x64.sh           # Windows
# macOS arm64: cd backend && CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o ../zentao-max .
```

### CI / 发布

GitHub Actions「Build zentao-max」：push `main` 或 `v*.*.*` tag 触发，单 ubuntu runner
构建 4 个目标——linux-amd64 / linux-arm64 / windows-amd64 / darwin-arm64（macOS 仅 arm64），
tag 触发时产物附加到对应 GitHub Release。

## 配置

加载链：**内置默认值 → config.yaml → 环境变量**（`ZENTAO_MAX_` 前缀，`server.port` →
`ZENTAO_MAX_SERVER_PORT`）。模板见 `backend/config.yaml.example`。

| 配置块 | 关键项 | 说明 |
|--------|--------|------|
| `server` | `port`（默认 `12345`）、`static_path` | static_path 仅 `-static` 模式使用 |
| `zentao` | `server/account/password/realm` | 凭据优先级：设置页热更新 / auth.db > env > config.yaml |
| `auth` | `admin.username/password`、`sso.enabled/base_url` | 平台登录；admin 默认 admin/admin |
| `auth` | `db_path` | auth.db 路径，默认由 `ResolveAuthDBPath` 统一解析 |
| `security` | `encryption_key` | auth.db 加密密钥；生产固定 `ZENTAO_ENCRYPTION_KEY`，否则自动使用托管密钥文件（`encryption.key`，首次生成后稳定） |
| `rate_limit` | `requests_per_minute` | oekit 令牌桶，按 IP 直连地址限流 |
| `mcp` | `enabled/transport/token/read_only/actions` | MCP 子系统，支持运行时热重载 |
| `log` | `level/format/output_path` | output_path 支持 `stderr`；格式 json/console |

**auth.db 解析**（`backend/internal/initialization/auth_path.go`）：显式配置（`auth.db_path` /
ldflags）→ 环境变量 `ZENTAO_AUTH_DB_PATH` → 已存在候选（`./data/auth.db` → `./auth.db` →
`~/.zentao-max/auth.db`）→ 默认落 `./data/auth.db`。启动日志输出 `Auth DB path: ...`。

**禅道认证模式**：`realm` 为空走 REST Token 模式（`/api.php/v1/*`）；`realm` 非空（如 `kydc`）
走 Session 模式（`*.json` 端点 + zentaosid cookie），适配禁用 REST API 的受限实例。

## MCP 接入

| 类别 | 工具 |
|------|------|
| 列表查询（9） | `ping` `get_products` `get_projects` `get_executions` `get_bugs` `get_stories` `get_tasks` `get_users` `get_timelog` |
| 详情/个人查询（5） | `get_bug_detail`（Bug 完整详情）`get_task_detail`（任务详情）`get_story_detail`（需求详情）`get_my_tasks`（我参与的任务）`get_effort_calendar`（指定用户全年工时明细） |
| 基础配置/认证（5） | `get_zentao_config`（查看连接配置）`update_zentao_config`（更新连接配置并持久化，首次接入用）`test_connection`（实时连通性探针）`send_verify_code` / `sso_login`（麒麟统一认证短信登录两步） |
| 写入（5） | `record_effort`（写工时日志）`add_bug_comment`（Bug 评论）`transition_bug`（Bug 流转）`create_bug`（创建 Bug，返回新 ID）`transition_task`（任务 start/finish/pause/assign） |

- **HTTP**：`POST http://host:12345/mcp/{action}`（简化协议）或 JSON-RPC 2.0 `tools/list` /
  `tools/call`（MCP 协议版本 `2026-07-28`）
- **stdio**：`cd backend && make build-mcp` → 产物 `backend/zentao-max-mcp`，供 AI 工具作为子进程启动
- `mcp.read_only: true` 时 8 个受控工具被拦截（5 个数据写 + 配置变更/认证操作）；
  Token 鉴权与 enabled 开关可经 `/api/v1/mcp/config` 热重载（无需重启）

## 目录结构

```
backend/
  main.go                 唯一服务入口（embedded 默认 / -static <dir> 磁盘伺服）
  cmd/mcp/                MCP stdio 独立入口（AI 工具子进程）
  biz/                    hz 生成层（handler/model/router，由 idl/ 生成）
  internal/               私有业务代码（对齐 octopus 布局）
    svc/                  装配 + HTTPApp 生命周期（手写装配，原 wire 已去化）
    routes/ middleware/   中间件栈（oekit accesslog/recovery/hmw/prometheus + gzip）与路由
    handlers/ service/    HTTP handlers 与业务逻辑（dashboard/cache）
    initialization/       启动装配、auth.db 凭据、auth.db 路径解析
    auth/                 平台访问控制（匿名只读/登录读写、SSO、token 签发）
    mcp/                  MCP 子系统（24 工具、JSON-RPC、热重载、stdio）
    infra/zentao/         禅道 client/SDK（Token+Session 双模式、熔断、池化、二级缓存）
    repository/sqlite/    业务层 SQLite 离线缓存
    config/ dto/ vo/ models/ event/ version/
  pkg/                    logger(oekit logx/slog) errors constants metrics utils
  idl/                    hz IDL 定义
frontend/
  src/views/              Dashboard/Bugs/Stories/Tasks/Cases/Timelog/EffortCalendar/
                          Scheduler/HealthCheck/Logs/MCPGuide/Settings/Login/Layout
  src/composables/        useTableColumns / useTheme / useDesktopNotification（Web 通知）
  tests/ui/               Playwright UI 测试套件（node tests/ui/run-all.js）
skills/                   zentao-bug / zentao-timelog Agent 技能包
docs/                     ARCHITECTURE.md / 使用说明.md / grafana/
scripts/copy-static.sh    frontend/dist → backend/static（嵌入前必跑；先清空目标目录）
```

## 数据文件

| 路径 | 用途 | 可否删除 |
|------|------|----------|
| `./data/auth.db` | 禅道连接凭据（AES 加密，密钥见 `security.encryption_key`） | 删除后需重新提供凭据 |
| `./data/encryption.key` | auth.db 托管加密密钥（0600，自动生成；与 auth.db 一起备份/搬迁） | 删除后旧 auth.db 需重新配置 |
| `./data/cron.db` | 定时任务与执行日志（进程工作目录 `data/` 下） | 删除丢定时任务 |
| `~/.zentao-max/cache.db` | 业务层 SQLite 离线缓存 | 可安全删除 |
| `./cache-session.db` | client 层 SessionCache L2（进程工作目录） | 可安全删除 |

## 已知限制

- **需求分页**：受限实例（biz8.x）`product-browse` 的 story 分页 URL 语义未完全摸清，
  pages 2+ 返回空数组——当前取第 1 页（每页上限内全量），修复需有效 web 会话抓包确认分页格式
- **大实例遍历耗时**：任务/工时跨全部执行遍历（禅道无产品级任务端点），已加 60s/90s
  截止门控，超时返回部分数据；首查（缓存过期）仍需数十秒
- **受限实例 SSO 限流**：短时间频繁登录会被临时封账号（代码已加 30s 登录限频）
- **auth.db 解密失败**：历史版本按机器 hostname 派生密钥，hostname 随网络/VPN 漂移导致
  反复失效——已改为托管密钥文件（`encryption.key`，首次生成后稳定），老文件自动走
  legacy 回退解密；仍失败（密钥文件也丢了）时重新保存配置即可（设置页或 AI 调
  `update_zentao_config`）

## 文档

- [AGENTS.md](AGENTS.md) — 开发约定、命令、踩坑（agent/新人入口）
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — 架构与请求链路
- [docs/使用说明.md](docs/使用说明.md) — 用户手册
- [docs/grafana/README.md](docs/grafana/README.md) — Prometheus 指标与仪表盘导入
- [skills/README.md](skills/README.md) — Agent 技能包使用

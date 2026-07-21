# zentao-max Grafana 仪表盘

本目录提供 zentao-max 的 Grafana 仪表盘 JSON 示例，配合应用内置的 Prometheus `/metrics` 端点使用。

## 仪表盘包含的指标

- **HTTP 概览**：QPS、P99 延迟、错误率（5xx 占比）、并发请求数
- **HTTP 详细**：按路径 Top10、按状态码分布
- **缓存**：按 cache_type 的命中率、操作延迟 P95
- **禅道 API**：调用速率、错误数、Token 刷新次数
- **业务实体**：活跃 Bug、活跃任务、累计工时

## 指标清单

应用通过 `/metrics` 暴露的指标（来自 `backend/core/metrics/metrics.go`）：

| 指标名 | 类型 | 标签 | 说明 |
|--------|------|------|------|
| `http_requests_total` | Counter | method, path, status | HTTP 请求总数 |
| `http_request_duration_seconds` | Histogram | method, path | HTTP 请求耗时 |
| `http_requests_in_flight` | Gauge | method | 正在处理的请求数 |
| `cache_hits_total` | Counter | cache_type | 缓存命中次数 |
| `cache_misses_total` | Counter | cache_type | 缓存未命中次数 |
| `cache_operation_duration_seconds` | Histogram | cache_type, operation | 缓存操作耗时 |
| `zentao_api_requests_total` | Counter | endpoint, method | 禅道 API 调用数 |
| `zentao_api_duration_seconds` | Histogram | endpoint, method | 禅道 API 耗时 |
| `zentao_api_errors_total` | Counter | endpoint, method, error_type | 禅道 API 错误数 |
| `zentao_token_refreshes_total` | Counter | - | Token 刷新次数 |
| `bugs_total` | Gauge | product, project, status | Bug 数量 |
| `stories_total` | Gauge | product, project, status | 需求数量 |
| `tasks_total` | Gauge | project, execution, status | 任务数量 |
| `timelog_hours_total` | Counter | user, project | 累计工时（小时） |

## 导入步骤

1. 启动 zentao-max 应用（任意模式），访问 `http://localhost:12345/metrics` 确认指标输出正常。
2. 配置 Prometheus 抓取该端点：
   ```yaml
   scrape_configs:
     - job_name: 'zentao-max'
       scrape_interval: 15s
       static_configs:
         - targets: ['localhost:12345']
   ```
3. 在 Grafana 中：
   - 进入 **Dashboards → New → Import**
   - 上传或粘贴 `zentao-max-dashboard.json` 内容
   - 选择 Prometheus 数据源
   - 点击 Import

## 模板变量

仪表盘定义了 `${DS_PROMETHEUS}` 数据源变量，导入时可在下拉框选择实际使用的 Prometheus 实例。

## 注意事项

- 部分业务指标（`bugs_total`、`tasks_total` 等）需要应用层主动调用 `metrics.UpdateBugsTotal()` 才会更新。目前应用未在所有路径调用，可能显示为空——后续会逐步完善。
- `cache_type` 当前包含 `bug`、`story`、`task`、`build`、`execution` 等命名空间。

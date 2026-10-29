# zentao-max 前端

Vue 3 + Vite + Element Plus + Chart.js 的禅道辅助工具前端。构建产物由后端单二进制嵌入伺服（或 `-static` 磁盘伺服）。

## 开发

```bash
npm install
npm run dev        # vite dev server；/api 代理到后端（需后端同时在跑）
```

本地 `.env` 需要 `VITE_API_BASE_URL=/api`（走 vite 代理，保证 cookie 同源）。

## 构建与检查

```bash
npm run type-check   # vue-tsc --noEmit
npm run build        # vue-tsc + vite build → dist/
```

构建后由仓库根目录的 `./scripts/copy-static.sh` 复制 `dist/` → `backend/static/` 嵌入后端。

## UI 测试

Playwright 套件（无独立依赖清单，需先安装）：

```bash
npm install --no-save playwright
node tests/ui/run-all.js    # 需要本地后端可达，多数写操作需平台登录
```

## 目录要点

- `src/views/` — 页面（Dashboard/Bugs/Stories/Tasks/Cases/Timelog/EffortCalendar/Scheduler/HealthCheck/Logs/MCPGuide/Settings/Login/Layout）
- `src/composables/` — useTableColumns / useTheme / useDesktopNotification（浏览器 Notification API + 轮询）/ useExternalLink / useAuth / useZentaoConfig
- `src/api/` — api.ts（axios 封装与 401 三分流）/ auth.ts / zentao.ts / scheduler.ts
- `src/utils/export.ts` — Excel/CSV/PDF 导出

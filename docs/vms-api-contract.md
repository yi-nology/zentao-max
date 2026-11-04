# VRMS（版本发布管理服务）API 契约 —— zentao-max 客户端视角

> 客户端已按本契约实现（`backend/internal/update`），服务端按此实现即可打通
> 「检查更新 → 弹窗提示 → 更新日志」闭环。任何非 200/网络失败客户端一律
> 静默降级（保留上次结果，不对外暴露错误），所以服务端上线/下线/升级期间
> 客户端无感。

## 基地址

```
https://api.murphyyi.com/vms/v1
```

- 客户端把该值当作 **API 前缀（含版本段）**，业务路径由客户端拼接。
- 可通过 `update.release_url` 配置覆盖（env `ZENTAO_MAX_UPDATE_RELEASE_URL`）。

## 1. 检查更新

```
GET {base}/products/{slug}/upgrade-check?current={version}&locale=zh-CN
```

- `slug`：客户端默认 `zentao-max`（`update.slug` 可配）。
- `current`：客户端当前版本，**不带 v 前缀**（如 `0.6.1`）。
- 请求头：`User-Agent: zentao-max/{version}`、`X-Client-Version: {version}`。

### 成功响应（200）

```json
{
  "has_update": true,
  "force_update": false,
  "latest": "0.6.2",
  "latest_status": "released",
  "note_md": "## 0.6.2 更新\n- 修复xxx",
  "released_at": "2026-11-04T12:00:00Z",
  "min_supported": "0.5.1",
  "channels": [
    {
      "key": "github",
      "name": "GitHub Releases",
      "url": "https://github.com/yi-nology/zentao-max/releases/tag/v0.6.2",
      "status": "published"
    }
  ]
}
```

### 字段语义（客户端行为）

| 字段 | 客户端行为 |
| --- | --- |
| `has_update` | **服务端权威**，客户端直接采信、不本地比版本 |
| `force_update` | true → 客户端弹不可关闭模态，唯一按钮跳 `channels[].url` |
| `min_supported` | 低于它的客户端会收到 `force_update=true`（当前版本与 min_supported 相等不强制） |
| `note_md` | Markdown，弹窗直接渲染；为空显示「本次更新未提供说明」 |
| `channels[].url` | 「立即更新」跳转入口。**若为 GitHub Releases 页面链接，客户端会经 GitHub API 解析当前平台安装包直链**（zentao-max-{version}-{os}-{arch} 命名）走 下载→应用→重启 自更新管线；解析不出则退化为浏览器打开发布页 |

### 边界（客户端已实测的判定口径）

- `current` 缺失 → 视为旧版（has_update=true）。
- **`current` 不可解析（如 `dev`、`main-8005430`）→ 目前返回 has_update=false，
  这与缺失语义矛盾且会让 dev 构建永远收不到提示**（已提单 P1，建议改为按缺失处理）。
- 版本比较支持 v 前缀/多段/前导零/prerelease（实测引擎正确）。

## 2. 更新日志（What's New）

```
GET {base}/products/{slug}/changelog?locale=zh-CN&limit=20
```

- `limit` 默认 20、客户端上限 50。

### 成功响应（200）

```json
{
  "slug": "zentao-max",
  "items": [
    { "version": "0.6.1", "released_at": "2026-11-04T06:00:00Z", "note_md": "## 0.6.1\n- ..." }
  ]
}
```

- `note_md` 可空（客户端显示「本次更新未提供说明」）。

## 3. 错误与非 200

- 客户端对**任何非 200/网络失败**静默降级：保留上次快照、不弹错、仅内部日志。
- 2xx 之外无需重试语义；429 时客户端不自动退避（当前版本），建议服务端
  返回 `Retry-After`。
- 产品未注册返回 404 也不会对用户暴露。

## 4. 服务端上线清单（murphyyi.com 网关侧）

1. 网关路由 `/vms/v1/*` → 发布管理服务（内部现网 10.10.10.21:30180 同源协议）。
2. 注册产品 `zentao-max`，GitHub 渠道置 `published` 且 `url` 指向
   `https://github.com/yi-nology/zentao-max/releases/tag/v{version}`。
3. 每次发版在管理台填 `upgrade_note`（Markdown）并置版本状态 `released`。
4. 建议开启 HTTPS + 简单限流（客户端每 24h 一次自动检查 + 手动检查，频率极低）。
5. 版本状态置 released 后对外生效延迟（服务端缓存）对客户端无感（静默降级）。

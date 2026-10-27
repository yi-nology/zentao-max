# 禅道接口参考（ZenTao biz8.x · realm 可配置）

本文记录 `zt.py` 依赖的接口细节与踩坑。接口路径与字段名来自对 biz8.x 实例的实测。

## 1. 网络链路：SSH 本地转发

如果禅道在内网、本机不能直连，可以在跳板机上做本地端口转发：

```bash
ssh -N -L 18443:zentao.example.com:443 jumpbox
```

于是本机 `https://zentao.example.com:18443/` 即禅道。好处：本机有 python + curl + 原生 UTF-8，
Windows 跳板机上的编码/换行/变量名坑全部消失。

### 链路检查

| 检查项 | 期望结果 |
|---|---|
| 跳板机 ping 禅道 | 通 |
| `ssh -L` 转发 | `Local forwarding listening on 127.0.0.1 port 18443` |
| 经隧道 GET `/user-login-Lw==.html` | HTTP 200，`<title>用户登录 - 禅道</title>` |
| 本机直连禅道域名 | DNS 不通是正常的，说明必须走隧道 |

### 三个必须处理的细节

1. **必须绕开本机代理。** 本机若设置了 `HTTP_PROXY/HTTPS_PROXY`，curl 会把它当 HTTP 代理去
   `CONNECT zentao.example.com:18443`，收到 502。→ curl 加 `--noproxy '*'`；python 用
   `urllib.request.ProxyHandler({})`。`zt.py` 已内置。
2. **SNI 必须是禅道域名。** 用 `--resolve zentao.example.com:18443:127.0.0.1`（curl）或劫持
   `socket.getaddrinfo`（python），这样 TLS SNI 与 Host 头都是真实域名，TLS 证书与后端路由才正常。
3. **隧道进程跨命令可能无法常驻。** 某些 agent 沙箱会用 job object 连带回收子进程，`ssh -f -N`、
   `DETACHED_PROCESS` 都留不住 ssh。所以 `zt.py` 不假设隧道常驻，而是**每次调用按需重建**
   （进程内子进程 + `atexit` 回收，约 1~2s）；若检测到端口已在监听则直接复用（适合用户自己起的常驻隧道）。

### VPN 掉线表现

跳板机到内网主机的 SSH 报 `Connection closed by UNKNOWN port 65535` 或
`channel 0: open failed: connect failed` = 跳板机到该网段 TCP 失败 = **VPN 隧道断**。
判据：同网段多台同时挂 = 网络问题。恢复方式：重连公司 VPN / 重新登录客户端。

## 2. 登录

```http
GET  /user-login-Lw==.html          # 拿初始 cookie
GET  /user-refreshRandom.html       # → 纯文本随机数
POST /user-login.json
     account=<ZT_USERNAME>
     password=<ZT_PASSWORD>
     verifyRand=<上一步>
     keepLogin=0&referer=/
     realm=<ZT_REALM>               # 实例需要统一认证域时才带
```

成功返回：

```json
{"status":"success","user":{"id":1234,"account":"your-account",
 "realname":"你的名字","email":"you@example.com", ...}}
```

要点：

- `realm` 是统一认证域。**不同实例要求不同**：需要时登录失败会提示域不匹配；不需要时不要发送该字段。
  `zt.py` 已做成「`ZT_REALM` 非空才发送」。
- 密码含特殊字符**不要**在 shell 里拼，凭证文件直读即可（python 不做 shell 转义）。
- 登录页路径里的 `Lw==` 是禅道对 `index` 一类的 base64 编码，别改。

## 3. 填工时（本 skill 的核心接口）

`GET /effort-createForObject-task-{taskID}.html` 返回工时表单页（约 27 KB）。表单结构：

```html
<form method='post' target='hiddenwin' id='createEffort' class='hidden'>
  <input  type='hidden' name='id[1]'         value='1' />
  <input  type='text'   name='dates[1]'      value='2026-01-15' />
  <input  type='hidden' name='objectType[1]' value='task' />
  <input  type='hidden' name='objectID[1]'   value='123456' />
  <textarea             name='work[1]'></textarea>
  <input  type='text'   name='consumed[1]' />
  <input  type='text'   name='left[1]' />
  ... id[2..5] 同样一组（同一对象最多 5 行）
  <input  type='text'   name='testPackageVersion' />
</form>
```

**提交**：POST 到同路径的 `.json` 版本（表单本身 `target='hiddenwin'` 无 action，由 JS 提交）：

```
POST /effort-createForObject-{task|bug}-{objectID}.json
     id[1]=1
     dates[1]=2026-01-15
     objectType[1]=task
     objectID[1]=123456
     work[1]=<工作内容>
     consumed[1]=2
     execution[1]=1234                     # 任务必带（部分实例校验「对象为任务时执行不能为空」）
     left[1]=38                            # ★必发★，且是绝对剩余值（见下）
     testPackageVersion=<仅 left=0 时> example-pkg=1.0.0-1
```

成功判据（三种形态都要认）：

- **成功** → `HTTP 200` + `<html><meta charset='utf-8'/><style>body{background:white}</style><script>if(parent !== window) parent.location.reload(true); </script>`
  —— 这是成功响应，不是失败；
- **表单校验失败** → `HTTP 200` + `{"status":"success","data":"{\"message\":\"日期1 : 剩余不能为空。\"}"}`
  —— ★**最外层 `status` 恒为 `"success"`，真正的业务结果在 `data` 里**★。只看 `"success"` 字符串会把失败当成功；
- **业务失败** → `{"result":"fail","message":"…"}` 或 `alert('对象为任务时执行不能为空！')`。

`zt.py` 的 `_judge()` 统一判定：先解 `data` 这层（转义 JSON 字符串）看 `message`，再退到显式失败/成功模式。

> ⚠️ **判定成功也必须回读。** 唯一可靠的验收是 `GET /my-work.json` 里该任务的 `consumed` 有没有涨。

### left 的真相

**`left` 必填、且是绝对值。** 证据：

1. **必填**：不带 `left[1]` 提交 → `{"status":"success","data":"{\"message\":\"日期1 : 剩余不能为空。\"}"}`，
   工时**完全没写进去**（任务 `consumed` 不变）。
2. **绝对值**（受控实测）：写前 `estimate=116, consumed=102, left=30`；
   发 `consumed=5, left=25` → 写后 `consumed=107, left=25`。**发多少就落多少**，不是叠加。

表单页 JS 只是**给表单预填一个默认值**（浏览器里看到的数字），不是服务端的算法：

```js
estimateTotally = 116;  consumedTotally = 102;      // 页面变量
leftNum = (estimateTotally - consumedTotally - ch + $prev)   // ch=本行消耗, $prev=本行的旧值
$('#left' + no).val(leftNum);                       // 仅赋给 input 的 value
```

人手工填的时候，看到的是这个默认值（即 `最初预计 − 总计消耗`）——所以**别人**的历史记录普遍满足
`预计剩余 = 最初预计 − 总计消耗`；那只是「大家接受了默认值」，不是服务端强制。

**脚本的策略**：默认取「该任务**当前**剩余 − 本次消耗」（不去重算 `最初预计 − 总计消耗`，
因为不少任务的历史 left 早已被人手工改过，重算会跳变）。
同一对象多行 → 按提交顺序累计扣减，dry-run 会把每一行的变化打出来。

**已知不保证**：连续写入时，偶尔最后一次只落了 `consumed`，left 停在旧值
（响应仍是成功脚本；`task-view` 变更史里那条记录没有「修改了 预计剩余」）。
所以：**提交成功 ≠ left 落对，一切以回读为准**；要精确修 left 就去 `task-edit-{id}.html` 手工填。

**旧事故的复盘**：曾出现发 `left=1036` 实际落 `2033` 的叠加异常，原因是那次**没带 `execution`**，
触发实例的同步插件走了一条异常路径把任务级 left 叠加坏了。带 `execution` 后按绝对值落值正常。
所以 `execution` 不只是「可选补充」，**必发**。

### 工时同步插件会拦下 update/delete

部分实例接了 外部工时系统的工时同步。网页上有入口：

| 动作 | 路径 | 实测 |
|---|---|---|
| 更新工时 | `POST /effort-edit-{effortID}.html`（字段**不带**行号：`date/consumed/left/work/objectType/objectID/execution/product[]`） | `{"result":"fail","message":"同步工时-更新的禅道工时数据不存在,请检查"}` —— 改不动 |
| 删除工时 | `GET /effort-delete-{effortID}.html`（`my-effort.html` 里就是纯链接） | 会动外部工时系统数据，谨慎 |

所以：**脚本只新建工时，不改不删已有工时**；需要修就请用户去网页处理。

### 语义红线

| 字段 | 语义 |
|---|---|
| `consumed` | 本次消耗工时，**禅道拒绝 0** |
| `left` | 任务级「预计剩余」**绝对值**（发多少落多少）。**必发**，不发整条表单被拒（「日期N : 剩余不能为空」）。**填 0 → 任务立刻变 `done`**，且此时 `testPackageVersion` 必填 |
| `testPackageVersion` | 格式 **`包名=版本号`（等号，不是连字符）**，如 `example-pkg=1.0.0-1` |
| `work` | 工作日志正文 |
| `execution` | 任务必带（部分实例校验）；表单页里没有这个 input，是随请求附送的 |

## 4. 查询接口

| 用途 | 接口 | 备注 |
|---|---|---|
| 我的任务 | `GET /my-work.json` | 外层 `{status,data}`，**`data` 是转义 JSON 字符串需二次解析**。`data.tasks[]` 含 `id/name/status/consumed/left/estimate/execution/project` |
| 我的日志 | `GET /my-effort.json` | 外层同构，`data.efforts[]`。**回查就用这个**，含 `id/objectType/objectID/date/consumed/left/work`。★**首页只返回 20 条且非严格按日期排序**★ —— 条数少时按日期筛「0 条」可能是漏的，判真伪要看 `/task-view-{id}.html` 的变更史 |
| 按用户+月份 | `GET /effort-ajaxGetEfforts-{uid}-{YYYYMM}.json` | 部分实例恒返回 `[]`，不可用（曾因此让回查误报「0 条」） |
| 任务详情 | `GET /task-view-{id}.html` | HTML（约 90KB），**含完整变更历史**（「修改了 总计消耗，旧值为…新值为…」），排查数值异常的第一手证据 |
| 全文搜索 | `GET /search-index.html?words=关键词` | 偶发超时，需重试；从 HTML 提 `task-view-(\d+)` / `bug-view-(\d+)` |
| 批量填工时 | `POST /effort-batchCreate-{date}.html?onlybody=yes` | 另一条路，本 skill 不用 |

**`/my-task.json` 可能返回 `user-deny-my-task`**（该 vision 无此菜单权限）→ 用 `my-work.json` 代替。

二次解析示例：

```python
inner = json.loads(json.loads(txt)["data"])     # 必须两次
tasks = inner["tasks"]
```

## 5. BUG 相关

`bug-resolve` / `bug-close` / `bug-edit` 及质量分析字段的写法，见 `../../zentao-bug/SKILL.md`。
这里只补充一条与工时相关的：

- `bug-edit` 是**全量更新**，必须带上 `bug-view` 取回的 `openedBuild`/`execution`/`title` 原值，否则必填校验失败且 `title` 被占位值覆盖。
- `resolution` 相关字段与工时无关，别在填工时流程里顺手动它们。

## 6. 与 PowerShell 方案的差异

| | 跳板机 PowerShell | 本 skill |
|---|---|---|
| 执行位置 | 跳板机上的 PowerShell | **本机 python** |
| 网络 | 跳板机本地 curl/PowerShell 直连 | **SSH -L 本地转发** |
| 中文 | 需 quote_plus 成 %XX 绕过编码 bug | 原生 UTF-8，`urlencode` 即可 |
| 输入 | 用户口述 / AI 临时组参 | **固定 Excel 模板** |
| 防重 | 无 | **台账 `.ledger/`** |
| 干跑 | 无 | **`plan` 默认 dry-run** |
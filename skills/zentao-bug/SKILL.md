---
name: zentao-bug
description: 禅道（ZenTao）BUG 查询、解决、关闭、编辑与质量分析字段填写。当用户提供 BUG 号要求查看详情、解决/关闭 BUG、修改修复包版本、填写原因分析/处理意见/实现说明/测试建议，或需要批量处理 BUG 时使用。
agent_created: true
---

# 禅道 BUG 处理

覆盖：查 BUG、搜索 BUG、解决、关闭、编辑字段、批量填「修复包版本 + 原因分析 + 处理意见 + 实现说明 + 测试建议」。

## 0. 两种执行方式

| 方式 | 适用场景 | 入口 |
|---|---|---|
| **本机 python + SSH 隧道（推荐）** | 跳板机允许端口转发 | `../zentao-timelog/scripts/zt.py` 的隧道/登录逻辑；`scripts/get_bug.py` 可直接查 BUG |
| 跳板机 PowerShell（备选） | 跳板机是 Windows、无 curl/python | `scripts/close_bug.ps1`、`scripts/set_quality_fields.ps1` |

### 配置

统一用环境变量 / 凭证文件（默认 `~/.zentao-credentials.env`）：

```ini
ZT_HOST=zentao.example.com
ZT_PORT=18443
ZT_JUMPBOX=jumpbox
ZT_REALM=your-realm
ZT_USERNAME=your-account
ZT_PASSWORD=your-password
```

PowerShell 方案的 `zt_creds.txt`（放在跳板机用户目录）使用同样的键，另可加：

```ini
ZT_BASE=https://zentao.example.com
```

> 禅道版本以实际实例为准；下面接口路径实测于 biz8.x。

## 1. BUG 生命周期

```
active（激活） → resolved（解决） → closed（关闭）
```

- `closed` 状态的 BUG 不能再 `bug-resolve`（返回空），要改得先 `bug-activate` 激活。
- `bug-edit` 在任意状态都可用（改字段 + 追加 comment）。

## 2. 核心接口

| 操作 | 接口 | 必填 body |
|------|------|----------|
| 查看 | `GET /bug-view-{id}.json` | — |
| 解决 | `POST /bug-resolve-{id}.json` | `resolution=fixed&resolvedBuild={版本}&comment={说明}` |
| 关闭 | `POST /bug-close-{id}.json` | `comment={说明}` |
| 编辑字段 | `POST /bug-edit-{id}.json` | 必须带齐 `openedBuild` + `execution` + `title` 原值（全量更新，见踩坑 #13） |
| 激活 | `POST /bug-activate-{id}.json` | 视实例要求 |

返回 `{"status":"success"}` 通常是成功，但**必须回查**（见踩坑 #19）。

## 3. 登录

### 3.1 隧道方案（推荐）

```bash
ssh -N -L 18443:zentao.example.com:443 jumpbox
```

之后本机按 `https://zentao.example.com:18443` 访问，并注意：

- 必须绕开本机代理（`urllib.request.ProxyHandler({})` 或 curl `--noproxy '*'`）。
- SNI 必须是禅道域名：curl 用 `--resolve zentao.example.com:18443:127.0.0.1`；python 劫持 `socket.getaddrinfo`。
- 登录序列：

```http
GET  /user-login-Lw==.html          # 拿初始 cookie
GET  /user-refreshRandom.html       # → 纯数字随机数
POST /user-login.json
     account=<ZT_USERNAME>
     password=<ZT_PASSWORD>
     verifyRand=<上一步>
     keepLogin=0&referer=/
     realm=<ZT_REALM>               # 仅实例需要统一认证域时发送
```

成功返回：

```json
{"status":"success","user":{"id":1234,"account":"your-account","realname":"你的名字", ...}}
```

- `realm` 因实例而异；不需要时不要发送，需要时缺失会登录失败。
- 密码含特殊字符**不要**在 shell 里拼，从凭证文件读取。

### 3.2 跳板机 PowerShell

跳板机（Windows）默认 shell 是 `cmd.exe`，无 `curl`/`base64`/`grep`，且 PS4 对中文/换行很敏感。流程：

```bash
scp /本地/脚本.ps1 "jumpbox:C:/Users/user/脚本.ps1"
ssh jumpbox 'powershell -NoProfile -ExecutionPolicy Bypass -File C:\Users\user\脚本.ps1'
```

**脚本必须纯 ASCII**（注释也别写中文），中文值走 `quote_plus` 成 `%XX` 或 UTF-8 文件。

## 4. 查看 BUG

```bash
python scripts/get_bug.py --bug 123456
```

直接用接口时注意：`bug-view-{id}.json` 的外层是 `{status, data}`，**`data` 是转义过的 JSON 字符串，必须二次解析**：

```python
outer = json.loads(txt)
inner = json.loads(outer["data"])
bug = inner["bug"]
product_name = inner["products"][str(bug["product"])]
```

### 关键字段

| 字段 | 说明 |
|---|---|
| `title` | 标题。安全 BUG 标题里的【系统名】通常就是「影响系统」 |
| `status` / `type` / `severity` / `pri` | 状态 / 类型 / 严重度 / 优先级 |
| `product` + `inner["products"][id]` | 产品 ID → 产品名 |
| `openedBuild` | 影响版本 |
| `steps` | 重现步骤（HTML，常含 CVE、影响版本、影响分支） |
| `os` | 影响系统字段（可能为空，以标题【】为准） |
| `fixedPackageVersion` | 修复包版本，格式 `包名=版本号` |
| `reasonAnalysis` / `advice` / `implementation` / `testAdvise` | 质量分析四字段 |

### 搜索 BUG

```bash
# 全文搜索（偶发超时，需重试）
GET /search-index.html?words=关键词
# 从 HTML 中提取 bug-view-(\d+) / task-view-(\d+)
```

---

## 5. 解决 / 关闭 / 编辑

### 5.1 解决 BUG

```http
POST /bug-resolve-{id}.json
resolution=fixed
resolvedBuild=<版本>       # 必填，可先用 openedBuild 或 trunk
comment=<说明>
```

### 5.2 关闭 BUG

```http
POST /bug-close-{id}.json
comment=<说明>
```

### 5.3 编辑字段（bug-edit）

**`bug-edit` 是全量更新**：必须先从 `bug-view` 取回 `openedBuild`、`execution`、`title` 原值，连同要改的字段一起 POST；否则必填校验失败，或 `title` 被占位值覆盖。

```text
openedBuild=<原值>
execution=<原值>
title=<原值，&#039; 先替换成 '>
fixedPackageVersion=example-pkg=1.0.0-1
```

可更新字段：

- `fixedPackageVersion`：修复包版本，格式 **`包名=版本号`（等号，不是连字符）**
- `resolution` / `resolvedBuild`
- `comment`：追加备注
- `title`：**改了会覆盖，谨慎**
- `reasonAnalysis` / `advice` / `implementation` / `testAdvise`：质量分析四字段

### 5.4 中文编码（PowerShell 方案）

中文绝不能乱码：

- 中文值写进 UTF-8 文件，PS 读取后**直接发 UTF-8 字节**；
- `ContentType` 必须带 `; charset=utf-8`；
- **不要**用 `[uri]::EscapeDataString`（`%XX` 会被按 ISO-8859-1 解码成乱码）；
- 如果走 `quote_plus` 路线，先在本地把中文编码成纯 ASCII 的 `%XX`，PS 只负责原样发送。

---

## 6. 质量分析五字段

关闭/解决 CVE 类 BUG 常要求一次写全 5 个字段：

```
fixedPackageVersion
reasonAnalysis    原因分析
advice            处理意见
implementation    实现说明
testAdvise        测试建议
```

一次 `bug-edit` 全量写全，不要分多次。

### 6.1 内容模板

- **原因分析**：什么组件、什么功能、什么情况触发、攻击者怎么利用、造成什么后果。
- **处理意见**：采用什么补丁（PR 号）、修复思路、改动范围、是否影响接口/ABI。
- **实现说明**：应用哪个 patch、修改哪个文件、主要改动点（原来怎么做 → 改成怎么做）。
- **测试建议**：怎么构造测试用例、预期结果、影响域。

用通俗中文，别堆代码术语。

### 6.2 生成 map（本机 python）

```python
import urllib.parse

fpv = "example-pkg=1.0.0-1"
ra = "原因分析内容..."
ad = "处理意见内容..."
impl = "实现说明内容..."
ta = "测试建议内容..."

with open(r"C:\Users\user\all_map.txt", "w", encoding="ascii") as f:
    f.write(
        f"123456|{fpv}|{urllib.parse.quote_plus(ra)}|{urllib.parse.quote_plus(ad)}|"
        f"{urllib.parse.quote_plus(impl)}|{urllib.parse.quote_plus(ta)}\n"
    )
```

map 每行：`bid|fixedPackageVersion|ra_quote|ad_quote|impl_quote|ta_quote`。

### 6.3 执行并验证

```bash
scp all_map.txt jumpbox:C:/Users/user/all_map.txt
ssh jumpbox 'powershell -NoProfile -ExecutionPolicy Bypass -File C:\Users\user\set_quality_fields.ps1'
```

输出里应看到 `FPV_OK RA_OK AD_OK IMPL_OK TA_OK`。**接口返回 success 不代表字段写进去了**，必须回查。

---

## 7. 批量脚本

### 7.1 `close_bug.ps1`

流程：`bug-view 取原值 → bug-edit 设 fixedPackageVersion → bug-resolve（仅 active）→ bug-view 回查`。

map 格式（每行）：

```text
bid|fixedPackageVersion|comment_quote_plus
123456|example-pkg=1.0.0-1|已在%20example-pkg%201.0.0-1%20修复
```

### 7.2 `set_quality_fields.ps1`

流程：`bug-view 取原值 → bug-edit 一次写 5 字段 → 回查`。
map 格式见 6.2。

### 7.3 本机 python 方案

`get_bug.py` 可直接查详情。若要把 resolve/close/edit 也做成 python，可复用
`../zentao-timelog/scripts/zt.py` 的 `Zentao` 类（登录、cookie、UTF-8 urlencode 都已处理），
再按第 2 节的接口 POST 即可。

---

## 8. 踩坑清单（实测）

| # | 现象 | 根因 / 解决 |
|---|---|---|
| 1 | `ssh jumpbox "...; uname"` 只回显一行 | 跳板机是 Windows，cmd 用 `&` 分隔，不认 `;`/单引号/`$()` → 远程命令走 PowerShell |
| 2 | `'curl.exe' 不是内部或外部命令` | Windows 跳板机无 curl/base64/grep → 全程 PowerShell（`HttpWebRequest` + `JavaScriptSerializer`） |
| 3 | heredoc 喂 `powershell -Command -` 只执行第一行 | PS4 + Windows sshd 的 stdin 行处理缺陷 → 本地写 `.ps1` → scp → `powershell -File` |
| 4 | PS 脚本报「意外标记 `}`」 | 脚本含中文，PS4 按 GBK 读 UTF-8 字节 → 脚本纯 ASCII |
| 5 | `FromBase64String` 报「无效字符」 | base64 末尾 `=` padding 被 `cut -d= -f2` 吃掉 → 用 `cut -d= -f2-`，或直接读明文文件 |
| 6 | `Invoke-WebRequest` 访问禅道超时，`WebClient` 正常 | PS4 的 Invoke-WebRequest 行为怪异 → 用 `HttpWebRequest` + `CookieContainer` |
| 7 | 变量赋值报「参数名无效」 | `$pid` 是 PS 只读自动变量 → 改名 `$prodId` 等 |
| 8 | `ConvertFrom-Json` 报「参数 name 无效」 | PS4 无法处理超大 JSON（BUG 详情含全产品/版本列表，可能 600KB+）→ 用 `JavaScriptSerializer` 并设 `MaxJsonLength` |
| 9 | 输出中文乱码 | Windows 控制台默认 GBK → 设 `[Console]::OutputEncoding=[Text.Encoding]::UTF8`，或写 UTF-8 文件 scp 回本地读 |
| 10 | 全文搜索偶发超时 | 端点慢 → 2~4 次重试 |
| 11 | `my-bug.json` 返回 `user-deny-my-bug` | 该 vision 无 my-bug 菜单权限 → 用全文搜索或 `bug-browse` |
| 12 | `bug-resolve`/`bug-close`/`bug-edit` 的中文 comment 乱码 | `EscapeDataString` 的 `%XX` 被按 ISO-8859-1 解码 → 中文不转义，发 UTF-8 字节，`ContentType` 带 `charset=utf-8` |
| 13 | `bug-edit` 返回 success 但 **title 被覆盖** | `bug-edit` 是全量更新，`openedBuild`/`execution`/`title` 必填；只传要改的字段会报错或被占位值覆盖 → 先 `bug-view` 取原值一起 POST，绝不用占位符 |
| 14 | `bug-resolve` 对 **closed** 的 BUG 返回空（LEN=0） | 状态机不允许 closed→resolved → 先 `bug-activate`，或用 `bug-edit` 带 `comment` 追加备注 |
| 15 | `fixedPackageVersion` 写 `example-pkg-1.0.0-1`（连字符）格式不对 | 规范是「包名=版本号」用等号 → `example-pkg=1.0.0-1` |
| 16 | `bug-edit` 能否写 `reasonAnalysis`/`advice`/`implementation`/`testAdvise` | **可以**。`bug-edit` 全量 + UTF-8 字节提交即可写入四个字段（早期误判为前端 ajax only） |
| 17 | `bug-edit` 后 title 变 `&amp;#039;`（网页渲染 `&#039;` 字面，应显示 `'`） | `bug-view` 返回的 title 含 `&#039;`，`bug-edit` 会二次转义 `&` → 传之前把 `&#039;` 替换成 `'` |
| 18 | PS4 读 UTF-8 文件全坏（`ReadAllLines`/`ReadAllText` 返回空） | PS4 .NET 的 UTF-8 读取 bug → 中文先用 python `quote_plus` 成纯 ASCII（`%XX`），PS 默认编码读 ASCII 即可 |
| 19 | `bug-updatetestpackageversion` 返回 success 但 `fixedPackageVersion` 没写入 | JSON body 不进 `$_POST`，禅道走页面渲染分支 → 用 `bug-edit` 全量 + urlencoded UTF-8；**必须回查** |
| 20 | `/bug-updatetestpackageversion-{id}.json` + multipart/JSON 都读不到字段 | 该表单走 Vue 前端独立 ajax，标准 PATH_INFO 读不到 → 放弃该端点，用 `bug-edit` |
| 21 | `bug-resolve` 不传 `resolvedBuild` 报「【解决版本】不能为空」 | `resolvedBuild` 必填 → 传 `trunk` 或从 `bug-view` 取 `openedBuild` |
| 22 | `bug-resolve` 后 `fixedPackageVersion` 没自动设 | resolve 只设 status/resolution/resolvedBuild/comment → 先 `bug-edit` 设版本，再 resolve |
| 23 | `.ps1` 报「字符串缺少终止符」或「意外标记」 | `.ps1` 是 LF 换行，PS4 解析错乱 → 生成时强制 CRLF（`content.replace('\n','\r\n')`），或 scp 后 `Set-Content` 归一化 |
| 24 | `.ps1` 报「变量引用无效，`:` 后面的变量名称字符无效」 | 双引号串里 `"$var:..."` 被当 drive 引用（像 `$env:`）→ 用 `"${var}"` 或字符串拼接 |

---

## 9. 安全与操作红线

1. **解决/关闭 BUG 是对外动作**：执行前必须让用户确认 BUG 号、版本、comment 内容。
2. **先 `bug-view`，后 `bug-edit`**：edit 是全量更新，漏字段会破坏数据。
3. **写后必回查**：`{"status":"success"}` 不代表字段真的落库（尤其 `fixedPackageVersion` 与质量分析四字段）。
4. **closed 不能直接 resolve**：需要先 activate。
5. **不要批量猜测版本号**：`fixedPackageVersion` 的包名和版本必须来自构建产物/包管理，不确定就问用户。
6. **凭证不落盘到脚本**：用 `~/.zentao-credentials.env` 或跳板机的 `zt_creds.txt`，不要把密码写进 `.ps1`/`.py`。
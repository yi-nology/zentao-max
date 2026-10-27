# 禅道技能包（脱敏版）

本技能包随 zentao-max 仓库分发（`skills/` 目录），可直接整体复制到所用 agent 的 skills
目录使用；`zentao-bug` / `zentao-timelog` 直连禅道实例（session 方式，支持 realm 统一
认证域），与 zentao-max 应用本身的 Token/Session 双认证模式覆盖同一类实例。

把本机在用的禅道（ZenTao）相关技能整理、脱敏后的可分享副本，包含两块：

| 目录 | 作用 |
|---|---|
| `zentao-timelog/` | 用 Excel 批量填报禅道工时/任务日志：dry-run、写入、回查、台账去重 |
| `zentao-bug/` | 禅道 BUG 查询、解决、关闭、编辑，以及质量分析字段填写 |

## 已脱敏的内容

- 内网域名、IP、端口转发目标 → 占位符（`zentao.example.com`）
- 账号、密码、真实姓名、uid、邮箱、公司名 → 全部移除，改为环境变量 / 本地凭证文件
- 真实任务号、BUG 号、包名、工作内容、台账记录 → 替换为通用示例（`123456`、`example-pkg=1.0.0-1`）
- 与禅道无关的其它内部平台（门禁、gerrit、koji 等）内容 → 未纳入本包

## 配置

1. 复制配置模板并填写：

   ```bash
   cp config.example.env ~/.zentao-credentials.env
   # 编辑 ~/.zentao-credentials.env
   ```

   字段说明：

   | 变量 | 含义 |
   |---|---|
   | `ZT_HOST` | 禅道域名 |
   | `ZT_PORT` | 隧道本地端口；直连时用 443 |
   | `ZT_JUMPBOX` | `~/.ssh/config` 里的跳板机别名；留空 = 直连 |
   | `ZT_REALM` | 统一认证域；不需要时留空 |
   | `ZT_USERNAME` / `ZT_PASSWORD` | 登录账号密码 |
   | `ZT_CRED` | 可选，凭证文件路径（默认 `~/.zentao-credentials.env`） |

2. 依赖：Python 3.9+，`pip install openpyxl`。
3. 网络：
   - 有跳板机：`zt.py` 会自动拉起 `ssh -N -L <ZT_PORT>:<ZT_HOST>:443 <ZT_JUMPBOX>`，
     前提是 `~/.ssh/config` 已配好别名、跳板机可登录、VPN 在线。
   - 无跳板机：把 `ZT_JUMPBOX` 留空，并设置 `ZT_PORT=443`，走本机直连。

## 安装为技能

把 `zentao-timelog/` 和 `zentao-bug/` 两个目录整体复制到你所用 agent 的 skills 目录
（例如 `~/.claude/skills/`、`~/.zcode/skills/` 或项目的 skills 目录），保持 `SKILL.md`
位于每个 skill 的根目录即可。也可以不安装，直接按本文档的命令行方式使用。

## 快速开始（工时填报）

```bash
cd zentao-timelog
python scripts/zt.py whoami
python scripts/zt.py tasks
python scripts/zt.py plan   "C:/path/to/禅道工时填报.xlsx"
python scripts/zt.py submit "C:/path/to/禅道工时填报.xlsx" --yes
python scripts/zt.py verify 2026-01-15
```

**红线**：先 `plan` 再 `submit`；`submit` 必须显式 `--yes`；提交后看回读校验，不能只看成功响应。

## 快速开始（BUG）

```bash
cd zentao-bug
python scripts/get_bug.py --bug 123456
```

`close_bug.ps1` / `set_quality_fields.ps1` 是「跳板机为 Windows、不能跑 Python」时的备选方案。
使用前把脚本里的 `C:\Users\user` 改成跳板机上的实际用户目录，并在同目录放 `zt_creds.txt`
（格式见 `zentao-bug/SKILL.md`）。

## 目录结构

```
skills/
├── README.md                 本文件
├── config.example.env        凭证配置模板
├── zentao-timelog/
│   ├── SKILL.md
│   ├── reference/chandao-api.md
│   ├── scripts/zt.py
│   ├── scripts/make_template.py
│   ├── templates/禅道工时填报.xlsx
│   ├── .cache/
│   └── .ledger/
└── zentao-bug/
    ├── SKILL.md
    └── scripts/
        ├── get_bug.py
        ├── close_bug.ps1
        └── set_quality_fields.ps1
```

## 安全建议

- 凭证文件权限设为仅当前用户可读（Linux: `chmod 600`）。
- `.ledger/` 会记录工作内容，分享或提交前先清空。
- 本包只保留接口用法与踩坑，不含任何真实账号、任务或项目数据。
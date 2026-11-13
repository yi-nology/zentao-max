/**
 * MCPGuide 静态数据目录（issue #7-1：从 1716 行 SFC 拆出，纯移动零风险）。
 * 工具目录与 backend/internal/mcp/tools.go 保持一致。
 *
 * 含 mcpBase 插值的模板（接入配置/多语言示例）以 build*(mcpBase) 函数形式
 * 导出，避免模块级依赖 window。
 */

// ---- 工具目录（与 backend/internal/mcp/tools.go 保持一致）----

export interface CatalogTool {
  name: string
  desc: string
  write?: boolean
}

export interface ToolGroup {
  title: string
  note?: string
  tools: CatalogTool[]
}

export const toolGroups: ToolGroup[] = [
  {
    title: '连接与配置',
    note: 'update_zentao_config 可让 AI 引导完成首次接入',
    tools: [
      { name: 'ping', desc: '测试 MCP 服务是否正常运行' },
      { name: 'test_connection', desc: '实时探测禅道连通性（登录会话探针，返回延迟与当前账号）' },
      { name: 'get_zentao_config', desc: '查看当前禅道连接配置（地址/账号/认证域，密码不回显）' },
      { name: 'update_zentao_config', desc: '更新禅道连接配置并立即验证登录、加密持久化；realm 留空为标准 REST，受限实例传对应域名', write: true },
      { name: 'send_verify_code', desc: '麒麟统一认证第一步：账号密码触发短信验证码，返回脱敏手机号', write: true },
      { name: 'sso_login', desc: '麒麟统一认证第二步：提交短信验证码完成登录；频繁登录会触发限流', write: true }
    ]
  },
  {
    title: '数据查询',
    tools: [
      { name: 'get_products', desc: '获取产品列表' },
      { name: 'get_projects', desc: '获取项目列表，可按产品 ID 过滤' },
      { name: 'get_executions', desc: '获取执行/迭代列表，可按项目或产品过滤' },
      { name: 'get_bugs', desc: '获取 Bug 列表，可按产品与状态（active/resolved/closed）过滤' },
      { name: 'get_stories', desc: '获取需求列表，可按产品过滤' },
      { name: 'get_tasks', desc: '获取任务列表，可按产品与执行过滤' },
      { name: 'get_users', desc: '获取用户列表' },
      { name: 'get_my_tasks', desc: '我负责/参与的任务（my-work 视图）；回答「我现在有哪些任务」优先用' }
    ]
  },
  {
    title: '单条详情',
    note: '比列表字段全，分析单个对象时用',
    tools: [
      { name: 'get_bug_detail', desc: '单个 Bug 完整详情（描述/状态/严重级/解决方案/所属执行等）' },
      { name: 'get_task_detail', desc: '单个任务完整详情（类型/状态/预计与消耗/剩余工时/起止日期等）' },
      { name: 'get_story_detail', desc: '单个需求完整详情（描述/验收标准/阶段/估时/状态等）' }
    ]
  },
  {
    title: '工时',
    tools: [
      { name: 'get_timelog', desc: '工时统计数据，可按产品与日期范围过滤' },
      { name: 'get_effort_calendar', desc: '指定用户全年工时日历明细；受限实例仅支持查当前登录账号' },
      { name: 'record_effort', desc: '写禅道日志（挂任务工时）：consumed>0，left 为剩余工时绝对值，不接受未来日期', write: true }
    ]
  },
  {
    title: '写操作',
    note: '只读模式下全部拦截',
    tools: [
      { name: 'add_bug_comment', desc: '给 Bug 添加备注（bug-edit 全量通道，自动带原值）；建议回查确认落库', write: true },
      { name: 'transition_bug', desc: 'Bug 流转 confirm/resolve/close/activate/assign；resolve 必填 resolution 与 resolvedBuild', write: true },
      { name: 'create_bug', desc: '创建禅道 Bug，返回新 Bug ID；标题必填，建议附复现步骤', write: true },
      { name: 'transition_task', desc: '任务流转 start/finish/pause/assign；finish 建议带累计 consumed', write: true }
    ]
  }
]

export const totalToolCount = toolGroups.reduce((n, g) => n + g.tools.length, 0)

// ---- 接入配置模板（含 mcpBase 插值）----

export function buildConfigs(mcpBase: string) {
  const claudeConfig = `{
  "mcpServers": {
    "zentao-max": {
      "command": "/path/to/zentao-max-mcp",
      "args": [],
      "env": {
        // 服务端开启 Token 鉴权时必填，否则可省略
        "ZENTAO_MAX_MCP_TOKEN": "your-secret-token"
      }
    }
  }
}`

const claudeCodeConfig = `# 方式 1：HTTP 远程接入（推荐，无需本地二进制）
claude mcp add --transport http zentao-max ${mcpBase}/mcp

# 服务端开启 Token 时附加请求头
claude mcp add --transport http zentao-max ${mcpBase}/mcp \\
  --header "Authorization: Bearer your-secret-token"

# 方式 2：stdio 子进程
claude mcp add zentao-max -- /path/to/zentao-max-mcp

# 方式 3：项目级共享配置 .mcp.json（仓库根目录）
{
  "mcpServers": {
    "zentao-max": {
      "command": "/path/to/zentao-max-mcp",
      "args": [],
      "env": { "ZENTAO_MAX_MCP_TOKEN": "your-secret-token" }
    }
  }
}

# 验证 / 移除
claude mcp list
claude mcp remove zentao-max`

const zcodeConfig = `# HTTP 远程接入（推荐）
zcode mcp add --transport http zentao-max ${mcpBase}/mcp

# 带 Token
zcode mcp add --transport http zentao-max ${mcpBase}/mcp \\
  --header "Authorization: Bearer your-secret-token"

# stdio 子进程
zcode mcp add zentao-max -- /path/to/zentao-max-mcp

# 验证
zcode mcp list`

const cursorConfig = `{
  "mcpServers": {
    "zentao-max": {
      "command": "/path/to/zentao-max-mcp",
      "args": [],
      "env": { "ZENTAO_MAX_MCP_TOKEN": "your-secret-token" }
    }
  }
}

// 或远程 URL 形态（需客户端支持 MCP 2026-07-28）
{
  "mcpServers": {
    "zentao-max": {
      "url": "${mcpBase}/mcp"
    }
  }
}`

const codexConfig = `# ~/.codex/config.toml
[mcp_servers.zentao-max]
command = "/path/to/zentao-max-mcp"
args = []

[mcp_servers.zentao-max.env]
ZENTAO_MAX_MCP_TOKEN = "your-secret-token"

# 或命令行添加
# codex mcp add zentao-max -- /path/to/zentao-max-mcp`

const openCodeConfig = `{
  "mcp": {
    "zentao-max": {
      "type": "local",
      "command": ["/path/to/zentao-max-mcp"],
      "environment": {
        "ZENTAO_MAX_MCP_TOKEN": "your-secret-token"
      }
    }
  }
}

// 或远程形态
{
  "mcp": {
    "zentao-max": {
      "type": "remote",
      "url": "${mcpBase}/mcp",
      "headers": {
        "Authorization": "Bearer your-secret-token"
      }
    }
  }
}`

const traeConfig = `{
  "mcpServers": {
    "zentao-max": {
      "command": "/path/to/zentao-max-mcp",
      "args": [],
      "env": { "ZENTAO_MAX_MCP_TOKEN": "your-secret-token" }
    }
  }
}`
  return { claudeConfig, claudeCodeConfig, zcodeConfig, cursorConfig, codexConfig, openCodeConfig, traeConfig }
}

// ---- 端点表 ----

export const mcpEndpoints = [
  { method: 'POST', path: '/mcp', desc: '统一入口：标准 JSON-RPC 与简化协议自动分流' },
  { method: 'GET', path: '/mcp?action=...', desc: '统一入口 GET 形式（简化协议）' },
  { method: 'GET', path: '/mcp/tools', desc: '列出全部工具（含 inputSchema）' },
  { method: 'GET', path: '/mcp/tools/:name', desc: '单个工具定义' },
  { method: 'GET', path: '/mcp/ping', desc: 'Ping（免工具调用探测）' },
  { method: 'POST', path: '/mcp/<工具全名>', desc: '简化协议按工具名单独端点（如 /mcp/get_bugs）' }
]

export const restEndpoints = [
  { method: 'GET', path: '/api/v1/mcp/status', desc: 'MCP 模式状态（热重载生效值）' },
  { method: 'GET', path: '/api/v1/mcp/config', desc: 'MCP 配置快照' },
  { method: 'PUT', path: '/api/v1/mcp/config', desc: '热重载 enabled / readOnly / token / actions' },
  { method: 'GET', path: '/api/products', desc: 'REST：产品列表（传统接口）' },
  { method: 'GET', path: '/api/bugs?productId=1', desc: 'REST：Bug 列表' },
  { method: 'GET', path: '/health', desc: '健康检查' }
]

// ---- 多语言示例 ----

export interface Language {
  name: string
  code: string
}

export function buildLanguages(mcpBase: string): Language[] {
  return [
  {
    name: 'Python · 标准 MCP（HTTP）',
    code: `import json
import urllib.request

BASE = "${mcpBase}/mcp"
VERSION = "2026-07-28"

def rpc(method, params=None, id=1):
    body = {"jsonrpc": "2.0", "id": id, "method": method}
    if params is not None:
        params.setdefault("_meta", {})["io.modelcontextprotocol/protocolVersion"] = VERSION
        body["params"] = params
    req = urllib.request.Request(
        BASE,
        data=json.dumps(body).encode(),
        headers={"Content-Type": "application/json"},
    )
    return json.loads(urllib.request.urlopen(req).read())

# 发现服务（唯一免版本检查的方法）
print(rpc("server/discover"))

# 列出工具
tools = rpc("tools/list", {})["result"]["tools"]
print([t["name"] for t in tools])

# 调用工具
result = rpc("tools/call", {
    "name": "get_my_tasks",
    "arguments": {},
})
print(result["result"]["structuredContent"])`
  },
  {
    name: 'Python · 简化协议（stdio）',
    code: `import subprocess
import json

class ZentaoMCP:
    def __init__(self, binary_path="./zentao-max-mcp"):
        self.process = subprocess.Popen(
            [binary_path],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,   # 日志走 stderr，勿与协议流混用
            text=True,
        )

    def call(self, action, params=None):
        request = {"action": action}
        if params:
            request["params"] = params
        self.process.stdin.write(json.dumps(request) + "\\n")
        self.process.stdin.flush()
        return json.loads(self.process.stdout.readline())

    def close(self):
        self.process.stdin.close()
        self.process.terminate()

mcp = ZentaoMCP()
print(mcp.call("ping"))
print(mcp.call("get_my_tasks"))
print(mcp.call("record_effort", {
    "taskId": 50001, "consumed": 2, "left": 3, "work": "修复登录超时",
}))
mcp.close()`
  },
  {
    name: 'Node.js · 简化协议（HTTP）',
    code: `const BASE = '${mcpBase}/mcp';

async function call(action, params = {}) {
  const res = await fetch(BASE, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ action, params }),
  });
  return res.json();
}

const bugs = await call('get_bugs', { productId: 1, status: 'active' });
console.log(bugs.data);

// 服务端配置 Token 时：
// headers: { 'Content-Type': 'application/json',
//            Authorization: 'Bearer your-secret-token' }`
  },
  {
    name: 'Bash / cURL · 完整流程',
    code: `# 1) 服务发现
curl -s -X POST ${mcpBase}/mcp \\
  -H "Content-Type: application/json" \\
  -d '{"jsonrpc":"2.0","id":1,"method":"server/discover"}'

# 2) 工具列表（必须携带协议版本）
curl -s -X POST ${mcpBase}/mcp \\
  -H "Content-Type: application/json" \\
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list",
       "params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}'

# 3) 查询我的任务
curl -s -X POST ${mcpBase}/mcp \\
  -H "Content-Type: application/json" \\
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call",
       "params":{"name":"get_my_tasks",
                 "_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}'

# 简化协议一行搞定
curl -s "${mcpBase}/mcp?action=get_my_tasks"`
  }
]
}

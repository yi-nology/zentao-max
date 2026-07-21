<template>
  <div class="mcp-guide">
    <h1 class="page-title">MCP 服务对接指南</h1>

    <div class="info-banner">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="20" height="20">
        <path d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
      <span>禅道 Max 支持两种 MCP 模式：<strong>stdio（标准输入输出）</strong>和 <strong>HTTP</strong>。AI 工具推荐使用 stdio；Web 应用和远程调用推荐使用 HTTP。</span>
    </div>

    <!-- 版本 & 服务状态 -->
    <section class="section">
      <div class="status-card">
        <div class="status-row">
          <span class="status-label">版本</span>
          <span class="status-value version-tag">v{{ versionInfo.version || '...' }}</span>
        </div>
        <div class="status-row">
          <span class="status-label">构建时间</span>
          <span class="status-value">{{ versionInfo.buildTime || '...' }}</span>
        </div>
        <div class="status-row">
          <span class="status-label">Git Commit</span>
          <span class="status-value mono">{{ versionInfo.gitCommit ? versionInfo.gitCommit.substring(0, 8) : '...' }}</span>
        </div>
        <div class="status-row">
          <span class="status-label">Go</span>
          <span class="status-value">{{ versionInfo.goVersion || '...' }}</span>
        </div>
        <div class="status-row">
          <span class="status-label">HTTP 服务</span>
          <span class="status-value" :class="httpStatusClass">
            <span class="status-dot"></span>
            {{ httpStatus }}
          </span>
        </div>
        <div class="status-row">
          <span class="status-label">MCP 服务</span>
          <span class="status-value" :class="mcpEnabledClass">
            <span class="status-dot"></span>
            {{ mcpEnabledText }}
          </span>
        </div>
        <div class="status-row">
          <span class="status-label">传输模式</span>
          <span class="status-value mono">{{ mcpStatus.transport || '...' }}</span>
        </div>
        <div class="status-row">
          <span class="status-label">只读模式</span>
          <span class="status-value" :class="mcpStatus.readOnly ? 'error' : 'online'">{{ mcpStatus.readOnly === null ? '...' : (mcpStatus.readOnly ? '开启' : '关闭') }}</span>
        </div>
        <div class="status-row">
          <span class="status-label">Token 鉴权</span>
          <span class="status-value" :class="mcpStatus.tokenSet ? 'online' : 'checking'">{{ mcpStatus.tokenSet === null ? '...' : (mcpStatus.tokenSet ? '已启用' : '未启用') }}</span>
        </div>
        <div class="status-row">
          <span class="status-label">连接地址</span>
          <code class="status-value connection-url">{{ connectionUrl }}/mcp</code>
        </div>
      </div>
    </section>

    <!-- 协议类型说明 -->
    <section class="section">
      <h2 class="section-title">MCP 模式</h2>
      <div class="protocol-cards">
        <div class="protocol-card" :class="{ active: mcpStatus.transport === 'stdio' || mcpStatus.transport === 'both' }">
          <div class="protocol-badge">推荐 AI 工具</div>
          <h3>stdio 标准输入输出</h3>
          <p>独立进程 <code>zentao-max-mcp</code> 通过 stdin/stdout 通信。AI 工具（Claude、Cursor 等）自动管理进程生命周期，无需占用端口。</p>
          <div class="protocol-meta">
            <span class="tag">JSON Lines</span>
            <span class="tag">进程内通信</span>
            <span class="tag">无需端口</span>
          </div>
          <pre class="code-block compact"># 构建独立 stdio 入口
cd backend && make build-mcp

# 命令行测试
echo '{"action":"ping"}' | ./zentao-max-mcp

# 配置 Token 后需在 params 中携带
echo '{"action":"ping","params":{"token":"xxx"}}' | ./zentao-max-mcp</pre>
        </div>
        <div class="protocol-card" :class="{ active: mcpStatus.transport === 'http' || mcpStatus.transport === 'both' }">
          <div class="protocol-badge">推荐 Web/远程</div>
          <h3>HTTP 模式</h3>
          <p>通过 HTTP 端口通信。支持远程调用、Web 应用集成、跨机器访问。本页面所连接的后端默认即此模式。</p>
          <div class="protocol-meta">
            <span class="tag">端口 12345</span>
            <span class="tag">POST/GET</span>
            <span class="tag">CORS 支持</span>
          </div>
          <pre class="code-block compact"># 启动 HTTP 服务
cd backend && go run cmd/server/main.go

# 调用示例
curl -X POST http://localhost:12345/mcp \
  -d '{"action":"ping"}'

# 配置 Token 后需携带 Bearer 头
curl -X POST http://localhost:12345/mcp \
  -H "Authorization: Bearer xxx" \
  -d '{"action":"ping"}'</pre>
        </div>
      </div>
    </section>

    <!-- AI 工具配置 -->
    <section class="section">
      <h2 class="section-title">AI 工具配置</h2>

      <div class="config-tabs">
        <button v-for="tab in configTabs" :key="tab.id" class="config-tab" :class="{ active: activeTab === tab.id }" @click="activeTab = tab.id">
          {{ tab.label }}
        </button>
      </div>

      <!-- Claude Desktop -->
      <div v-if="activeTab === 'claude'" class="config-content">
        <div class="config-header">
          <h3>Claude Desktop</h3>
          <span class="config-badge">Anthropic 官方桌面客户端</span>
        </div>
        <p class="config-desc">编辑配置文件：</p>
        <div class="config-path">~/Library/Application Support/Claude/claude_desktop_config.json</div>
        <pre class="code-block"><code>{{ claudeConfig }}</code></pre>
        <div class="config-steps">
          <div class="step"><span class="step-num">1</span>将上述配置添加到 claude_desktop_config.json</div>
          <div class="step"><span class="step-num">2</span>将 zentao-max 可执行文件路径替换为你的实际路径</div>
          <div class="step"><span class="step-num">3</span>重启 Claude Desktop</div>
          <div class="step"><span class="step-num">4</span>在对话中即可使用禅道相关工具</div>
        </div>
      </div>

      <!-- Claude Code -->
      <div v-if="activeTab === 'claudecode'" class="config-content">
        <div class="config-header">
          <h3>Claude Code</h3>
          <span class="config-badge">Anthropic CLI 编程工具</span>
        </div>
        <p class="config-desc">Claude Code 支持多种配置方式：</p>
        <pre class="code-block"><code>{{ claudeCodeConfig }}</code></pre>
        <div class="config-steps">
          <div class="step"><span class="step-num">1</span>推荐使用 <code>claude mcp add</code> 命令直接添加</div>
          <div class="step"><span class="step-num">2</span>也可以手动编辑 ~/.claude/claude_desktop_config.json</div>
          <div class="step"><span class="step-num">3</span>项目级别配置放在 .claude/settings.json</div>
          <div class="step"><span class="step-num">4</span>使用 <code>claude mcp list</code> 验证配置</div>
        </div>
      </div>

      <!-- Cursor -->
      <div v-if="activeTab === 'cursor'" class="config-content">
        <div class="config-header">
          <h3>Cursor</h3>
          <span class="config-badge">AI 代码编辑器</span>
        </div>
        <p class="config-desc">在项目根目录创建 <code>.cursor/mcp.json</code>：</p>
        <pre class="code-block"><code>{{ cursorConfig }}</code></pre>
        <div class="config-steps">
          <div class="step"><span class="step-num">1</span>在 Cursor 中打开 Settings → MCP</div>
          <div class="step"><span class="step-num">2</span>添加上面的配置</div>
          <div class="step"><span class="step-num">3</span>启用禅道 Max MCP server</div>
        </div>
      </div>

      <!-- OpenCode -->
      <div v-if="activeTab === 'opencode'" class="config-content">
        <div class="config-header">
          <h3>OpenCode</h3>
          <span class="config-badge">开源 AI 编程工具</span>
        </div>
        <p class="config-desc">OpenCode 支持 MCP 协议集成：</p>
        <pre class="code-block"><code>{{ openCodeConfig }}</code></pre>
        <div class="config-steps">
          <div class="step"><span class="step-num">1</span>编辑 ~/.opencode/config.json 或项目 .opencode/config.json</div>
          <div class="step"><span class="step-num">2</span>或使用 <code>opencode mcp add</code> 命令</div>
          <div class="step"><span class="step-num">3</span>重启 OpenCode 即可使用</div>
        </div>
      </div>

      <!-- OpenClaw -->
      <div v-if="activeTab === 'openclaw'" class="config-content">
        <div class="config-header">
          <h3>OpenClaw</h3>
          <span class="config-badge">AI 编程助手</span>
        </div>
        <p class="config-desc">OpenClaw MCP 配置：</p>
        <pre class="code-block"><code>{{ openclawConfig }}</code></pre>
        <div class="config-steps">
          <div class="step"><span class="step-num">1</span>编辑 ~/.openclaw/config.json</div>
          <div class="step"><span class="step-num">2</span>支持通过 env 传入禅道连接配置</div>
          <div class="step"><span class="step-num">3</span>重启 OpenClaw 即可使用</div>
        </div>
      </div>

      <!-- Codex -->
      <div v-if="activeTab === 'codex'" class="config-content">
        <div class="config-header">
          <h3>Codex</h3>
          <span class="config-badge">OpenAI CLI</span>
        </div>
        <p class="config-desc">OpenAI Codex CLI MCP 配置：</p>
        <pre class="code-block"><code>{{ codexConfig }}</code></pre>
        <div class="config-steps">
          <div class="step"><span class="step-num">1</span>编辑 ~/.codex/config.json</div>
          <div class="step"><span class="step-num">2</span>或使用 <code>codex mcp add</code> 命令</div>
          <div class="step"><span class="step-num">3</span>重启 Codex 即可使用</div>
        </div>
      </div>

      <!-- Qoder -->
      <div v-if="activeTab === 'qoder'" class="config-content">
        <div class="config-header">
          <h3>Qoder</h3>
          <span class="config-badge">AI 编程平台</span>
        </div>
        <p class="config-desc">Qoder MCP 配置：</p>
        <pre class="code-block"><code>{{ qoderConfig }}</code></pre>
        <div class="config-steps">
          <div class="step"><span class="step-num">1</span>编辑 ~/.qoder/config.json 或 .qoder/mcp.json</div>
          <div class="step"><span class="step-num">2</span>支持插件方式集成</div>
          <div class="step"><span class="step-num">3</span>重启 Qoder 即可使用</div>
        </div>
      </div>

      <!-- Trae -->
      <div v-if="activeTab === 'trae'" class="config-content">
        <div class="config-header">
          <h3>Trae</h3>
          <span class="config-badge">字节跳动 AI IDE</span>
        </div>
        <p class="config-desc">在 Trae 中配置 MCP Server：</p>
        <pre class="code-block"><code>{{ traeConfig }}</code></pre>
        <div class="config-steps">
          <div class="step"><span class="step-num">1</span>打开 Trae 设置 → MCP Servers</div>
          <div class="step"><span class="step-num">2</span>添加 stdio 类型的 MCP Server</div>
          <div class="step"><span class="step-num">3</span>填入命令和参数</div>
        </div>
      </div>

      <!-- 通用 CLI -->
      <div v-if="activeTab === 'cli'" class="config-content">
        <div class="config-header">
          <h3>命令行 / 通用</h3>
          <span class="config-badge">适用于任何支持 stdio 的工具</span>
        </div>
        <p class="config-desc">通过命令行直接调用：</p>
        <pre class="code-block"><code>{{ cliExample }}</code></pre>
      </div>
    </section>

    <!-- 运行时模式管理（热重载） -->
    <section class="section">
      <h2 class="section-title">运行时模式管理</h2>
      <p class="section-desc">MCP 子系统支持运行时热重载，无需重启进程即可切换开关、只读模式、Token 与 action 白名单。通过以下管理 API 操作（仅 HTTP 模式可用）。</p>

      <div class="api-table">
        <div class="api-row header">
          <span class="api-method">方法</span>
          <span class="api-path">路径</span>
          <span class="api-desc">说明</span>
        </div>
        <div class="api-row">
          <span class="api-method get">GET</span>
          <span class="api-path"><code>/api/v1/mcp/status</code></span>
          <span class="api-desc">查询当前模式状态（轻量探测）</span>
        </div>
        <div class="api-row">
          <span class="api-method get">GET</span>
          <span class="api-path"><code>/api/v1/mcp/config</code></span>
          <span class="api-desc">查询当前配置快照（Token 不返回明文）</span>
        </div>
        <div class="api-row">
          <span class="api-method put">PUT</span>
          <span class="api-path"><code>/api/v1/mcp/config</code></span>
          <span class="api-desc">部分热重载（enabled / readOnly / token / actions）</span>
        </div>
      </div>

      <div class="code-card">
        <h3 class="code-title">热重载示例</h3>
        <pre class="code-block"><code># 临时禁用 MCP 服务
curl -X PUT http://localhost:12345/api/v1/mcp/config \
  -H "Content-Type: application/json" \
  -d '{"enabled": false}'

# 设置 Token 鉴权（立即生效，无需重启）
curl -X PUT http://localhost:12345/api/v1/mcp/config \
  -H "Content-Type: application/json" \
  -d '{"token": "your-secret-token"}'

# 限制仅暴露部分 action（白名单）
curl -X PUT http://localhost:12345/api/v1/mcp/config \
  -H "Content-Type: application/json" \
  -d '{"actions": ["get_bugs", "get_tasks"]}'

# 关闭鉴权（token 设为空串）
curl -X PUT http://localhost:12345/api/v1/mcp/config \
  -H "Content-Type: application/json" \
  -d '{"token": ""}'</code></pre>
      </div>
      <p class="config-desc" style="margin-top: 8px; color: #DC2626;">
        ⚠️ 安全提示：管理 API 当前未加额外鉴权，生产环境请配合反向代理鉴权或网络隔离，避免公网裸暴露。
      </p>
    </section>

    <!-- HTTP API 对接 -->
    <section class="section">
      <h2 class="section-title">HTTP REST API 对接</h2>
      <p class="section-desc">启动 HTTP 服务后，通过 REST API 对接。默认端口 <code>12345</code>。</p>

      <!-- 连接测试 -->
      <div class="conn-test-card">
        <div class="conn-test-header">
          <h3 class="code-title">连接测试</h3>
          <button class="conn-test-btn" :class="{ 'testing': connTesting, 'success': connStatus === 'ok', 'error': connStatus === 'error' }" @click="testConnection" :disabled="connTesting">
            <span v-if="connTesting" class="spinner"></span>
            <span v-else-if="connStatus === 'ok'">✓</span>
            <span v-else-if="connStatus === 'error'">✗</span>
            <span>{{ connTesting ? '测试中...' : connStatus === 'ok' ? '已连接' : connStatus === 'error' ? '连接失败' : '测试连接' }}</span>
          </button>
        </div>
        <div v-if="connStatus === 'ok'" class="conn-info">
          <span class="conn-badge success">服务在线</span>
          <span class="conn-detail">版本 {{ connVersion }}</span>
        </div>
        <div v-if="connStatus === 'error'" class="conn-info">
          <span class="conn-badge error">离线</span>
          <span class="conn-detail">{{ connError }}</span>
        </div>
      </div>

      <!-- 可用工具列表（从服务端获取） -->
      <div v-if="tools.length" class="code-card">
        <h3 class="code-title">可用工具 ({{ tools.length }})</h3>
        <div class="tools-grid">
          <div v-for="tool in tools" :key="tool.name" class="tool-chip" @click="showToolDetail(tool)">
            <code>{{ tool.name }}</code>
            <span class="tool-chip-desc">{{ tool.description }}</span>
          </div>
        </div>
      </div>

      <div class="code-card">
        <h3 class="code-title">启动 HTTP 服务</h3>
        <pre class="code-block"><code>cd backend && go run cmd/server/main.go

# 或使用编译后的二进制
./server</code></pre>
      </div>

      <div class="api-table">
        <div class="api-row header">
          <span class="api-method">方法</span>
          <span class="api-path">路径</span>
          <span class="api-desc">说明</span>
        </div>
        <div v-for="api in apiEndpoints" :key="api.path" class="api-row">
          <span class="api-method" :class="api.method.toLowerCase()">{{ api.method }}</span>
          <span class="api-path"><code>{{ api.path }}</code></span>
          <span class="api-desc">{{ api.desc }}</span>
        </div>
      </div>

      <div class="code-card">
        <h3 class="code-title">调用示例</h3>
        <pre class="code-block"><code># ============================================
# MCP HTTP 端点（统一入口）
# ============================================

# POST 方式 - 获取产品列表
curl -X POST http://localhost:12345/mcp \
  -H "Content-Type: application/json" \
  -d '{"action": "get_products"}'

# POST 方式 - 获取 Bug 列表
curl -X POST http://localhost:12345/mcp \
  -H "Content-Type: application/json" \
  -d '{"action": "get_bugs", "params": {"productId": 1, "status": "active"}}'

# GET 方式 - 获取产品列表
curl "http://localhost:12345/mcp?action=get_products"

# GET 方式 - 获取 Bug 列表
curl "http://localhost:12345/mcp?action=get_bugs&productId=1"

# 便捷端点
curl http://localhost:12345/mcp/ping
curl http://localhost:12345/mcp/products
curl "http://localhost:12345/mcp/bugs?productId=1"

# ============================================
# REST API 端点（传统接口）
# ============================================
curl http://localhost:12345/api/products
curl "http://localhost:12345/api/bugs?productId=1&status=active"
curl http://localhost:12345/health</code></pre>
      </div>
    </section>

    <!-- 支持的 MCP Actions -->
    <section class="section">
      <h2 class="section-title">支持的 MCP Actions</h2>
      <div class="capability-grid">
        <div v-for="cap in capabilities" :key="cap.name" class="capability-card">
          <div class="cap-icon" :style="{ color: cap.color }">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="18" height="18">
              <path :d="cap.icon" />
            </svg>
          </div>
          <div class="cap-info">
            <code class="cap-name">{{ cap.name }}</code>
            <span class="cap-desc">{{ cap.desc }}</span>
          </div>
        </div>
      </div>
    </section>

    <!-- 请求/响应格式 -->
    <section class="section">
      <h2 class="section-title">请求/响应格式（两种模式通用）</h2>
      <div class="format-grid">
        <div class="format-card">
          <h4>stdio 请求（stdin）</h4>
          <pre class="code-block compact"><code>{"action": "get_bugs", "params": {"productId": 1}}</code></pre>
        </div>
        <div class="format-card">
          <h4>HTTP POST 请求</h4>
          <pre class="code-block compact"><code>POST /mcp
{"action": "get_bugs", "params": {"productId": 1}}</code></pre>
        </div>
        <div class="format-card">
          <h4>HTTP GET 请求</h4>
          <pre class="code-block compact"><code>GET /mcp?action=get_bugs&productId=1</code></pre>
        </div>
        <div class="format-card">
          <h4>成功响应</h4>
          <pre class="code-block compact success"><code>{"status":"ok","data":[...]}</code></pre>
        </div>
        <div class="format-card">
          <h4>错误响应</h4>
          <pre class="code-block compact error"><code>{"status":"error","message":"..."}</code></pre>
        </div>
        <div class="format-card">
          <h4>Ping 测试</h4>
          <pre class="code-block compact"><code>{"action":"ping"} → {"status":"ok","version":"1.0"}</code></pre>
        </div>
      </div>
    </section>

    <!-- HTTP MCP 端点 -->
    <section class="section">
      <h2 class="section-title">HTTP MCP 端点</h2>
      <p class="section-desc">启动 HTTP 服务后，通过以下端点调用 MCP。统一入口 <code>/mcp</code>，也支持便捷端点。</p>
      <div class="api-table">
        <div class="api-row header">
          <span class="api-method">方法</span>
          <span class="api-path">端点</span>
          <span class="api-desc">说明</span>
        </div>
        <div v-for="api in mcpEndpoints" :key="api.path" class="api-row">
          <span class="api-method" :class="api.method.toLowerCase()">{{ api.method }}</span>
          <span class="api-path"><code>{{ api.path }}</code></span>
          <span class="api-desc">{{ api.desc }}</span>
        </div>
      </div>
    </section>

    <!-- 多语言代码示例 -->
    <section class="section">
      <h2 class="section-title">多语言对接示例</h2>
      <div v-for="lang in languages" :key="lang.name" class="code-card">
        <h3 class="code-title">{{ lang.name }}</h3>
        <pre class="code-block"><code>{{ lang.code }}</code></pre>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import api from '@/api/api'
import type { ApiResponse } from '@/types/api'
import { ElMessageBox } from 'element-plus'

const activeTab = ref('claude')

// 版本 & 连接状态
const versionInfo = ref<Record<string, string>>({})
const httpStatus = ref('检测中...')
const httpStatusClass = ref('checking')
const connectionUrl = ref('')

// MCP 模式状态（从 /api/v1/mcp/status 获取）
interface MCPStatusInfo {
  enabled: boolean | null
  transport: string | null
  readOnly: boolean | null
  tokenSet: boolean | null
  actionAll: boolean | null
  actions: string[]
}
const mcpStatus = ref<MCPStatusInfo>({
  enabled: null,
  transport: null,
  readOnly: null,
  tokenSet: null,
  actionAll: null,
  actions: []
})
const mcpEnabledText = computed(() => {
  if (mcpStatus.value.enabled === null) return '检测中...'
  return mcpStatus.value.enabled ? '已启用' : '已禁用'
})
const mcpEnabledClass = computed(() => {
  if (mcpStatus.value.enabled === null) return 'checking'
  return mcpStatus.value.enabled ? 'online' : 'offline'
})

async function fetchMCPStatus() {
  try {
    const res = await fetch(`${baseUrl}/api/v1/mcp/status`)
    const json = await res.json()
    // 兼容 errors.Response({code,message,data}) 与裸 {status,...} 两种格式
    const d = json.data || json
    if (d && typeof d === 'object') {
      mcpStatus.value = {
        enabled: d.enabled ?? null,
        transport: d.transport ?? null,
        readOnly: d.readOnly ?? null,
        tokenSet: d.tokenSet ?? null,
        actionAll: d.actionAll ?? null,
        actions: d.actions || []
      }
    }
  } catch {
    // 优雅降级：状态保持 null（显示 ...）
  }
}

onMounted(async () => {
  connectionUrl.value = window.location.origin

  try {
    const res = await api.get('/version') as ApiResponse<Record<string, string>>
    if (res?.data) versionInfo.value = res.data
  } catch {}

  try {
    const res = await fetch('/mcp', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ action: 'ping' })
    })
    const json = await res.json()
    httpStatus.value = json.status === 'ok' ? '在线' : '异常'
    httpStatusClass.value = json.status === 'ok' ? 'online' : 'error'
  } catch {
    httpStatus.value = '离线'
    httpStatusClass.value = 'offline'
  }

  // 拉取 MCP 模式状态
  fetchMCPStatus()
})
const connTesting = ref(false)
const connStatus = ref<string | null>(null)
const connVersion = ref('')
const connError = ref('')
const tools = ref<any[]>([])

const baseUrl = typeof window !== 'undefined' ? window.location.origin : 'http://localhost:12345'

async function testConnection() {
  connTesting.value = true
  connError.value = ''
  try {
    const res = await fetch(`${baseUrl}/mcp/ping`)
    const data = await res.json()
    if (data.status === 'ok') {
      connStatus.value = 'ok'
      connVersion.value = data.version || '1.0'
      // 同时拉取 tools
      fetchTools()
    } else {
      connStatus.value = 'error'
      connError.value = data.message || 'Unknown error'
    }
  } catch (e: any) {
    connStatus.value = 'error'
    connError.value = e.message || 'Network error'
  } finally {
    connTesting.value = false
  }
}

async function fetchTools() {
  try {
    const res = await fetch(`${baseUrl}/mcp/tools`)
    const data = await res.json()
    if (data.status === 'ok') {
      tools.value = data.tools || []
    }
  } catch {}
}

function showToolDetail(tool: any) {
  const params = Object.entries(tool.inputSchema?.properties || {}).map(([k, v]: any) => `  ${k}: ${v.description || v.type}`).join('\n') || '  (无参数)'
  ElMessageBox.alert(
    `<pre style="white-space: pre-wrap; font-family: monospace; font-size: 13px;">参数:\n${params}</pre>`,
    tool.name,
    { dangerouslyUseHTMLString: true, confirmButtonText: '关闭' }
  )
}

// 页面加载时自动测试
onMounted(() => {
  testConnection()
})

const configTabs = [
  { id: 'claude', label: 'Claude Desktop' },
  { id: 'claudecode', label: 'Claude Code' },
  { id: 'cursor', label: 'Cursor' },
  { id: 'opencode', label: 'OpenCode' },
  { id: 'openclaw', label: 'OpenClaw' },
  { id: 'codex', label: 'Codex' },
  { id: 'qoder', label: 'Qoder' },
  { id: 'trae', label: 'Trae' },
  { id: 'cli', label: '命令行' }
]

const claudeConfig = `{
  "mcpServers": {
    "zentao-max": {
      "command": "/path/to/zentao-max-mcp",
      "args": [],
      "env": {
        // 设置后所有 MCP 调用需携带此 Token（可选）
        "ZENTAO_MAX_MCP_TOKEN": "your-secret-token"
      }
    }
  }
}`

const claudeCodeConfig = `# 方式 1：通过 claude mcp add 命令（推荐）
claude mcp add zentao-max /path/to/zentao-max-mcp

# 方式 2：手动编辑配置文件 ~/.claude/claude_desktop_config.json
{
  "mcpServers": {
    "zentao-max": {
      "command": "/path/to/zentao-max-mcp",
      "args": [],
      "env": { "ZENTAO_MAX_MCP_TOKEN": "your-secret-token" }
    }
  }
}

# 方式 3：项目级别配置 .claude/settings.json
{
  "mcpServers": {
    "zentao-max": {
      "command": "/path/to/zentao-max-mcp",
      "args": [],
      "env": {}
    }
  }
}

# 验证 MCP Server 是否添加成功
claude mcp list

# 移除 MCP Server
claude mcp remove zentao-max`

const cursorConfig = `{
  "mcpServers": {
    "zentao-max": {
      "command": "/path/to/zentao-max-mcp",
      "args": [],
      "env": {}
    }
  }
}`

const openCodeConfig = `# OpenCode MCP 配置
# 配置文件位置：~/.opencode/config.json 或项目根目录 .opencode/config.json

{
  "mcpServers": {
    "zentao-max": {
      "command": "/path/to/zentao-max-mcp",
      "args": [],
      "env": {}
    }
  }
}

# 或使用 CLI 命令添加
opencode mcp add zentao-max /path/to/zentao-max`

const openclawConfig = `# OpenClaw MCP 配置
# 配置文件位置：~/.openclaw/config.json

{
  "mcpServers": {
    "zentao-max": {
      "command": "/path/to/zentao-max-mcp",
      "args": [],
      "env": {}
    }
  }
}

# 环境变量方式（可选）
# 如果禅道 Max 需要连接远程禅道服务，可通过 env 传入配置
{
  "mcpServers": {
    "zentao-max": {
      "command": "/path/to/zentao-max-mcp",
      "args": [],
      "env": {
        "ZENTAO_URL": "https://your-ZENTAO_DOMAIN",
        "ZENTAO_TOKEN": "your-api-token"
      }
    }
  }
}`

const codexConfig = `# OpenAI Codex CLI MCP 配置
# 配置文件位置：~/.codex/config.json 或 codex.json

{
  "mcpServers": {
    "zentao-max": {
      "command": "/path/to/zentao-max-mcp",
      "args": [],
      "env": {}
    }
  }
}

# Codex 也支持通过 CLI 添加
codex mcp add zentao-max -- /path/to/zentao-max`

const qoderConfig = `# Qoder MCP 配置
# 配置文件位置：~/.qoder/config.json 或项目根目录 .qoder/mcp.json

{
  "mcpServers": {
    "zentao-max": {
      "command": "/path/to/zentao-max-mcp",
      "args": [],
      "env": {}
    }
  }
}

# Qoder 插件方式
class ZentaoPlugin {
  name = "zentao-max";
  version = "1.0.0";

  async start() {
    return this.spawn("./zentao-max-mcp");
  }

  async call(action, params = {}) {
    return this.mcpCall(action, params);
  }
}`

const traeConfig = `{
  "name": "zentao-max",
  "type": "stdio",
  "command": "/path/to/zentao-max-mcp",
  "args": [],
  "env": { "ZENTAO_MAX_MCP_TOKEN": "your-secret-token" }
}`

const cliExample = `# 发送 ping 请求
echo '{"action": "ping"}' | /path/to/zentao-max-mcp

# 获取产品列表
echo '{"action": "get_products"}' | /path/to/zentao-max-mcp

# 获取 Bug 列表
echo '{"action": "get_bugs", "params": {"productId": 1}}' | /path/to/zentao-max-mcp

# 配置 Token 后需在 params 中携带
echo '{"action": "ping", "params": {"token": "xxx"}}' | /path/to/zentao-max-mcp

# Python 调用
import subprocess, json
proc = subprocess.Popen(["/path/to/zentao-max-mcp"],
    stdin=subprocess.PIPE, stdout=subprocess.PIPE, text=True)
proc.stdin.write(json.dumps({"action": "ping"}) + "\\n")
proc.stdin.flush()
print(proc.stdout.readline())`

const apiEndpoints = [
  { method: 'GET', path: '/api/products', desc: '获取产品列表' },
  { method: 'GET', path: '/api/projects', desc: '获取项目列表' },
  { method: 'GET', path: '/api/executions', desc: '获取执行/迭代列表' },
  { method: 'GET', path: '/api/bugs', desc: '获取 Bug 列表' },
  { method: 'GET', path: '/api/stories', desc: '获取需求列表' },
  { method: 'GET', path: '/api/tasks', desc: '获取任务列表' },
  { method: 'GET', path: '/api/users', desc: '获取用户列表' },
  { method: 'GET', path: '/api/timelog/dashboard', desc: '工时统计面板' },
  { method: 'GET', path: '/api/timelog/efforts', desc: '工时流水明细' },
  { method: 'GET', path: '/health', desc: '健康检查' }
]

const mcpEndpoints = [
  { method: 'GET', path: '/mcp/tools', desc: '列出所有可用工具' },
  { method: 'GET', path: '/mcp/tools/:name', desc: '获取单个工具详情' },
  { method: 'POST', path: '/mcp', desc: 'MCP 统一入口（推荐）' },
  { method: 'GET', path: '/mcp?action=...', desc: 'MCP 统一入口（GET）' },
  { method: 'POST', path: '/mcp/ping', desc: 'Ping 测试' },
  { method: 'POST', path: '/mcp/products', desc: '获取产品列表' },
  { method: 'POST', path: '/mcp/projects', desc: '获取项目列表' },
  { method: 'POST', path: '/mcp/bugs', desc: '获取 Bug 列表' },
  { method: 'POST', path: '/mcp/stories', desc: '获取需求列表' },
  { method: 'POST', path: '/mcp/tasks', desc: '获取任务列表' },
  { method: 'POST', path: '/mcp/users', desc: '获取用户列表' },
  { method: 'POST', path: '/mcp/timelog', desc: '获取工时数据' },
  { method: 'GET', path: '/mcp/ping', desc: 'Ping（GET）' },
  { method: 'GET', path: '/mcp/products', desc: '产品列表（GET）' },
  { method: 'GET', path: '/mcp/bugs?productId=1', desc: 'Bug 列表（GET）' }
]

interface Capability {
  name: string
  desc: string
  icon: string
  color: string
}

const capabilities: Capability[] = [
  { name: 'ping', desc: '测试服务状态', icon: 'M13 10V3L4 14h7v7l9-11h-7z', color: '#22C55E' },
  { name: 'get_products', desc: '获取产品列表', icon: 'M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4', color: '#4F6BF6' },
  { name: 'get_projects', desc: '获取项目列表', icon: 'M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z', color: '#F59E0B' },
  { name: 'get_executions', desc: '获取执行/迭代列表', icon: 'M13 10V3L4 14h7v7l9-11h-7z', color: '#6B7280' },
  { name: 'get_bugs', desc: '获取 Bug 列表', icon: 'M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z', color: '#EF4444' },
  { name: 'get_stories', desc: '获取需求列表', icon: 'M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253', color: '#4F6BF6' },
  { name: 'get_tasks', desc: '获取任务列表', icon: 'M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2', color: '#22C55E' },
  { name: 'get_users', desc: '获取用户列表', icon: 'M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z', color: '#F59E0B' },
  { name: 'get_timelog', desc: '获取工时数据', icon: 'M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z', color: '#6B7280' }
]

interface Language {
  name: string
  code: string
}

const languages: Language[] = [
  {
    name: 'Python',
    code: `import subprocess
import json

class ZentaoMCP:
    def __init__(self, binary_path="./zentao-max-mcp"):
        self.process = subprocess.Popen(
            [binary_path],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True
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

# 使用示例
mcp = ZentaoMCP()
print(mcp.call("ping"))
print(mcp.call("get_products"))
print(mcp.call("get_bugs", {"productId": 1}))
mcp.close()`
  },
  {
    name: 'Node.js',
    code: `const { spawn } = require('child_process');

class ZentaoMCP {
  constructor(binaryPath = './zentao-max-mcp') {
    this.process = spawn(binaryPath);
  }

  call(action, params = {}) {
    return new Promise((resolve, reject) => {
      const request = { action, params };
      this.process.stdin.write(JSON.stringify(request) + '\\n');

      const handler = (data) => {
        try {
          resolve(JSON.parse(data.toString()));
        } catch (e) {
          reject(e);
        }
        this.process.stdout.removeListener('data', handler);
      };

      this.process.stdout.on('data', handler);
      this.process.stderr.on('data', (data) => {
        reject(new Error(data.toString()));
      });
    });
  }

  close() {
    this.process.stdin.end();
    this.process.kill();
  }
}

// 使用示例
const mcp = new ZentaoMCP();
mcp.call('ping').then(console.log);
mcp.call('get_bugs', { productId: 1 }).then(console.log);`
  },
  {
    name: 'Go',
    code: `package main

import (
    "bufio"
    "encoding/json"
    "fmt"
    "os/exec"
)

type MCPRequest struct {
    Action string                 \`json:"action"\`
    Params map[string]interface{} \`json:"params,omitempty"\`
}

type MCPResponse struct {
    Status  string      \`json:"status"\`
    Message string      \`json:"message"\`
    Data    interface{} \`json:"data,omitempty"\`
}

func main() {
    cmd := exec.Command("./zentao-max-mcp")
    stdin, _ := cmd.StdinPipe()
    stdout, _ := cmd.StdoutPipe()
    cmd.Start()

    // 发送 ping 请求
    req := MCPRequest{Action: "ping"}
    reqJSON, _ := json.Marshal(req)
    stdin.Write(append(reqJSON, '\\n'))
    stdin.Close()

    // 读取响应
    scanner := bufio.NewScanner(stdout)
    if scanner.Scan() {
        var resp MCPResponse
        json.Unmarshal(scanner.Bytes(), &resp)
        fmt.Printf("Status: %s, Message: %s\\n", resp.Status, resp.Message)
    }

    cmd.Wait()
}`
  },
  {
    name: 'Bash / cURL (HTTP 模式)',
    code: `# 先启动 HTTP 服务
cd backend && go run cmd/server/main.go &

# 获取产品列表
curl -s http://localhost:12345/api/products | jq .

# 获取 Bug 列表
curl -s "http://localhost:12345/api/bugs?productId=1" | jq .

# 获取需求列表
curl -s "http://localhost:12345/api/stories?productId=1" | jq .

# 获取任务列表
curl -s "http://localhost:12345/api/tasks?productId=1" | jq .

# 获取工时统计
curl -s "http://localhost:12345/api/timelog/dashboard?productId=1&dateFrom=2024-03-01&dateTo=2024-03-31" | jq .`
  }
]
</script>

<style scoped>
.mcp-guide {
  max-width: 960px;
  margin: 0 auto;
}

/* Status Card */
.status-card {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 12px;
  padding: 20px;
  background: var(--color-bg-card);
  border-radius: var(--radius-md);
  border: 1px solid var(--color-border-light);
}

.status-row {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.status-label {
  font-size: 12px;
  color: var(--color-text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.status-value {
  font-size: 14px;
  color: var(--color-text-primary);
  font-weight: 500;
}

.status-value.mono {
  font-family: monospace;
}

.version-tag {
  display: inline-block;
  padding: 2px 8px;
  background: var(--color-primary);
  color: white;
  border-radius: 4px;
  font-size: 13px;
  font-weight: 600;
  width: fit-content;
}

.status-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-right: 6px;
  vertical-align: middle;
}

.status-value.online .status-dot { background: #22c55e; }
.status-value.checking .status-dot { background: #eab308; }
.status-value.offline .status-dot { background: #ef4444; }
.status-value.error .status-dot { background: #ef4444; }

.connection-url {
  font-size: 13px;
  background: var(--color-primary-light);
  padding: 4px 10px;
  border-radius: 4px;
  width: fit-content;
  word-break: break-all;
}

.page-title {
  font-size: 26px;
  font-weight: 700;
  color: var(--color-text-primary);
  margin-bottom: 24px;
}

.info-banner {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 16px 20px;
  background: var(--color-primary-light);
  border-radius: var(--radius-md);
  color: var(--color-primary);
  font-size: 13px;
  line-height: 1.6;
  margin-bottom: 32px;
}

.info-banner svg { flex-shrink: 0; margin-top: 2px; }
.info-banner strong { color: var(--color-primary-dark); }

.section { margin-bottom: 36px; }

.section-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin-bottom: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--color-border-light);
}

.section-desc {
  font-size: 14px;
  color: var(--color-text-secondary);
  margin-bottom: 16px;
}

.section-desc code {
  background: var(--color-bg-hover);
  padding: 2px 6px;
  border-radius: 4px;
  font-family: var(--font-mono);
  font-size: 12px;
}

/* Protocol Cards */
.protocol-cards {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.protocol-card {
  padding: 20px;
  background: var(--color-bg-card);
  border: 2px solid var(--color-border-light);
  border-radius: var(--radius-md);
  position: relative;
}

.protocol-card.active {
  border-color: var(--color-primary);
  background: linear-gradient(135deg, var(--color-primary-light) 0%, var(--color-bg-card) 100%);
}

.protocol-badge {
  position: absolute;
  top: 12px;
  right: 12px;
  background: var(--color-primary);
  color: white;
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 100px;
}

.protocol-card h3 {
  font-size: 15px;
  font-weight: 600;
  margin: 0 0 8px;
  color: var(--color-text-primary);
}

.protocol-card p {
  font-size: 13px;
  color: var(--color-text-secondary);
  margin: 0 0 12px;
  line-height: 1.5;
}

.protocol-meta { display: flex; gap: 6px; flex-wrap: wrap; }

.tag {
  font-size: 11px;
  padding: 3px 8px;
  background: var(--color-bg-hover);
  border-radius: 100px;
  color: var(--color-text-secondary);
}

/* Config Tabs */
.config-tabs {
  display: flex;
  gap: 4px;
  margin-bottom: 20px;
  background: var(--color-bg-hover);
  padding: 4px;
  border-radius: var(--radius-sm);
  width: 100%;
  overflow-x: auto;
}

.config-tab {
  padding: 8px 16px;
  border: none;
  background: transparent;
  border-radius: 4px;
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text-secondary);
  cursor: pointer;
  transition: all var(--transition-fast);
}

.config-tab:hover { color: var(--color-text-primary); }
.config-tab.active {
  background: var(--color-bg-card);
  color: var(--color-primary);
  box-shadow: var(--shadow-sm);
}

.config-content {
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-md);
  padding: 24px;
}

.config-header {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 16px;
}

.config-header h3 {
  font-size: 16px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0;
}

.config-badge {
  font-size: 11px;
  padding: 3px 10px;
  background: var(--color-primary-light);
  color: var(--color-primary);
  border-radius: 100px;
  font-weight: 500;
}

.config-desc {
  font-size: 14px;
  color: var(--color-text-secondary);
  margin: 0 0 12px;
}

.config-desc code {
  background: var(--color-bg-hover);
  padding: 2px 6px;
  border-radius: 4px;
  font-family: var(--font-mono);
  font-size: 12px;
}

.config-steps code {
  background: var(--color-bg-hover);
  padding: 2px 6px;
  border-radius: 4px;
  font-family: var(--font-mono);
  font-size: 12px;
}

.config-path {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--color-text-tertiary);
  background: var(--color-bg);
  padding: 8px 12px;
  border-radius: 4px;
  margin-bottom: 16px;
}

.config-steps {
  margin-top: 20px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.step {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;
  color: var(--color-text-secondary);
}

.step-num {
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: var(--color-primary);
  color: white;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
  font-weight: 600;
  flex-shrink: 0;
}

/* Code Blocks */
.code-block {
  background: var(--color-sidebar);
  color: var(--color-text-on-dark);
  padding: 16px;
  border-radius: var(--radius-sm);
  overflow-x: auto;
  font-family: var(--font-mono);
  font-size: 13px;
  line-height: 1.6;
  margin: 0;
}

/* Connection Test */
.conn-test-card {
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-md);
  padding: 20px;
  margin-bottom: 16px;
}

.conn-test-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.conn-test-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-sm);
  background: var(--color-bg-card);
  color: var(--color-text-secondary);
  font-size: 13px;
  cursor: pointer;
  transition: all var(--transition-fast);
}

.conn-test-btn:hover { border-color: var(--color-primary); color: var(--color-primary); }
.conn-test-btn.success { border-color: #22C55E; color: #22C55E; background: #F0FDF4; }
.conn-test-btn.error { border-color: #EF4444; color: #EF4444; background: #FEF2F2; }
.conn-test-btn:disabled { opacity: 0.7; cursor: not-allowed; }

.spinner {
  width: 14px; height: 14px;
  border: 2px solid var(--color-border-light);
  border-top-color: var(--color-primary);
  border-radius: 50%;
  animation: spin 0.6s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }

.conn-info { display: flex; align-items: center; gap: 10px; margin-top: 8px; }
.conn-badge {
  font-size: 11px; font-weight: 600; padding: 3px 10px;
  border-radius: 100px;
}
.conn-badge.success { background: #F0FDF4; color: #22C55E; }
.conn-badge.error { background: #FEF2F2; color: #EF4444; }
.conn-detail { font-size: 12px; color: var(--color-text-tertiary); }

/* Tools Grid */
.tools-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 8px;
  margin-top: 12px;
}

.tool-chip {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 10px 12px;
  background: var(--color-bg);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: all var(--transition-fast);
}

.tool-chip:hover {
  border-color: var(--color-primary);
  transform: translateY(-1px);
}

.tool-chip code {
  font-family: var(--font-mono);
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.tool-chip-desc {
  font-size: 11px;
  color: var(--color-text-tertiary);
  line-height: 1.4;
}

.code-block.compact { padding: 12px; font-size: 12px; }
.code-block.success code { color: #22C55E; }
.code-block.error code { color: #EF4444; }

.code-card {
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-md);
  padding: 20px;
  margin-bottom: 16px;
}

.code-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 12px;
}

/* API Table */
.api-table {
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-md);
  overflow: hidden;
  margin-bottom: 16px;
}

.api-row {
  display: grid;
  grid-template-columns: 70px 1fr 1fr;
  gap: 16px;
  padding: 12px 20px;
  align-items: center;
  border-bottom: 1px solid var(--color-border-light);
  font-size: 13px;
}

.api-row:last-child { border-bottom: none; }
.api-row.header {
  background: #F8FAFC;
  font-weight: 600;
  color: var(--color-text-secondary);
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.02em;
}

.api-method {
  font-size: 11px;
  font-weight: 600;
  padding: 3px 8px;
  border-radius: 4px;
  text-align: center;
}

.api-method.get {
  background: var(--color-primary-light);
  color: var(--color-primary);
}

.api-path code {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--color-text-primary);
}

.api-desc { color: var(--color-text-secondary); }

/* Format Grid */
.format-grid {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 12px;
}

.format-card {
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-md);
  padding: 16px;
}

.format-card h4 {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-secondary);
  margin: 0 0 10px;
  text-transform: uppercase;
  letter-spacing: 0.02em;
}

/* Capability Grid */
.capability-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 12px;
}

.capability-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  background: var(--color-bg-card);
  border-radius: var(--radius-md);
  border: 1px solid var(--color-border-light);
  transition: all var(--transition-fast);
}

.capability-card:hover {
  transform: translateY(-1px);
  box-shadow: var(--shadow-md);
}

.cap-icon {
  width: 36px;
  height: 36px;
  border-radius: var(--radius-sm);
  background: var(--color-bg);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.cap-info { display: flex; flex-direction: column; gap: 2px; }

.cap-name {
  font-family: var(--font-mono);
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.cap-desc {
  font-size: 12px;
  color: var(--color-text-secondary);
}

@media (max-width: 768px) {
  .protocol-cards,
  .format-grid { grid-template-columns: 1fr; }
  .api-row { grid-template-columns: 60px 1fr; }
  .api-desc { display: none; }
}
</style>

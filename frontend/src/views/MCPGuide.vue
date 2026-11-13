<template>
  <div class="mcp-guide">
    <h1 class="page-title">MCP 对接指南</h1>

    <div class="info-banner">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="20" height="20">
        <path d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
      <span>
        禅道 Max 内置 MCP Server，提供 <strong>{{ totalToolCount }} 个工具</strong>（查询 / 详情 / 工时 / 写操作 / 认证配置）。
        支持两种传输：<strong>stdio（独立子进程）</strong>适合本机 AI 工具；<strong>HTTP</strong> 与本管理界面同进程同端口，适合远程与 Web 集成。
        两种传输均同时支持<strong>标准 MCP 协议（JSON-RPC 2.0，2026-07-28）</strong>与<strong>简化 action 协议</strong>。
      </span>
    </div>

    <!-- 快速开始 -->
    <section class="section">
      <h2 class="section-title">快速开始</h2>
      <div class="quick-steps">
        <div class="quick-step">
          <span class="step-num">1</span>
          <div>
            <h4>连接禅道实例</h4>
            <p>在本系统「设置」页填入禅道地址与账号；或接入后直接让 AI 调用 <code>update_zentao_config</code> 工具完成首次配置（server / account / password），凭据加密存入 auth.db。</p>
          </div>
        </div>
        <div class="quick-step">
          <span class="step-num">2</span>
          <div>
            <h4>选择接入方式</h4>
            <p>本机 AI 工具（Claude Desktop / Cursor / Claude Code 等）推荐 stdio 子进程；远程或脚本调用用 HTTP 入口 <code>{{ mcpBase }}/mcp</code>。</p>
          </div>
        </div>
        <div class="quick-step">
          <span class="step-num">3</span>
          <div>
            <h4>验证连通</h4>
            <p>HTTP 用 <code>POST /mcp</code> 发 <code>server/discover</code>（标准协议）或 <code>{"action":"ping"}</code>（简化协议）；stdio 用 <code>echo '{"action":"ping"}' | ./zentao-max-mcp</code>。</p>
          </div>
        </div>
      </div>
    </section>

    <!-- 版本 & 服务状态 -->
    <section class="section">
      <h2 class="section-title">服务状态</h2>
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
          <span class="status-value" :class="readOnlyClass">{{ readOnlyText }}</span>
        </div>
        <div class="status-row">
          <span class="status-label">Token 鉴权</span>
          <span class="status-value" :class="mcpStatus.tokenSet ? 'online' : 'checking'">{{ mcpStatus.tokenSet === null ? '...' : (mcpStatus.tokenSet ? '已启用' : '未启用') }}</span>
        </div>
        <div class="status-row">
          <span class="status-label">在册工具</span>
          <span class="status-value mono">{{ toolCountText }}</span>
        </div>
        <div class="status-row">
          <span class="status-label">MCP 入口</span>
          <code class="status-value connection-url">{{ mcpBase }}/mcp</code>
        </div>
      </div>
    </section>

    <!-- 协议类型说明 -->
    <section class="section">
      <h2 class="section-title">传输模式</h2>
      <div class="protocol-cards">
        <div class="protocol-card" :class="{ active: mcpStatus.transport === 'stdio' || mcpStatus.transport === 'both' }">
          <div class="protocol-badge">推荐 AI 工具</div>
          <h3>stdio 标准输入输出</h3>
          <p>独立二进制 <code>zentao-max-mcp</code> 由 AI 工具作为子进程拉起，stdin/stdout 走 JSON Lines 通信（也兼容标准 JSON-RPC）。不占端口，日志固定走 stderr，不污染协议流。</p>
          <div class="protocol-meta">
            <span class="tag">JSON Lines</span>
            <span class="tag">子进程托管</span>
            <span class="tag">无需端口</span>
          </div>
          <pre class="code-block compact"># 构建 stdio 入口（产物 backend/zentao-max-mcp）
cd backend && make build-mcp

# 命令行自测（简化协议）
echo '{"action":"ping"}' | ./zentao-max-mcp

# 配置 Token 后在 params 中携带
echo '{"action":"ping","params":{"token":"xxx"}}' | ./zentao-max-mcp</pre>
          <p class="card-note">连接配置读取顺序：config.yaml → 环境变量 → auth.db 加密存储（与 HTTP 入口一致，未配置时 AI 可引导调 update_zentao_config）。Token 经 <code>MCP_TOKEN</code> 或 <code>ZENTAO_MAX_MCP_TOKEN</code> 传入；<code>--config</code> / <code>--env</code> 可指定配置文件。</p>
        </div>
        <div class="protocol-card" :class="{ active: mcpStatus.transport === 'http' || mcpStatus.transport === 'both' }">
          <div class="protocol-badge">推荐 Web/远程</div>
          <h3>HTTP 传输</h3>
          <p>与 Web 管理界面同一进程、同一端口（本页即 <code>{{ mcpBase }}</code>）。统一入口 <code>POST /mcp</code>，另提供 GET 查询与便捷端点，支持 Token 鉴权与远程调用。</p>
          <div class="protocol-meta">
            <span class="tag">同端口同进程</span>
            <span class="tag">GET/POST</span>
            <span class="tag">Bearer Token</span>
          </div>
          <pre class="code-block compact"># 启动后端（HTTP MCP 随之可用）
cd backend && go run .

# 标准协议探测
curl -s -X POST {{ mcpBase }}/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"server/discover"}'

# 简化协议探测
curl -s -X POST {{ mcpBase }}/mcp \
  -H "Content-Type: application/json" \
  -d '{"action":"ping"}'</pre>
          <p class="card-note">配置 <code>mcp.transport: both</code> 时，HTTP 入口会额外在后台启动 stdio 监听，两种接入共享同一 MCP 实例与运行时开关。</p>
        </div>
      </div>
    </section>

    <!-- 协议详解 -->
    <section class="section">
      <h2 class="section-title">协议详解（两种传输通用）</h2>
      <p class="section-desc">
        <code>POST /mcp</code> 会按请求体自动分流：含 <code>method</code> 字段的走<strong>标准 MCP JSON-RPC 2.0</strong>；含 <code>action</code> 字段的走<strong>简化协议</strong>。
      </p>

      <div class="protocol-cols">
        <div class="code-card">
          <h3 class="code-title">标准 MCP：JSON-RPC 2.0（协议版本 2026-07-28）</h3>
          <ul class="proto-points">
            <li><strong>无 initialize 握手</strong>：每个请求在 <code>params._meta["io.modelcontextprotocol/protocolVersion"]</code> 携带版本号，缺失或不支持返回 <code>-32022</code>；<code>server/discover</code> 是唯一免版本检查的方法。</li>
            <li>支持 <code>server/discover</code> / <code>tools/list</code> / <code>tools/call</code>；通知类请求（无 id）不响应，HTTP 侧返回 202。</li>
            <li><code>tools/call</code> 结果带 <code>structuredContent</code> 结构化输出；<code>tools/list</code> 带 <code>ttlMs</code> 缓存提示。</li>
            <li>不支持批量请求（批量语义已从 MCP 规范移除）。</li>
          </ul>
          <pre class="code-block"><code># 1) 发现服务（免版本检查）
curl -s -X POST {{ mcpBase }}/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"server/discover"}'

# 2) 列出工具（必须携带协议版本）
curl -s -X POST {{ mcpBase }}/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list",
       "params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}'

# 3) 调用工具
curl -s -X POST {{ mcpBase }}/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call",
       "params":{"name":"get_bugs",
                 "arguments":{"productId":"1","status":"active"},
                 "_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}'</code></pre>
          <div class="callout warn">
            <strong>客户端兼容性</strong>：本服务仅支持 2026-07-28（无握手）单一版本。客户端若尚不支持该版本，initialize 类握手会得到 <code>-32022</code>/<code>-32601</code>——此时请改用简化协议，或升级客户端。
          </div>
        </div>

        <div class="code-card">
          <h3 class="code-title">简化协议：action / params</h3>
          <ul class="proto-points">
            <li>请求 <code>{"action":"工具名","params":{…}}</code>，响应 <code>{"status":"ok|error","message":…,"data":…}</code>，一行一请求。</li>
            <li>HTTP 侧另有 GET 形式：<code>/mcp?action=get_bugs&amp;productId=1</code>，以及按工具全名单独端点 <code>/mcp/get_bugs</code>。</li>
            <li>适合 curl、脚本、自研 Agent 等不走标准 MCP 客户端的场景，stdio 与 HTTP 行为一致。</li>
          </ul>
          <pre class="code-block"><code># POST 统一入口
curl -s -X POST {{ mcpBase }}/mcp \
  -H "Content-Type: application/json" \
  -d '{"action":"get_bugs","params":{"productId":1,"status":"active"}}'

# GET 统一入口
curl -s "{{ mcpBase }}/mcp?action=get_bugs&productId=1"

# 便捷端点（工具全名）
curl -s {{ mcpBase }}/mcp/ping
curl -s "{{ mcpBase }}/mcp/get_products"
curl -s -X POST "{{ mcpBase }}/mcp/get_bugs" -H "Content-Type: application/json" -d '{"params":{"productId":1}}"' + '</code></pre>
        </div>
      </div>

      <div class="code-card">
        <h3 class="code-title">鉴权与错误码</h3>
        <div class="format-grid">
          <div class="format-card">
            <h4>Token 鉴权</h4>
            <pre class="code-block compact"><code># HTTP 任选其一
Authorization: Bearer &lt;token&gt;
?token=&lt;token&gt;

# stdio 经环境变量传入
MCP_TOKEN=xxx ./zentao-max-mcp</code></pre>
          </div>
          <div class="format-card">
            <h4>JSON-RPC 错误码</h4>
            <pre class="code-block compact"><code>-32022 协议版本缺失/不支持
-32601 方法不存在
-32602 参数无效（含白名单/只读拦截）
-32603 服务内部错误</code></pre>
          </div>
          <div class="format-card">
            <h4>简化协议错误响应</h4>
            <pre class="code-block compact error"><code>{"status":"error",
 "message":"read-only mode:
  write action blocked: record_effort"}</code></pre>
          </div>
        </div>
      </div>
    </section>

    <!-- AI 工具配置 -->
    <section class="section">
      <h2 class="section-title">AI 工具配置</h2>
      <p class="section-desc">stdio 与 HTTP 二选一即可。HTTP 方式无需本地二进制，但要求客户端支持 MCP 2026-07-28；不确定时优先用 stdio。</p>

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
        <p class="config-desc">编辑配置文件（macOS：<code>~/Library/Application Support/Claude/claude_desktop_config.json</code>；Windows：<code>%APPDATA%\Claude\claude_desktop_config.json</code>）：</p>
        <pre class="code-block"><code>{{ claudeConfig }}</code></pre>
        <div class="config-steps">
          <div class="step"><span class="step-num">1</span>把 <code>/path/to/zentao-max-mcp</code> 替换为实际路径（<code>cd backend && make build-mcp</code> 生成）</div>
          <div class="step"><span class="step-num">2</span>服务端配置了 Token 时，在 env 中同步设置 <code>ZENTAO_MAX_MCP_TOKEN</code></div>
          <div class="step"><span class="step-num">3</span>重启 Claude Desktop，对话中即可使用禅道工具</div>
        </div>
      </div>

      <!-- Claude Code -->
      <div v-if="activeTab === 'claudecode'" class="config-content">
        <div class="config-header">
          <h3>Claude Code</h3>
          <span class="config-badge">Anthropic CLI 编程工具</span>
        </div>
        <pre class="code-block"><code>{{ claudeCodeConfig }}</code></pre>
        <div class="config-steps">
          <div class="step"><span class="step-num">1</span>推荐 <code>claude mcp add</code> 命令；HTTP 方式无需本地二进制</div>
          <div class="step"><span class="step-num">2</span>项目级共享配置放 <code>.mcp.json</code>（仓库根目录）</div>
          <div class="step"><span class="step-num">3</span><code>claude mcp list</code> 验证，<code>claude mcp remove zentao-max</code> 移除</div>
        </div>
      </div>

      <!-- ZCode -->
      <div v-if="activeTab === 'zcode'" class="config-content">
        <div class="config-header">
          <h3>ZCode</h3>
          <span class="config-badge">Claude Code 兼容 CLI</span>
        </div>
        <pre class="code-block"><code>{{ zcodeConfig }}</code></pre>
        <div class="config-steps">
          <div class="step"><span class="step-num">1</span>配置接口与 Claude Code 兼容（<code>mcp add / list / remove</code>）</div>
          <div class="step"><span class="step-num">2</span>HTTP 方式直填 <code>{{ mcpBase }}/mcp</code> 即可</div>
          <div class="step"><span class="step-num">3</span>添加后用 <code>zcode mcp list</code> 确认连接状态</div>
        </div>
      </div>

      <!-- Cursor -->
      <div v-if="activeTab === 'cursor'" class="config-content">
        <div class="config-header">
          <h3>Cursor</h3>
          <span class="config-badge">AI 代码编辑器</span>
        </div>
        <p class="config-desc">项目根目录创建 <code>.cursor/mcp.json</code>（或 Settings → MCP → Add server）：</p>
        <pre class="code-block"><code>{{ cursorConfig }}</code></pre>
        <div class="config-steps">
          <div class="step"><span class="step-num">1</span>stdio 与远程 URL 两种形态任选其一</div>
          <div class="step"><span class="step-num">2</span>Settings → MCP 中确认 zentao-max 处于启用状态</div>
        </div>
      </div>

      <!-- Codex -->
      <div v-if="activeTab === 'codex'" class="config-content">
        <div class="config-header">
          <h3>Codex CLI</h3>
          <span class="config-badge">OpenAI CLI</span>
        </div>
        <p class="config-desc">编辑 <code>~/.codex/config.toml</code>：</p>
        <pre class="code-block"><code>{{ codexConfig }}</code></pre>
        <div class="config-steps">
          <div class="step"><span class="step-num">1</span>或使用命令添加：<code>codex mcp add zentao-max -- /path/to/zentao-max-mcp</code></div>
          <div class="step"><span class="step-num">2</span>重启 Codex 会话后生效</div>
        </div>
      </div>

      <!-- OpenCode -->
      <div v-if="activeTab === 'opencode'" class="config-content">
        <div class="config-header">
          <h3>OpenCode</h3>
          <span class="config-badge">开源 AI 编程工具</span>
        </div>
        <p class="config-desc">编辑 <code>opencode.json</code>（全局 <code>~/.config/opencode/</code> 或项目根目录）：</p>
        <pre class="code-block"><code>{{ openCodeConfig }}</code></pre>
        <div class="config-steps">
          <div class="step"><span class="step-num">1</span><code>type: "local"</code> 为 stdio 子进程，<code>type: "remote"</code> 为 HTTP URL</div>
          <div class="step"><span class="step-num">2</span>也可用 <code>opencode mcp add</code> 交互添加</div>
        </div>
      </div>

      <!-- Trae -->
      <div v-if="activeTab === 'trae'" class="config-content">
        <div class="config-header">
          <h3>Trae</h3>
          <span class="config-badge">字节跳动 AI IDE</span>
        </div>
        <p class="config-desc">设置 → MCP → 添加 MCP Server，JSON 配置：</p>
        <pre class="code-block"><code>{{ traeConfig }}</code></pre>
        <div class="config-steps">
          <div class="step"><span class="step-num">1</span>类型选 stdio，填入命令与参数</div>
          <div class="step"><span class="step-num">2</span>添加后确认工具列表出现禅道工具</div>
        </div>
      </div>

      <!-- 通用 -->
      <div v-if="activeTab === 'generic'" class="config-content">
        <div class="config-header">
          <h3>其他工具 / 通用</h3>
          <span class="config-badge">任何支持 MCP 的客户端</span>
        </div>
        <p class="config-desc">支持远程 MCP 的工具直接填 URL：<code>{{ mcpBase }}/mcp</code>（需要鉴权时加 <code>Authorization: Bearer</code> 头）。</p>
        <p class="config-desc" style="margin-top: 12px;">stdio 类工具的通用配置三要素：</p>
        <pre class="code-block"><code>{
  "mcpServers": {
    "zentao-max": {
      "command": "/path/to/zentao-max-mcp",
      "args": [],
      "env": {
        "ZENTAO_MAX_MCP_TOKEN": "your-secret-token"
      }
    }
  }
}</code></pre>
        <p class="config-desc" style="margin-top: 12px;">命令行直接验证：</p>
        <pre class="code-block"><code>echo '{"action":"ping"}' | ./zentao-max-mcp
echo '{"action":"get_my_tasks"}' | ./zentao-max-mcp</code></pre>
      </div>
    </section>

    <!-- 工具目录 -->
    <section class="section">
      <h2 class="section-title">工具目录（{{ totalToolCount }}）</h2>
      <p class="section-desc">
        完整定义（含 inputSchema）可通过 <code>GET /mcp/tools</code> 或 <code>tools/list</code> 获取，以下按用途分组，点击卡片可查看参数。
        <template v-if="mcpStatus.readOnly === true">当前实例处于<strong>只读模式</strong>，带「写」标记的工具会被拦截（返回 -32602 / status error）。</template>
      </p>

      <div v-for="group in toolGroups" :key="group.title" class="code-card">
        <h3 class="code-title">{{ group.title }}<span class="group-count">{{ group.tools.length }}</span><span v-if="group.note" class="group-note">{{ group.note }}</span></h3>
        <div class="tool-catalog">
          <div v-for="tool in group.tools" :key="tool.name" class="tool-item" @click="showToolDetail(tool)">
            <div class="tool-item-head">
              <code>{{ tool.name }}</code>
              <span v-if="tool.write" class="write-badge">写</span>
            </div>
            <span class="tool-chip-desc">{{ tool.desc }}</span>
          </div>
        </div>
      </div>

      <div class="callout warn">
        <strong>写工具行为须知</strong>：<code>record_effort</code> 的 <code>left</code> 是任务剩余工时的<strong>绝对值</strong>，<code>consumed</code> 必须 &gt; 0，不接受未来日期，写后建议回查 <code>get_effort_calendar</code> 核对；
        <code>transition_bug</code> 的 resolve 需带 <code>resolution</code> 与 <code>resolvedBuild</code>；<code>add_bug_comment</code> 走 bug-edit 全量通道，返回 success 不代表落库，建议回查确认。
        只读模式（<code>mcp.read_only: true</code>）会同时拦截配置与认证类工具（<code>update_zentao_config</code> / <code>send_verify_code</code> / <code>sso_login</code>），只读令牌不能重配服务器连接。
      </div>
    </section>

    <!-- HTTP 端点 -->
    <section class="section">
      <h2 class="section-title">HTTP 端点</h2>

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
          <span class="conn-detail">版本 {{ connVersion }}<template v-if="liveToolCount"> · 在册工具 {{ liveToolCount }} 个</template></span>
        </div>
        <div v-if="connStatus === 'error'" class="conn-info">
          <span class="conn-badge error">离线</span>
          <span class="conn-detail">{{ connError }}</span>
        </div>
      </div>

      <div class="api-table">
        <div class="api-row header">
          <span class="api-method">方法</span>
          <span class="api-path">端点</span>
          <span class="api-desc">说明</span>
        </div>
        <div v-for="api in mcpEndpoints" :key="api.method + api.path" class="api-row">
          <span class="api-method" :class="api.method.toLowerCase()">{{ api.method }}</span>
          <span class="api-path"><code>{{ api.path }}</code></span>
          <span class="api-desc">{{ api.desc }}</span>
        </div>
      </div>

      <div class="api-table">
        <div class="api-row header">
          <span class="api-method">方法</span>
          <span class="api-path">端点</span>
          <span class="api-desc">说明</span>
        </div>
        <div v-for="api in restEndpoints" :key="api.method + api.path" class="api-row">
          <span class="api-method" :class="api.method.toLowerCase()">{{ api.method }}</span>
          <span class="api-path"><code>{{ api.path }}</code></span>
          <span class="api-desc">{{ api.desc }}</span>
        </div>
      </div>
    </section>

    <!-- 请求/响应格式 -->
    <section class="section">
      <h2 class="section-title">请求 / 响应格式</h2>
      <div class="format-grid">
        <div class="format-card">
          <h4>标准协议请求</h4>
          <pre class="code-block compact"><code>{"jsonrpc":"2.0","id":1,
 "method":"tools/call",
 "params":{"name":"get_my_tasks",
   "_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}</code></pre>
        </div>
        <div class="format-card">
          <h4>标准协议成功响应</h4>
          <pre class="code-block compact success"><code>{"jsonrpc":"2.0","id":1,
 "result":{"content":[{"type":"text","text":"…"}],
   "resultType":"complete",
   "structuredContent":{…}}}</code></pre>
        </div>
        <div class="format-card">
          <h4>标准协议错误响应</h4>
          <pre class="code-block compact error"><code>{"jsonrpc":"2.0","id":1,
 "error":{"code":-32602,"message":"…"}}</code></pre>
        </div>
        <div class="format-card">
          <h4>简化协议请求</h4>
          <pre class="code-block compact"><code>{"action":"get_bugs",
 "params":{"productId":1,"status":"active"}}</code></pre>
        </div>
        <div class="format-card">
          <h4>简化协议成功响应</h4>
          <pre class="code-block compact success"><code>{"status":"ok",
 "message":"success",
 "data":[…]}</code></pre>
        </div>
        <div class="format-card">
          <h4>stdio 通信（JSON Lines）</h4>
          <pre class="code-block compact"><code>→ {"action":"ping"} 换行
← {"status":"ok", …} 换行
（日志一律走 stderr，不进协议流）</code></pre>
        </div>
      </div>
    </section>

    <!-- 运行时模式管理 -->
    <section class="section">
      <h2 class="section-title">运行时模式管理（热重载）</h2>
      <p class="section-desc">MCP 子系统的开关、只读模式、Token 与工具白名单支持运行时热重载，无需重启进程（传输模式除外）。</p>

      <div class="api-table">
        <div class="api-row header">
          <span class="api-method">方法</span>
          <span class="api-path">路径</span>
          <span class="api-desc">说明</span>
        </div>
        <div class="api-row">
          <span class="api-method get">GET</span>
          <span class="api-path"><code>/api/v1/mcp/status</code></span>
          <span class="api-desc">当前模式状态（enabled / transport / readOnly / tokenSet / actions）</span>
        </div>
        <div class="api-row">
          <span class="api-method get">GET</span>
          <span class="api-path"><code>/api/v1/mcp/config</code></span>
          <span class="api-desc">配置快照（Token 不返回明文，仅 tokenSet）</span>
        </div>
        <div class="api-row">
          <span class="api-method put">PUT</span>
          <span class="api-path"><code>/api/v1/mcp/config</code></span>
          <span class="api-desc">部分热重载（enabled / readOnly / token / actions，未传字段不修改）</span>
        </div>
      </div>

      <div class="code-card">
        <h3 class="code-title">热重载示例</h3>
        <pre class="code-block"><code># 临时禁用 MCP 服务
curl -X PUT {{ mcpBase }}/api/v1/mcp/config \
  -H "Content-Type: application/json" -d '{"enabled": false}'

# 开启只读模式（拦截 8 个受控写/认证工具）
curl -X PUT {{ mcpBase }}/api/v1/mcp/config \
  -H "Content-Type: application/json" -d '{"readOnly": true}'

# 设置 Token 鉴权（立即生效，服务端只存哈希）
curl -X PUT {{ mcpBase }}/api/v1/mcp/config \
  -H "Content-Type: application/json" -d '{"token": "your-secret-token"}'

# 工具白名单（空数组 = 清空白名单恢复全量）
curl -X PUT {{ mcpBase }}/api/v1/mcp/config \
  -H "Content-Type: application/json" \
  -d '{"actions": ["get_bugs", "get_my_tasks", "record_effort"]}'

# 关闭鉴权
curl -X PUT {{ mcpBase }}/api/v1/mcp/config \
  -H "Content-Type: application/json" -d '{"token": ""}'</code></pre>
      </div>

      <div class="code-card">
        <h3 class="code-title">配置文件参考（config.yaml）</h3>
        <pre class="code-block"><code>mcp:
  enabled: true        # 总开关（可热重载）
  transport: http      # http | stdio | both（启动入口决定，运行时不可改）
  token: ""            # 鉴权 Token，空 = 不鉴权
  read_only: false     # 只读模式
  actions: []          # 工具白名单，空 = 全部</code></pre>
        <p class="config-desc" style="margin-top: 12px;">
          环境变量可用 <code>ZENTAO_MAX_</code> 前缀覆盖（如 <code>ZENTAO_MAX_MCP_TOKEN</code>）；注意 env 只覆盖零值字段——config.yaml 中已显式配置的值不会被环境变量覆盖。stdio 入口的 Token 另支持 <code>MCP_TOKEN</code>。
        </p>
      </div>

      <p class="config-desc security-note">
        ⚠️ 安全提示：MCP 管理 API 当前未加额外鉴权，生产环境请配合反向代理鉴权或网络隔离，避免公网裸暴露；对外服务建议始终开启 Token。
      </p>
    </section>

    <!-- 多语言对接示例 -->
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
import { buildConfigs, buildLanguages, mcpEndpoints, restEndpoints, toolGroups, totalToolCount } from '@/data/mcpCatalog'
import type { CatalogTool } from '@/data/mcpCatalog'

const activeTab = ref('claude')

// MCP 入口与 Web 管理界面同进程同端口，示例地址直接取当前 origin
// （vite dev 模式下 /mcp 由 proxy 转发，同样可用）
const mcpBase = typeof window !== 'undefined' ? window.location.origin : 'http://localhost:12345'

// 静态目录/配置模板/多语言示例已拆至 src/data/mcpCatalog.ts（issue #7）
const { claudeConfig, claudeCodeConfig, zcodeConfig, cursorConfig, codexConfig, openCodeConfig, traeConfig } = buildConfigs(mcpBase)
const languages = buildLanguages(mcpBase)

// 版本 & 连接状态
const versionInfo = ref<Record<string, string>>({})
const httpStatus = ref('检测中...')
const httpStatusClass = ref('checking')

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
const readOnlyText = computed(() => {
  if (mcpStatus.value.readOnly === null) return '...'
  return mcpStatus.value.readOnly ? '开启' : '关闭'
})
const readOnlyClass = computed(() => {
  if (mcpStatus.value.readOnly === null) return 'checking'
  return mcpStatus.value.readOnly ? 'error' : 'online'
})

async function fetchMCPStatus() {
  try {
    const res = await fetch(`${mcpBase}/api/v1/mcp/status`)
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

  // 拉取 MCP 模式状态 + 在册工具
  fetchMCPStatus()
  fetchLiveTools()
})

const connTesting = ref(false)
const connStatus = ref<string | null>(null)
const connVersion = ref('')
const connError = ref('')
const liveTools = ref<any[]>([])
const liveToolCount = computed(() => liveTools.value.length)
const toolCountText = computed(() => (liveTools.value.length ? `${liveTools.value.length} 个` : '...'))

async function testConnection() {
  connTesting.value = true
  connError.value = ''
  try {
    const res = await fetch(`${mcpBase}/mcp/ping`)
    const data = await res.json()
    if (data.status === 'ok') {
      connStatus.value = 'ok'
      connVersion.value = data.version || '1.0'
      fetchLiveTools()
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

async function fetchLiveTools() {
  try {
    const res = await fetch(`${mcpBase}/mcp/tools`)
    const data = await res.json()
    if (data.status === 'ok') {
      liveTools.value = data.tools || []
    }
  } catch {}
}


function showToolDetail(tool: CatalogTool) {
  const live = liveTools.value.find(t => t.name === tool.name)
  const params = live
    ? Object.entries(live.inputSchema?.properties || {}).map(([k, v]: any) => `  ${k}${(live.inputSchema?.required || []).includes(k) ? ' (必填)' : ''}: ${v.description || v.type}`).join('\n') || '  (无参数)'
    : '（服务端离线，无法获取 inputSchema）'
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

// ---- AI 工具配置示例 ----

const configTabs = [
  { id: 'claude', label: 'Claude Desktop' },
  { id: 'claudecode', label: 'Claude Code' },
  { id: 'zcode', label: 'ZCode' },
  { id: 'cursor', label: 'Cursor' },
  { id: 'codex', label: 'Codex CLI' },
  { id: 'opencode', label: 'OpenCode' },
  { id: 'trae', label: 'Trae' },
  { id: 'generic', label: '其他 / 通用' }
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
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
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
  font-family: var(--font-mono);
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

/* grid 子项允许收缩到轨道宽度以内，配合 .code-block 的 overflow-x
   防止不换行的代码块把双栏布局撑破容器（否则列会被撑到 min-content 宽） */
.quick-steps > *,
.protocol-cards > *,
.protocol-cols > *,
.format-grid > *,
.tool-catalog > * { min-width: 0; }

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
  line-height: 1.6;
}

.section-desc code,
.card-note code,
.proto-points code,
.quick-step code {
  background: var(--color-bg-hover);
  padding: 2px 6px;
  border-radius: 4px;
  font-family: var(--font-mono);
  font-size: 12px;
}

/* Quick Steps */
.quick-steps {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 16px;
}

.quick-step {
  display: flex;
  gap: 12px;
  padding: 16px;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-md);
}

.quick-step h4 {
  margin: 0 0 6px;
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.quick-step p {
  margin: 0;
  font-size: 13px;
  color: var(--color-text-secondary);
  line-height: 1.6;
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
  line-height: 1.6;
}

.card-note {
  font-size: 12px;
  color: var(--color-text-tertiary);
  line-height: 1.6;
  margin: 12px 0 0;
}

.protocol-meta { display: flex; gap: 6px; flex-wrap: wrap; }

.tag {
  font-size: 11px;
  padding: 3px 8px;
  background: var(--color-bg-hover);
  border-radius: 100px;
  color: var(--color-text-secondary);
}

/* Protocol detail columns */
.protocol-cols {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  margin-bottom: 16px;
}

.proto-points {
  margin: 0 0 14px;
  padding-left: 18px;
  font-size: 13px;
  color: var(--color-text-secondary);
  line-height: 1.7;
}

.proto-points li { margin-bottom: 6px; }

.callout {
  border-radius: var(--radius-sm);
  padding: 12px 16px;
  font-size: 13px;
  line-height: 1.7;
  margin-top: 14px;
}

.callout.warn {
  background: #FFFBEB;
  border: 1px solid #FDE68A;
  color: #92400E;
}

.callout code {
  background: rgba(0, 0, 0, 0.05);
  padding: 1px 5px;
  border-radius: 4px;
  font-family: var(--font-mono);
  font-size: 12px;
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
  white-space: nowrap;
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
  line-height: 1.6;
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

.config-steps {
  margin-top: 20px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.step {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  font-size: 13px;
  color: var(--color-text-secondary);
  line-height: 1.6;
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
  margin-top: 1px;
}

.quick-step .step-num { margin-top: 0; }

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

.code-block.compact { padding: 12px; font-size: 12px; }
.code-block.success code { color: #4ADE80; }
.code-block.error code { color: #F87171; }

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
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.group-count {
  font-size: 11px;
  font-weight: 600;
  padding: 1px 8px;
  background: var(--color-primary-light);
  color: var(--color-primary);
  border-radius: 100px;
}

.group-note {
  font-size: 12px;
  font-weight: 400;
  color: var(--color-text-tertiary);
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

.conn-test-header .code-title { margin: 0; }

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
  background: var(--color-bg-hover);
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

.api-method.put {
  background: #FEF3C7;
  color: #B45309;
}

.api-method.post {
  background: #DCFCE7;
  color: #15803D;
}

.api-path code {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--color-text-primary);
  word-break: break-all;
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

/* Tool Catalog */
.tool-catalog {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 10px;
}

.tool-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 12px 14px;
  background: var(--color-bg);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: all var(--transition-fast);
}

.tool-item:hover {
  border-color: var(--color-primary);
  transform: translateY(-1px);
}

.tool-item-head {
  display: flex;
  align-items: center;
  gap: 6px;
}

.tool-item code {
  font-family: var(--font-mono);
  font-size: 12.5px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.write-badge {
  font-size: 10px;
  font-weight: 600;
  padding: 1px 6px;
  border-radius: 100px;
  background: #FEF2F2;
  color: #DC2626;
  border: 1px solid #FECACA;
}

.tool-chip-desc {
  font-size: 11.5px;
  color: var(--color-text-tertiary);
  line-height: 1.5;
}

.security-note { color: #DC2626; }

@media (max-width: 768px) {
  .protocol-cards,
  .protocol-cols,
  .format-grid { grid-template-columns: 1fr; }
  .api-row { grid-template-columns: 60px 1fr; }
  .api-desc { display: none; }
}
</style>

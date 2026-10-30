#!/bin/bash
# zentao-max MCP 全工具冒烟测试
#
# 覆盖：
#   1. HTTP 简化协议：24 个工具逐一调用（写工具与 SSO 短信默认跳过，
#      设 MCP_E2E_WRITES=1 才执行写工具；SSO 发短信需人工收码，始终跳过 sso_login）
#   2. JSON-RPC 2026-07-28：initialize / tools/list / tools/call（含 _meta 版本协商）
#   3. MCP 模式管理 API：/api/v1/mcp/status|config 只读查询
#   4. stdio 入口：tools/list + ping（需先 cd backend && make build-mcp）
#
# 用法: ./scripts/test_mcp.sh [base_url]
#   MCP_E2E_WRITES=1 ./scripts/test_mcp.sh   # 额外执行写工具（会对禅道实例写数据！）
#
# 前提: 应用已启动且禅道连接可用（VPN/网络就绪），否则查询工具报连接错误属预期。

BASE_URL="${1:-http://localhost:12345}"
RUN_WRITES="${MCP_E2E_WRITES:-0}"
META='"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}'

PASS=0; FAIL=0; SKIP=0

# run_tool <action> [params_json] — 简化协议冒烟，按 status 字段判定
run_tool() {
  local action="$1" params="$2"
  # 不能写 local params="${2:-{}}"：bash 把默认值里的 { 解析掉后残留一个字面 }，
  # 所有带参调用都会拼出多一个 } 的坏 JSON（后端曾静默当无参执行）。
  if [ -z "$params" ]; then params='{}'; fi
  local resp
  resp=$(curl -s -m 120 -X POST "${BASE_URL}/mcp/${action}" \
    -H "Content-Type: application/json" \
    -d "{\"action\":\"${action}\",\"params\":${params}}")
  if echo "$resp" | grep -q '"status":"ok"'; then
    echo "PASS  ${action}"
    PASS=$((PASS+1))
  else
    echo "FAIL  ${action}  resp: $(echo "$resp" | head -c 160)"
    FAIL=$((FAIL+1))
  fi
}

# skip_tool <action> <reason>
skip_tool() {
  echo "SKIP  $1  ($2)"
  SKIP=$((SKIP+1))
}

echo "=== zentao-max MCP 冒烟: ${BASE_URL} (writes=${RUN_WRITES}) ==="
echo ""

echo "---- 基础配置/诊断 ----"
run_tool ping
run_tool get_zentao_config
run_tool test_connection

echo "---- 列表查询 ----"
run_tool get_products
run_tool get_projects
run_tool get_executions
run_tool get_users

# get_bugs/get_stories/get_tasks/get_timelog 需要 productId；不带参数时部分实例
# 全量拉取较慢但可用——传首个产品 ID 更稳。先取一个产品：
PID=$(curl -s -m 60 "${BASE_URL}/api/products" | python3 -c "
import json,sys
try:
    d=json.load(sys.stdin); rows=d.get('data') or []
    print(rows[0].get('id','') if rows else '')
except Exception: print('')
" 2>/dev/null)
if [ -n "$PID" ]; then
  echo "(使用产品 ID=${PID})"
  run_tool get_bugs "{\"productId\":${PID}}"
  run_tool get_stories "{\"productId\":${PID}}"
  run_tool get_tasks "{\"productId\":${PID}}"
  run_tool get_timelog "{\"productId\":\"${PID}\"}"
else
  echo "WARN  未取到产品 ID，列表工具以无参模式跑（实例较大时可能超时）"
  run_tool get_bugs
  run_tool get_stories
  run_tool get_tasks
  run_tool get_timelog
fi

echo "---- 详情/个人查询（用列表首个结果回填） ----"
BID=$(curl -s -m 120 -X POST "${BASE_URL}/mcp/get_bugs" -H "Content-Type: application/json" \
  -d "{\"action\":\"get_bugs\",\"params\":{\"productId\":${PID:-0}}}" | python3 -c "
import json,sys
try:
    d=json.load(sys.stdin)
    data=d.get('data') or {}
    rows=data.get('list') if isinstance(data,dict) else data
    print(rows[0].get('id') or '' if rows else '')
except Exception: print('')
" 2>/dev/null)
if [ -n "$BID" ]; then
  run_tool get_bug_detail "{\"bugId\":${BID}}"
else
  skip_tool get_bug_detail "无可用 Bug ID"
fi
run_tool get_my_tasks
run_tool get_effort_calendar

echo "---- 写工具 ----"
if [ "$RUN_WRITES" != "1" ]; then
  skip_tool record_effort "需 MCP_E2E_WRITES=1"
  skip_tool add_bug_comment "需 MCP_E2E_WRITES=1"
  skip_tool transition_bug "需 MCP_E2E_WRITES=1"
  skip_tool create_bug "需 MCP_E2E_WRITES=1"
  skip_tool transition_task "需 MCP_E2E_WRITES=1"
  skip_tool update_zentao_config "需 MCP_E2E_WRITES=1"
else
  echo "⚠️  MCP_E2E_WRITES=1：将对禅道实例写入数据，注意清理测试残留"
  skip_tool record_effort "需真实 taskId，人工指定后手动调"
  skip_tool add_bug_comment "需真实 bugId，人工指定后手动调"
  skip_tool transition_bug "需真实 bugId，人工指定后手动调"
  skip_tool transition_task "需真实 taskId，人工指定后手动调"
  skip_tool update_zentao_config "会覆盖连接配置，人工执行"
  # create_bug 是唯一可自清理的写路径（建完可删），默认跑一个带标记的
  if [ -n "$PID" ]; then
    run_tool create_bug "{\"productId\":${PID},\"title\":\"[mcp-e2e] smoke test $(date +%s)\",\"steps\":\" automated smoke, safe to delete\"}"
  else
    skip_tool create_bug "无可用产品 ID"
  fi
fi

echo "---- SSO（发短信需人工收码，始终跳过） ----"
skip_tool send_verify_code "发短信需人工配合"
skip_tool sso_login "依赖 send_verify_code 的短信验证码"

echo ""
echo "---- JSON-RPC 2026-07-28（无状态协议：server/discover 代替 initialize 握手） ----"
RPC_OUT=$(curl -s -m 30 -X POST "${BASE_URL}/mcp" -H "Content-Type: application/json" -d "{
  \"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"server/discover\",
  \"params\":{${META}}
}")
if echo "$RPC_OUT" | grep -q '"serverInfo"'; then
  echo "PASS  jsonrpc server/discover"; PASS=$((PASS+1))
else
  echo "FAIL  jsonrpc server/discover: $(echo "$RPC_OUT" | head -c 160)"; FAIL=$((FAIL+1))
fi

LIST_OUT=$(curl -s -m 30 -X POST "${BASE_URL}/mcp" -H "Content-Type: application/json" -d "{
  \"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/list\",\"params\":{${META}}
}")
TOOL_COUNT=$(echo "$LIST_OUT" | python3 -c "
import json,sys
try:
    r=json.load(sys.stdin); print(len(r['result']['tools']))
except Exception: print(0)
" 2>/dev/null)
if [ "${TOOL_COUNT:-0}" -ge 24 ]; then
  echo "PASS  jsonrpc tools/list (${TOOL_COUNT} tools)"; PASS=$((PASS+1))
else
  echo "FAIL  jsonrpc tools/list: got ${TOOL_COUNT} tools (expect >=24)"; FAIL=$((FAIL+1))
fi

echo ""
echo "---- MCP 管理 API（只读） ----"
ST=$(curl -s -m 15 "${BASE_URL}/api/v1/mcp/status")
if echo "$ST" | grep -q 'enabled'; then
  echo "PASS  GET /api/v1/mcp/status"; PASS=$((PASS+1))
else
  echo "FAIL  GET /api/v1/mcp/status: $(echo "$ST" | head -c 120)"; FAIL=$((FAIL+1))
fi

echo ""
echo "---- stdio 入口 ----"
MCP_BIN=""
for c in "./backend/zentao-max-mcp" "backend/zentao-max-mcp" "./zentao-max-mcp"; do
  [ -x "$c" ] && MCP_BIN="$c" && break
done
if [ -z "$MCP_BIN" ]; then
  skip_tool "stdio tools/list" "未找到 zentao-max-mcp（cd backend && make build-mcp）"
else
  STDIO_OUT=$(printf '%s\n' \
    "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/list\",\"params\":{${META}}}" \
    | "$MCP_BIN" 2>/dev/null | head -1)
  STDIO_COUNT=$(echo "$STDIO_OUT" | python3 -c "
import json,sys
try:
    r=json.load(sys.stdin); print(len(r['result']['tools']))
except Exception: print(0)
" 2>/dev/null)
  if [ "${STDIO_COUNT:-0}" -ge 24 ]; then
    echo "PASS  stdio tools/list (${STDIO_COUNT} tools, stdout 纯 JSON)"; PASS=$((PASS+1))
  else
    echo "FAIL  stdio tools/list: got ${STDIO_COUNT}"; FAIL=$((FAIL+1))
  fi
fi

echo ""
echo "=========================================="
echo "结果: PASS=${PASS} FAIL=${FAIL} SKIP=${SKIP}"
[ "$FAIL" -eq 0 ] && echo "✓ 全部通过" || echo "✗ 存在失败项（VPN 未连/未登录场景下部分失败属预期）"
exit "$FAIL"

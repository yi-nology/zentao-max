#!/bin/bash

# 测试MCP服务的HTTP端点
# 用法: ./scripts/test_mcp.sh [base_url]

BASE_URL="${1:-http://localhost:12345}"

echo "Testing MCP endpoints at ${BASE_URL}..."
echo ""

echo "=== Testing MCP ping ==="
curl -s -X POST "${BASE_URL}/mcp/ping" -H "Content-Type: application/json" | head -c 200
echo ""

echo "=== Testing MCP get_products ==="
curl -s -X POST "${BASE_URL}/mcp/products" -H "Content-Type: application/json" | head -c 200
echo ""

echo "=== Testing MCP get_projects ==="
curl -s -X POST "${BASE_URL}/mcp/projects" -H "Content-Type: application/json" | head -c 200
echo ""

echo "=== Testing MCP get_bugs ==="
curl -s -X POST "${BASE_URL}/mcp/bugs" -H "Content-Type: application/json" | head -c 200
echo ""

echo "=== Testing MCP get_stories ==="
curl -s -X POST "${BASE_URL}/mcp/stories" -H "Content-Type: application/json" | head -c 200
echo ""

echo "=== Testing MCP get_tasks ==="
curl -s -X POST "${BASE_URL}/mcp/tasks" -H "Content-Type: application/json" | head -c 200
echo ""

echo "=== Testing health endpoint ==="
curl -s "${BASE_URL}/health" | head -c 200
echo ""

echo ""
echo "========== MCP 模式管理 API =========="

echo "=== Testing GET /api/v1/mcp/status (当前模式状态) ==="
curl -s "${BASE_URL}/api/v1/mcp/status" | head -c 300
echo ""

echo "=== Testing GET /api/v1/mcp/config (当前配置快照) ==="
curl -s "${BASE_URL}/api/v1/mcp/config" | head -c 300
echo ""

echo ""
echo "========== 热重载场景（会修改运行时状态，注意） =========="

echo "=== 设置 Token 鉴权 ==="
curl -s -X PUT "${BASE_URL}/api/v1/mcp/config" \
  -H "Content-Type: application/json" \
  -d '{"token":"test-token-123"}' | head -c 300
echo ""

echo "=== 不带 Token 调用（应 401） ==="
curl -s -o /dev/null -w "HTTP %{http_code}\n" -X POST "${BASE_URL}/mcp/ping"
echo ""

echo "=== 带正确 Token 调用（应 200） ==="
curl -s -X POST "${BASE_URL}/mcp/ping" \
  -H "Authorization: Bearer test-token-123" | head -c 200
echo ""

echo "=== 还原：关闭鉴权 ==="
curl -s -X PUT "${BASE_URL}/api/v1/mcp/config" \
  -H "Content-Type: application/json" \
  -d '{"token":""}' > /dev/null
echo "鉴权已关闭"

echo ""
echo "========== stdio 模式测试（需先 make build-mcp） =========="
MCP_BIN="./zentao-max-mcp"
if [ ! -f "$MCP_BIN" ] && [ -f "backend/zentao-max-mcp" ]; then
  MCP_BIN="backend/zentao-max-mcp"
fi
if [ -f "$MCP_BIN" ]; then
  echo "=== stdio ping（二进制: $MCP_BIN） ==="
  echo '{"action":"ping"}' | "$MCP_BIN" | head -c 200
  echo ""
else
  echo "未找到 zentao-max-mcp，跳过 stdio 测试（先运行 cd backend && make build-mcp）"
fi

package routes

import (
	"net/http"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	hertzassert "github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/common/ut"

	"github.com/yi-nology/zentao-max/backend/internal/config"
	"github.com/yi-nology/zentao-max/backend/internal/mcp"
)

// MCP 简化协议 POST /mcp/<tool> 的访问控制回归测试。
// 起因：HandleActionByNameWithParams 漏调 checkAccess，带 JSON body params 的
// POST /mcp/get_bugs 无 token 也能执行并返回真实数据（v0.9.0 复测确认的鉴权绕过）。
// 本文件钉死：该入口与 JSON-RPC /mcp 端点一样必须过 Token/白名单/只读三道检查。
func setupMCPPostRoutes(t *testing.T) *server.Hertz {
	t.Helper()

	h := server.New(server.WithHostPorts("localhost:0"))
	transport := mcp.NewHTTPTransport(mcp.NewMCPServer(nil))
	registerMCPPostRoutes(h, transport)
	return h
}

func postMCP(h *server.Hertz, path, body, bearer string) *ut.ResponseRecorder {
	req := ut.Body{Body: strings.NewReader(body), Len: len(body)}
	var hdrs []ut.Header
	if body != "" {
		hdrs = append(hdrs, ut.Header{Key: "Content-Type", Value: "application/json"})
	}
	if bearer != "" {
		hdrs = append(hdrs, ut.Header{Key: "Authorization", Value: "Bearer " + bearer})
	}
	return ut.PerformRequest(h.Engine, http.MethodPost, path, &req, hdrs...)
}

// TestMCPPostRoutes_TokenRequired 无 token 时 POST /mcp/<tool>（带/不带 body params）
// 必须统一 401；带正确 token 时放行到业务层（ping → 200）。
func TestMCPPostRoutes_TokenRequired(t *testing.T) {
	mgr := mcp.GetMCPModeManager()
	mgr.InitFromConfig(config.MCPConfig{Enabled: true, Token: "mcp-test-token"})
	t.Cleanup(func() {
		// 还原默认（无 token、非只读），避免污染同包其它测试
		mgr.InitFromConfig(config.MCPConfig{Enabled: true})
	})

	h := setupMCPPostRoutes(t)

	// 1) 无 token + body params（正是此前的绕过形态）
	w := postMCP(h, "/mcp/get_bugs", `{"params":{"productId":1}}`, "")
	hertzassert.DeepEqual(t, http.StatusUnauthorized, w.Result().StatusCode())

	// 2) 无 token + 无 body（query 回退路径，原有检查本就生效）
	w = postMCP(h, "/mcp/get_bugs", "", "")
	hertzassert.DeepEqual(t, http.StatusUnauthorized, w.Result().StatusCode())

	// 3) 无 token + 短别名
	w = postMCP(h, "/mcp/bugs", `{"params":{"productId":1}}`, "")
	hertzassert.DeepEqual(t, http.StatusUnauthorized, w.Result().StatusCode())

	// 4) 正确 token + body params → 放行（ping 不触业务依赖，应 200）
	w = postMCP(h, "/mcp/ping", `{"params":{}}`, "mcp-test-token")
	hertzassert.DeepEqual(t, http.StatusOK, w.Result().StatusCode())

	// 5) 错误 token → 401
	w = postMCP(h, "/mcp/ping", `{"params":{}}`, "wrong-token")
	hertzassert.DeepEqual(t, http.StatusUnauthorized, w.Result().StatusCode())
}

// TestMCPPostRoutes_ReadOnlyBlocked 只读模式下带 token 调写工具也必须被拦（403）。
func TestMCPPostRoutes_ReadOnlyBlocked(t *testing.T) {
	mgr := mcp.GetMCPModeManager()
	mgr.InitFromConfig(config.MCPConfig{Enabled: true, Token: "mcp-test-token", ReadOnly: true})
	t.Cleanup(func() {
		mgr.InitFromConfig(config.MCPConfig{Enabled: true})
	})

	h := setupMCPPostRoutes(t)

	// record_effort 是写工具；只读模式下即使 token 正确也应 403
	w := postMCP(h, "/mcp/record_effort", `{"params":{"taskId":1,"date":"2026-11-12","consumed":1}}`, "mcp-test-token")
	hertzassert.DeepEqual(t, http.StatusForbidden, w.Result().StatusCode())

	// 读工具放行
	w = postMCP(h, "/mcp/ping", `{"params":{}}`, "mcp-test-token")
	hertzassert.DeepEqual(t, http.StatusOK, w.Result().StatusCode())
}

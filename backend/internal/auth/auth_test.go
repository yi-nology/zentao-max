package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/common/ut"
)

// newTestManager 创建测试用 Manager。平台已移除本地账号密码登录，
// 会话令牌一律经 signTokenWithUser 构造（模拟 SSO 登录成功后的签发）。
func newTestManager() *Manager {
	return NewManager("test-secret")
}

func setupRouter(m *Manager) *server.Hertz {
	h := server.New(server.WithHostPorts("localhost:0"))
	h.Use(m.Middleware())
	h.GET("/api/products", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(200, map[string]interface{}{"code": 200})
	})
	h.POST("/api/scheduler/tasks", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(200, map[string]interface{}{"code": 200})
	})
	h.DELETE("/api/logs", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(200, map[string]interface{}{"code": 200})
	})
	m.RegisterRoutes(h.Group("/api"))
	return h
}

func body(t *testing.T, w *ut.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(w.Result().Body(), &m); err != nil {
		t.Fatalf("响应不是合法 JSON: %v, body=%s", err, w.Result().Body())
	}
	return m
}

// doSSOSendCode 打 SSO 发送验证码端点（登录链路里第一个会触发上游调用的步骤）。
func doSSOSendCode(h *server.Hertz, username, password string) *ut.ResponseRecorder {
	payload := fmt.Sprintf(`{"username":%q,"password":%q}`, username, password)
	return ut.PerformRequest(h.Engine, http.MethodPost, "/api/auth/sso/send-code", &ut.Body{Body: strings.NewReader(payload), Len: len(payload)},
		ut.Header{Key: "Content-Type", Value: "application/json"})
}

func TestTokenSignAndVerify(t *testing.T) {
	m := newTestManager()

	token := m.signTokenWithUser("zhangyi01", time.Now().Add(time.Hour))
	assert.True(t, m.Verify(token))
	user, ok := m.VerifyUsername(token)
	assert.True(t, ok)
	assert.DeepEqual(t, "zhangyi01", user)

	// 过期令牌
	expired := m.signTokenWithUser("zhangyi01", time.Now().Add(-time.Hour))
	assert.False(t, m.Verify(expired))
	// 篡改载荷
	parts := strings.SplitN(token, ".", 2)
	assert.False(t, m.Verify(parts[0]+"1."+parts[1]))
	// 垃圾令牌
	assert.False(t, m.Verify("garbage"))
	// 无身份的旧格式令牌（历史密码登录会话）：verifyToken 兼容解析，VerifyUsername 返回空用户名
	legacy := m.signTokenWithUser("", time.Now().Add(time.Hour))
	assert.True(t, m.Verify(legacy))
	user, _ = m.VerifyUsername(legacy)
	assert.DeepEqual(t, "", user)
}

func TestMiddleware_AnonymousReadAllowed(t *testing.T) {
	h := setupRouter(newTestManager())

	w := ut.PerformRequest(h.Engine, http.MethodGet, "/api/products", nil)
	assert.DeepEqual(t, http.StatusOK, w.Result().StatusCode())
}

func TestMiddleware_AnonymousWriteBlocked(t *testing.T) {
	h := setupRouter(newTestManager())

	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/api/scheduler/tasks"},
		{http.MethodDelete, "/api/logs"},
	} {
		w := ut.PerformRequest(h.Engine, tc.method, tc.path, nil)
		assert.DeepEqual(t, http.StatusUnauthorized, w.Result().StatusCode())
		msg := body(t, w)["message"].(string)
		if !strings.Contains(msg, "管理员") {
			t.Errorf("%s %s 的 401 提示应说明需要管理员登录, got: %s", tc.method, tc.path, msg)
		}
	}
}

func TestMiddleware_TokenThenWriteAllowed(t *testing.T) {
	m := newTestManager()
	h := setupRouter(m)

	// 模拟 SSO 登录成功后签发的会话令牌（平台已无密码登录端点）
	token := m.signTokenWithUser("zhangyi01", time.Now().Add(time.Hour))

	// Bearer 头
	w := ut.PerformRequest(h.Engine, http.MethodPost, "/api/scheduler/tasks", nil,
		ut.Header{Key: "Authorization", Value: "Bearer " + token})
	assert.DeepEqual(t, http.StatusOK, w.Result().StatusCode())

	// Cookie
	w = ut.PerformRequest(h.Engine, http.MethodPost, "/api/scheduler/tasks", nil,
		ut.Header{Key: "Cookie", Value: "zentao_max_session=" + token})
	assert.DeepEqual(t, http.StatusOK, w.Result().StatusCode())

	// 坏令牌仍 401
	w = ut.PerformRequest(h.Engine, http.MethodPost, "/api/scheduler/tasks", nil,
		ut.Header{Key: "Authorization", Value: "Bearer bad.token"})
	assert.DeepEqual(t, http.StatusUnauthorized, w.Result().StatusCode())
}

// TestSSOBruteForceGuard 防爆破锁定改走 SSO 通道验证：SSO 上游持续失败达阈值后，
// 即使上游恢复（mock 开始成功）锁定窗口内也拒绝——fail2ban 语义与原密码登录一致。
func TestSSOBruteForceGuard(t *testing.T) {
	upstreamOK := false
	// mock 统一认证平台：/mobiles 在 upstreamOK=false 时返回 500（模拟凭据错误/上游故障）
	ssoSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !upstreamOK {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":200,"data":{"mobile":"186****1498"}}`))
	}))
	defer ssoSrv.Close()

	m := newTestManager()
	m.SetSSO(NewSSOClient(ssoSrv.URL, ""))
	h := setupRouter(m)

	// 连续失败达阈值（loginFailuresPerMinute = 10）
	for i := 0; i < 10; i++ {
		w := doSSOSendCode(h, "zhangyi01", "wrongpw")
		assert.DeepEqual(t, http.StatusUnauthorized, w.Result().StatusCode())
	}
	// 上游恢复后，锁定窗口内仍拒绝
	upstreamOK = true
	w := doSSOSendCode(h, "zhangyi01", "rightpw")
	assert.DeepEqual(t, http.StatusUnauthorized, w.Result().StatusCode())
	// （锁定窗口过后自动恢复的语义由 ratelimit.Lockout 原语自身保证，此处不重复）
}

func TestPasswordLoginRemoved(t *testing.T) {
	// /api/auth/login 路由已删除且不在匿名写豁免清单：无会话请求在 middleware
	// 即被 401 拒绝（到不了路由层，行为等同端点不存在）
	h := setupRouter(newTestManager())
	payload := `{"username":"admin","password":"x"}`
	w := ut.PerformRequest(h.Engine, http.MethodPost, "/api/auth/login", &ut.Body{Body: strings.NewReader(payload), Len: len(payload)},
		ut.Header{Key: "Content-Type", Value: "application/json"})
	assert.DeepEqual(t, http.StatusUnauthorized, w.Result().StatusCode())
}

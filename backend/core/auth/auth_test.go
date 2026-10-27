package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/cloudwego/hertz/pkg/common/ut"
)

func newTestManager() *Manager {
	return NewManager("admin", "secret123", "test-secret")
}

// firstStart 可变的首次启动标志，模拟全新部署
func setupRouter(m *Manager, firstStart *bool) *server.Hertz {
	h := server.New(server.WithHostPorts("localhost:0"))
	h.Use(m.Middleware(func() bool { return firstStart != nil && *firstStart }))
	h.GET("/api/products", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(200, map[string]interface{}{"code": 200})
	})
	h.POST("/api/scheduler/tasks", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(200, map[string]interface{}{"code": 200})
	})
	h.DELETE("/api/logs", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(200, map[string]interface{}{"code": 200})
	})
	h.POST("/api/init/login", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(200, map[string]interface{}{"code": 200})
	})
	h.POST("/api/init/zentaosid", func(ctx context.Context, c *app.RequestContext) {
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

func doLogin(t *testing.T, h *server.Hertz, username, password string) *ut.ResponseRecorder {
	t.Helper()
	payload := `{"username":"` + username + `","password":"` + password + `"}`
	return ut.PerformRequest(h.Engine, http.MethodPost, "/api/auth/login", &ut.Body{Body: strings.NewReader(payload), Len: len(payload)},
		ut.Header{Key: "Content-Type", Value: "application/json"})
}

func TestLoginAndToken(t *testing.T) {
	m := newTestManager()

	token, ok := m.Login("admin", "secret123")
	assert.True(t, ok)
	assert.True(t, m.Verify(token))

	// 错误密码
	if _, ok := m.Login("admin", "wrong"); ok {
		t.Error("错误密码不应登录成功")
	}
	// 过期令牌
	expired := m.signToken(time.Now().Add(-time.Hour))
	assert.False(t, m.Verify(expired))
	// 篡改载荷
	parts := strings.SplitN(token, ".", 2)
	assert.False(t, m.Verify(parts[0]+"1."+parts[1]))
	// 垃圾令牌
	assert.False(t, m.Verify("garbage"))
}

func TestMiddleware_AnonymousReadAllowed(t *testing.T) {
	h := setupRouter(newTestManager(), nil)

	w := ut.PerformRequest(h.Engine, http.MethodGet, "/api/products", nil)
	assert.DeepEqual(t, http.StatusOK, w.Result().StatusCode())
}

func TestMiddleware_AnonymousWriteBlocked(t *testing.T) {
	h := setupRouter(newTestManager(), nil)

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

func TestMiddleware_LoginThenWriteAllowed(t *testing.T) {
	h := setupRouter(newTestManager(), nil)

	w := doLogin(t, h, "admin", "secret123")
	assert.DeepEqual(t, http.StatusOK, w.Result().StatusCode())
	token := body(t, w)["data"].(map[string]interface{})["token"].(string)

	// Bearer 头
	w = ut.PerformRequest(h.Engine, http.MethodPost, "/api/scheduler/tasks", nil,
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

func TestMiddleware_FirstBootInitAnonymous(t *testing.T) {
	m := newTestManager()
	firstStart := true
	h := setupRouter(m, &firstStart)

	for _, path := range []string{"/api/init/login", "/api/init/zentaosid"} {
		w := ut.PerformRequest(h.Engine, http.MethodPost, path, nil)
		assert.DeepEqual(t, http.StatusOK, w.Result().StatusCode())

		// 初始化完成后，同样的写配置必须登录
		firstStart = false
		w = ut.PerformRequest(h.Engine, http.MethodPost, path, nil)
		assert.DeepEqual(t, http.StatusUnauthorized, w.Result().StatusCode())
		firstStart = true
	}
}

func TestLoginWrongPasswordAndBruteForceGuard(t *testing.T) {
	m := newTestManager()
	h := setupRouter(m, nil)

	w := doLogin(t, h, "admin", "wrong")
	assert.DeepEqual(t, http.StatusUnauthorized, w.Result().StatusCode())

	// 连续失败 10 次后触发防爆破：即使密码正确也拒绝
	for i := 0; i < 10; i++ {
		doLogin(t, h, "admin", "wrong")
	}
	lastStatus := doLogin(t, h, "admin", "secret123").Result().StatusCode()
	assert.DeepEqual(t, http.StatusUnauthorized, lastStatus)
}

package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"

	"github.com/yi-nology/zentao-max/backend/internal/auth"
	"github.com/yi-nology/zentao-max/backend/internal/initialization"
)

// ssoTestEnv 组装 handler + 真实 auth.Manager + 测试路由。
// 路由形状与生产一致：GET /api/v1/sso-config、POST /api/v1/sso-config、
// GET /api/auth/sso/status（Manager 热切换的可观测出口）。
type ssoTestEnv struct {
	h          *server.Hertz
	svc        *initialization.InitService
	mgr        *auth.Manager
	handler    *SSOConfigHandler
	swapped    []*auth.SSOClient // onSSOChange 回调记录（nil=停用）
	preflightS *httptest.Server  // 可达性预检用的本地 httptest（nil=不启用）
}

func newSSOTestEnv(t *testing.T, yamlEnabled bool, yamlBaseURL string) *ssoTestEnv {
	t.Helper()
	dir := t.TempDir()
	svc := initialization.NewInitService(filepath.Join(dir, "auth.db"), "sso-test-key")
	mgr := auth.NewManager("admin", "secret123", "test-secret")
	env := &ssoTestEnv{svc: svc, mgr: mgr}

	// 模拟 yaml bootstrap 生效：enabled 时注入客户端
	var bootClient *auth.SSOClient
	if yamlEnabled && yamlBaseURL != "" {
		bootClient = auth.NewSSOClient(yamlBaseURL, "")
		mgr.SetSSO(bootClient)
	}

	handler := NewSSOConfigHandler(svc, mgr, func(c *auth.SSOClient) {
		env.swapped = append(env.swapped, c)
	}, yamlEnabled, yamlBaseURL, "")
	env.handler = handler

	h := server.New(server.WithHostPorts("localhost:0"))
	h.GET("/api/v1/sso-config", handler.GetConfig)
	h.POST("/api/v1/sso-config", handler.UpdateConfig)
	mgr.RegisterRoutes(h.Group("/api"))
	env.h = h
	return env
}

func (e *ssoTestEnv) post(t *testing.T, payload string) map[string]interface{} {
	t.Helper()
	w := ut.PerformRequest(e.h.Engine, http.MethodPost, "/api/v1/sso-config",
		&ut.Body{Body: strings.NewReader(payload), Len: len(payload)},
		ut.Header{Key: "Content-Type", Value: "application/json"})
	var m map[string]interface{}
	if err := json.Unmarshal(w.Result().Body(), &m); err != nil {
		t.Fatalf("响应不是合法 JSON: %v body=%s", err, w.Result().Body())
	}
	return m
}

func (e *ssoTestEnv) getConfig(t *testing.T) map[string]interface{} {
	t.Helper()
	w := ut.PerformRequest(e.h.Engine, http.MethodGet, "/api/v1/sso-config", nil)
	var m map[string]interface{}
	if err := json.Unmarshal(w.Result().Body(), &m); err != nil {
		t.Fatalf("GET 响应不是合法 JSON: %v", err)
	}
	return m["data"].(map[string]interface{})
}

func (e *ssoTestEnv) ssoStatus(t *testing.T) bool {
	t.Helper()
	w := ut.PerformRequest(e.h.Engine, http.MethodGet, "/api/auth/sso/status", nil)
	var m map[string]interface{}
	if err := json.Unmarshal(w.Result().Body(), &m); err != nil {
		t.Fatalf("sso/status 响应不是合法 JSON: %v", err)
	}
	return m["data"].(map[string]interface{})["enabled"].(bool)
}

// 场景 1：数据库无保存记录时回退 yaml bootstrap，source=yaml。
func TestSSOConfig_YamlFallback(t *testing.T) {
	env := newSSOTestEnv(t, true, "https://sso.yaml.example.com")
	data := env.getConfig(t)
	if data["source"] != "yaml" || data["managed"] != false {
		t.Fatalf("want yaml bootstrap, got %v", data)
	}
	if data["baseUrl"] != "https://sso.yaml.example.com" || data["enabled"] != true {
		t.Fatalf("yaml 配置未透出: %v", data)
	}
	if !env.ssoStatus(t) {
		t.Fatal("yaml 启用时 sso/status 应为 true")
	}
}

// 场景 2：显式停用——不需要预检，热切换生效（status 翻 false），持久化 managed=true，
// 且重启解析（Effective=数据库）不会回退到 yaml。
func TestSSOConfig_DisableHotSwap(t *testing.T) {
	env := newSSOTestEnv(t, true, "https://sso.yaml.example.com")
	resp := env.post(t, `{"enabled":false}`)
	if resp["code"] != float64(200) {
		t.Fatalf("停用失败: %v", resp)
	}
	if env.swapped[len(env.swapped)-1] != nil {
		t.Fatal("onSSOChange 应收到 nil（停用）")
	}
	if env.ssoStatus(t) {
		t.Fatal("停用后 sso/status 应立即为 false（热生效）")
	}
	data := env.getConfig(t)
	if data["source"] != "db" || data["managed"] != true || data["enabled"] != false {
		t.Fatalf("停用未持久化为数据库配置: %v", data)
	}
	// 启动解析同源：数据库为准，yaml 的 enabled=true 不会复活
	if eff := env.handler.Effective(); eff.Source != "db" || eff.Enabled {
		t.Fatalf("启动解析应取数据库（停用）: %+v", eff)
	}
}

// 场景 3：启用校验——空 base_url 400；不可达地址 400；都不落盘不切换。
func TestSSOConfig_EnableValidation(t *testing.T) {
	env := newSSOTestEnv(t, false, "")
	// 失败响应的业务码为 40000（errors.BadRequest 包装），断言「非 200 + 有错误文案」
	resp := env.post(t, `{"enabled":true,"baseUrl":""}`)
	if resp["code"] == float64(200) || resp["message"] == "" {
		t.Fatalf("空 base_url 应失败并带文案: %v", resp)
	}
	resp = env.post(t, `{"enabled":true,"baseUrl":"http://127.0.0.1:1"}`)
	if resp["code"] == float64(200) {
		t.Fatalf("不可达 base_url 应失败: %v", resp)
	}
	if env.ssoStatus(t) {
		t.Fatal("校验失败不应热切换")
	}
	if data := env.getConfig(t); data["source"] != "yaml" {
		t.Fatalf("校验失败不应落盘: %v", data)
	}
}

// 场景 4：启用成功——预检通过（本地 httptest）→ 热切换 → 合并持久化（不抹禅道字段）。
func TestSSOConfig_EnableSuccessAndMerge(t *testing.T) {
	env := newSSOTestEnv(t, false, "")
	// 预检目标：本地 httptest server（必然可达）
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()

	// 预置禅道连接字段（模拟已有 auth.db 记录）
	if err := env.svc.UpdateAuthConfig(func(cfg *initialization.AuthConfig) {
		cfg.Username = "zhangsan"
		cfg.Password = "pw"
		cfg.Domain = "https://zentao.example.com"
		cfg.Realm = "kydc"
	}); err != nil {
		t.Fatalf("预置禅道字段失败: %v", err)
	}

	resp := env.post(t, fmt.Sprintf(`{"enabled":true,"baseUrl":%q,"authPageUrl":"%s/sso/auth"}`, ts.URL, ts.URL))
	if resp["code"] != float64(200) {
		t.Fatalf("启用失败: %v", resp)
	}
	if env.ssoStatus(t) != true {
		t.Fatal("启用后 sso/status 应立即为 true（热生效）")
	}
	if last := env.swapped[len(env.swapped)-1]; last == nil {
		t.Fatal("onSSOChange 应收到新客户端")
	}
	// 合并持久化：SSO 字段落库且禅道字段完好
	got, _, err := env.svc.LoadAuthConfig()
	if err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	if !got.SSOManaged || !got.SSOEnabled || got.SSOBaseURL != ts.URL {
		t.Errorf("SSO 字段未持久化: %+v", got)
	}
	if got.Username != "zhangsan" || got.Domain != "https://zentao.example.com" {
		t.Errorf("SSO 保存抹掉了禅道字段: %+v", got)
	}
	// GET 反映数据库生效配置
	if data := env.getConfig(t); data["source"] != "db" || data["enabled"] != true {
		t.Fatalf("GET 应反映数据库配置: %v", data)
	}
}

// Package auth 实现平台访问控制：
// 匿名用户只读（GET），登录管理员获得读写权限。
// 会话令牌为 HMAC 签名的无状态 token（exp.signature），放在 HttpOnly Cookie，
// 同时接受 Authorization: Bearer 头（供 API/MCP 客户端使用）。
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route"

	"github.com/yi-nology/zentao-max/backend/core/errors"
)

const (
	sessionCookieName = "zentao_max_session"
	defaultSessionTTL = 7 * 24 * time.Hour
	// 每分钟允许的匿名登录失败次数（防爆破），超过后临时拒绝该 IP
	loginFailuresPerMinute = 10
)

type Manager struct {
	username string
	password string
	secret   []byte
	ttl      time.Duration

	mu       sync.Mutex
	failures map[string]*failureInfo

	// sso 麒麟统一认证登录（用户名+密码+短信验证码）；nil = 未启用
	sso *SSOClient
}

// SetSSO 启用 SSO 登录通道（/api/auth/sso/*）。
func (m *Manager) SetSSO(client *SSOClient) {
	m.sso = client
}

type failureInfo struct {
	count     int
	resetTime time.Time
}

// NewManager 创建认证管理器。secret 用于会话令牌签名，
// 建议传入部署时稳定的随机串（如加密密钥派生值），重启后会话不失效。
func NewManager(username, password, secret string) *Manager {
	return &Manager{
		username: username,
		password: password,
		secret:   []byte(secret),
		ttl:      defaultSessionTTL,
		failures: make(map[string]*failureInfo),
	}
}

// Login 校验账号密码，成功返回签名会话令牌。
// 用户名比较非常量时间无碍（用户名本身不是机密），密码用常量时间比较。
func (m *Manager) Login(username, password string) (string, bool) {
	userOK := subtle.ConstantTimeCompare([]byte(username), []byte(m.username)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(password), []byte(m.password)) == 1
	if !userOK || !passOK {
		return "", false
	}
	return m.signToken(time.Now().Add(m.ttl)), true
}

func (m *Manager) signToken(expiry time.Time) string {
	return m.signTokenWithUser("", expiry)
}

// signTokenWithUser 签发带登录者身份的令牌：exp[.username] 分段。
// username 为空时保持旧格式（exp.signature），兼容测试与旧会话。
func (m *Manager) signTokenWithUser(username string, expiry time.Time) string {
	// 分隔符用 ":"（cookie-value 合法字符，浏览器不会百分号编码；
	// 曾用 "|"，Chrome 会把值里的 | 编码为 %7C 回传，导致 Verify 解析失败）
	payload := strconv.FormatInt(expiry.Unix(), 10)
	if username != "" {
		payload += ":" + username
	}
	return payload + "." + hex.EncodeToString(m.mac(payload))
}

func (m *Manager) mac(payload string) []byte {
	h := hmac.New(sha256.New, m.secret)
	h.Write([]byte(payload))
	return h.Sum(nil)
}

// Verify 校验令牌签名与有效期。
func (m *Manager) Verify(token string) bool {
	_, ok := m.verifyToken(token)
	return ok
}

// VerifyUsername 校验令牌并返回登录者用户名（无用户名的旧格式令牌返回空串）。
func (m *Manager) VerifyUsername(token string) (string, bool) {
	return m.verifyToken(token)
}

func (m *Manager) verifyToken(token string) (string, bool) {
	// 浏览器可能把 cookie 值中的分隔符百分号编码（: → %3A；历史 | → %7C），校验前归一化。
	// 签名覆盖原始字节，归一化只影响解析，不弱化校验。
	for _, enc := range []string{"%3A", "%7c", "%7C"} {
		if strings.Contains(token, enc) {
			sep := ":"
			if enc == "%7c" || enc == "%7C" {
				sep = ":"
			}
			token = strings.ReplaceAll(token, enc, sep)
		}
	}
	payload, sig, ok := strings.Cut(token, ".")
	if !ok {
		return "", false
	}
	// payload 形如 "exp" 或 "exp:username"
	expPart := payload
	user := ""
	if exp, usr, has := strings.Cut(payload, ":"); has {
		expPart = exp
		user = usr
	}
	expiry, err := strconv.ParseInt(expPart, 10, 64)
	if err != nil || time.Now().Unix() > expiry {
		return "", false
	}
	expected := m.mac(payload)
	given, err := hex.DecodeString(sig)
	if err != nil {
		return "", false
	}
	if !hmac.Equal(expected, given) {
		return "", false
	}
	return user, true
}

// loginAllowed 供测试断言内部状态
func (m *Manager) loginAllowed(ip string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	info, exists := m.failures[ip]
	if !exists || time.Now().After(info.resetTime) {
		return true
	}
	return info.count < loginFailuresPerMinute
}

func (m *Manager) recordLoginFailure(ip string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	info, exists := m.failures[ip]
	if !exists || time.Now().After(info.resetTime) {
		m.failures[ip] = &failureInfo{count: 1, resetTime: time.Now().Add(time.Minute)}
		return
	}
	info.count++
}

// Middleware 返回 Hertz 中间件：
//   - MCP 端点完全豁免（有自己独立的 mcp.token/只读/白名单访问控制）；
//   - 读方法（GET/HEAD/OPTIONS）匿名放行；
//   - 匿名放行的写路径：登录/登出接口、SSO 两步、报告预览；
//   - 其余写方法需要有效会话（Cookie 或 Bearer 头），否则 401。
func (m *Manager) Middleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		path := string(c.Request.URI().Path())
		// MCP 端点有独立的访问控制（mcp.enabled / mcp.token / read_only / action 白名单），
		// 不受平台登录体系约束：AI 客户端使用 MCP Token 而非平台管理员账号，
		// 且 MCP 调用多为 POST，不能按 HTTP 方法划分权限
		if path == "/mcp" || strings.HasPrefix(path, "/mcp/") {
			c.Next(ctx)
			return
		}

		method := string(c.Request.Method())
		if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
			c.Next(ctx)
			return
		}

		if m.isAnonymousWriteAllowed(path) {
			c.Next(ctx)
			return
		}

		if m.Verify(string(c.Cookie(sessionCookieName))) {
			c.Next(ctx)
			return
		}
		if bearer := string(c.GetHeader("Authorization")); strings.HasPrefix(bearer, "Bearer ") && m.Verify(strings.TrimSpace(bearer[len("Bearer "):])) {
			c.Next(ctx)
			return
		}

		errors.Unauthorized(c, "该操作需要管理员登录（匿名访问为只读模式）")
		c.Abort()
	}
}

func (m *Manager) isAnonymousWriteAllowed(path string) bool {
	switch path {
	case "/api/auth/login", "/api/auth/logout":
		return true
	// SSO 登录两步（发验证码/验证码登录）匿名放行——登录前本就无会话
	case "/api/auth/sso/send-code", "/api/auth/sso/login":
		return true
	// 报告预览只是读禅道数据生成预览，不改任何状态
	case "/api/scheduler/preview", "/api/v1/scheduler/preview":
		return true
	}
	return false
}

// LoginHandler POST /api/auth/login  {"username","password"}
func (m *Manager) LoginHandler(ctx context.Context, c *app.RequestContext) {
	ip := c.ClientIP()
	if !m.loginAllowed(ip) {
		errors.Unauthorized(c, "登录失败次数过多，请 1 分钟后再试")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.BindJSON(&req); err != nil {
		errors.BadRequest(c, "请求格式错误")
		return
	}

	token, ok := m.Login(req.Username, req.Password)
	if !ok {
		m.recordLoginFailure(ip)
		errors.Unauthorized(c, "用户名或密码错误")
		return
	}
	token = m.signTokenWithUser(m.username, time.Now().Add(m.ttl))

	c.SetCookie(sessionCookieName, token, int(m.ttl.Seconds()), "/", "", protocol.CookieSameSiteLaxMode, false, true)
	errors.Success(c, map[string]interface{}{
		"token":    token,
		"username": m.username,
		"role":     "admin",
	})
}

// LogoutHandler POST /api/auth/logout 清除 Cookie（无状态令牌自然过期）
func (m *Manager) LogoutHandler(ctx context.Context, c *app.RequestContext) {
	c.SetCookie(sessionCookieName, "", -1, "/", "", protocol.CookieSameSiteLaxMode, false, true)
	errors.SuccessWithMessage(c, "已退出登录", nil)
}

// SSOSendCodeHandler POST /api/auth/sso/send-code {"username","password"}
// → 触发 SSO 短信验证码发送，返回脱敏手机号。
func (m *Manager) SSOSendCodeHandler(ctx context.Context, c *app.RequestContext) {
	if m.sso == nil {
		errors.BadRequest(c, "SSO 登录未启用")
		return
	}
	ip := c.ClientIP()
	if !m.loginAllowed(ip) {
		errors.Unauthorized(c, "登录失败次数过多，请 1 分钟后再试")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.BindJSON(&req); err != nil {
		errors.BadRequest(c, "请求格式错误")
		return
	}
	if req.Username == "" || req.Password == "" {
		errors.BadRequest(c, "用户名和密码不能为空")
		return
	}

	result, err := m.sso.SendCode(ctx, req.Username, req.Password)
	if err != nil {
		m.recordLoginFailure(ip)
		errors.Unauthorized(c, err.Error())
		return
	}
	errors.Success(c, map[string]interface{}{
		"mobile": result.Mobile,
		"hint":   "验证码已发送",
	})
}

// SSOLoginHandler POST /api/auth/sso/login {"username","password","code"}
// → SSO 短信验证码登录，成功签发平台会话令牌（Cookie + Bearer）。
func (m *Manager) SSOLoginHandler(ctx context.Context, c *app.RequestContext) {
	if m.sso == nil {
		errors.BadRequest(c, "SSO 登录未启用")
		return
	}
	ip := c.ClientIP()
	if !m.loginAllowed(ip) {
		errors.Unauthorized(c, "登录失败次数过多，请 1 分钟后再试")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := c.BindJSON(&req); err != nil {
		errors.BadRequest(c, "请求格式错误")
		return
	}
	if req.Username == "" || req.Password == "" || req.Code == "" {
		errors.BadRequest(c, "用户名、密码和验证码不能为空")
		return
	}

	if _, err := m.sso.DoSSOLogin(ctx, req.Username, req.Password, req.Code); err != nil {
		m.recordLoginFailure(ip)
		errors.Unauthorized(c, err.Error())
		return
	}

	// SSO 验证通过 → 签发平台会话令牌（带 SSO 用户名），与本地管理员登录同一会话体系
	token := m.signTokenWithUser(req.Username, time.Now().Add(m.ttl))
	c.SetCookie(sessionCookieName, token, int(m.ttl.Seconds()), "/", "", protocol.CookieSameSiteLaxMode, false, true)
	errors.Success(c, map[string]interface{}{
		"token":    token,
		"username": req.Username,
		"role":     "admin",
	})
}

// StatusHandler GET /api/auth/status
func (m *Manager) StatusHandler(ctx context.Context, c *app.RequestContext) {
	user, authenticated := m.VerifyUsername(string(c.Cookie(sessionCookieName)))
	if !authenticated {
		if bearer := string(c.GetHeader("Authorization")); strings.HasPrefix(bearer, "Bearer ") {
			user, authenticated = m.VerifyUsername(strings.TrimSpace(bearer[len("Bearer "):]))
		}
	}
	role := "readonly"
	if authenticated {
		role = "admin"
	}
	// 用户名取令牌内记录的登录者（SSO 账号 / 本地管理员）；旧格式无身份令牌回退本地管理员名
	username := user
	if authenticated && username == "" {
		username = m.username
	}
	errors.Success(c, map[string]interface{}{
		"authenticated": authenticated,
		"role":          role,
		"username":      map[bool]string{true: username, false: ""}[authenticated],
	})
}

// RegisterRoutes 注册 /api/auth/* 路由
func (m *Manager) RegisterRoutes(r *route.RouterGroup) {
	r.POST("/auth/login", m.LoginHandler)
	r.POST("/auth/logout", m.LogoutHandler)
	r.GET("/auth/status", m.StatusHandler)
	if m.sso != nil {
		r.POST("/auth/sso/send-code", m.SSOSendCodeHandler)
		r.POST("/auth/sso/login", m.SSOLoginHandler)
	}
}

// SecretFromEncryptionKey 从部署加密密钥派生会话签名密钥，保证重启后会话不失效
func SecretFromEncryptionKey(encryptionKey string) string {
	sum := sha256.Sum256([]byte("zentao-max:auth-session:" + encryptionKey))
	return hex.EncodeToString(sum[:])
}

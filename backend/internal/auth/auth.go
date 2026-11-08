// Package auth 实现平台访问控制：
// 匿名用户只读（GET），登录用户获得读写权限。登录仅支持公司 SSO
//（统一认证平台，用户名+密码+短信验证码）；本地账号密码登录已移除。
// 会话令牌为 HMAC 签名的无状态 token（exp.signature），放在 HttpOnly Cookie，
// 同时接受 Authorization: Bearer 头（供 API/MCP 客户端使用）。
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	ratelimit "github.com/oekit/concurrency/ratelimit"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/route"

	"github.com/yi-nology/zentao-max/backend/pkg/errors"
)

const (
	sessionCookieName = "zentao_max_session"
	defaultSessionTTL = 7 * 24 * time.Hour
	// 每分钟允许的匿名登录失败次数（防爆破），超过后临时拒绝该 IP
	loginFailuresPerMinute = 10
)

type Manager struct {
	secret []byte
	ttl    time.Duration

	// loginLockout 登录防爆破锁（oekit ratelimit.Lockout，fail2ban 形态）：
	// 窗口内失败达阈值 → 该 IP 进入锁定拒绝态。替代手搓 failures map
	//（滑动窗口/锁定剩余时长/空闲回收/容量护栏 fail-open 全部由原语钉死）
	loginLockout *ratelimit.LockoutOf[string]

	// sso 麒麟统一认证登录（用户名+密码+短信验证码）；nil = 未启用
	sso *SSOClient

	// bootstrapOpen 首启引导窗口判定：系统无任何已保存的禅道连接配置时为 true，
	// 放行 /api/v1/zentao-config 与 /api/v1/sso-config 的匿名写——否则首启会
	// 死锁（配置端点要求登录，而登录依赖 SSO，SSO 依赖先配置）。任何配置来源
	//（auth.db / config.yaml / env）存在即不开窗，防公网未初始化实例被接管。
	bootstrapOpen func() bool
}

// SetSSO 启用 SSO 登录通道（/api/auth/sso/*）。
func (m *Manager) SetSSO(client *SSOClient) {
	m.sso = client
}

// SetBootstrapOpen 注入首启引导窗口判定（nil = 永不开窗）。
func (m *Manager) SetBootstrapOpen(fn func() bool) {
	m.bootstrapOpen = fn
}

// NewManager 创建认证管理器。secret 用于会话令牌签名，
// 建议传入部署时稳定的随机串（如加密密钥派生值），重启后会话不失效。
func NewManager(secret string) *Manager {
	return &Manager{
		secret: []byte(secret),
		ttl:      defaultSessionTTL,
		loginLockout: ratelimit.NewLockoutOf[string](ratelimit.LockoutConfig{
			Window:       time.Minute,
			Threshold:    loginFailuresPerMinute,
			LockDuration: time.Minute,
			IdleTTL:      10 * time.Minute,
			MaxEntries:   4096,
		}),
	}
}

// signTokenWithUser 签发带登录者身份的令牌：exp:username 分段（SSO 登录者）。
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

// loginAllowed 供测试断言内部状态（生产路径走 CheckLocked）。
func (m *Manager) loginAllowed(ip string) bool {
	locked, _ := m.loginLockout.CheckLocked(ip)
	return !locked
}

// recordLoginFailure 记一次登录失败（窗口内达阈值自动进入锁定态）。
func (m *Manager) recordLoginFailure(ip string) {
	m.loginLockout.RecordFailure(ip)
}

// resetLoginFailures 登录成功后清零该 IP 的失败计数（fail2ban Reset 语义）。
func (m *Manager) resetLoginFailures(ip string) {
	m.loginLockout.Reset(ip)
}

// Middleware 返回 Hertz 中间件：
//   - MCP 端点完全豁免（有自己独立的 mcp.token/只读/白名单访问控制）；
//   - 读方法（GET/HEAD/OPTIONS）匿名放行；
//   - 匿名放行的写路径：登出接口、SSO 两步、报告预览；
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

		// 首启引导窗口：系统完全未配置时，放行两个配置端点的匿名写，
		// 让首启向导能完成初始设置（否则配置需要登录、登录需要配置，死锁）
		if m.bootstrapOpen != nil && m.bootstrapOpen() &&
			(path == "/api/v1/zentao-config" || path == "/api/v1/sso-config") {
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
	case "/api/auth/logout":
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
	// 用户名取令牌内记录的登录者（SSO 账号）；旧格式无身份令牌显示为空
	username := user
	errors.Success(c, map[string]interface{}{
		"authenticated": authenticated,
		"role":          role,
		"username":      map[bool]string{true: username, false: ""}[authenticated],
	})
}

// RegisterRoutes 注册 /api/auth/* 路由
func (m *Manager) RegisterRoutes(r *route.RouterGroup) {
	r.POST("/auth/logout", m.LogoutHandler)
	r.GET("/auth/status", m.StatusHandler)
	// sso/status 恒注册（匿名 GET，仅暴露启用布尔值）：登录页据此选择默认登录方式
	r.GET("/auth/sso/status", m.SSOStatusHandler)
	// send-code / login 恒注册：SSO 配置支持运行时热切换（设置页保存即生效），
	// 未启用时 handler 内统一报「SSO 登录未启用」（不注册的话启用后要重启才通）。
	r.POST("/auth/sso/send-code", m.SSOSendCodeHandler)
	r.POST("/auth/sso/login", m.SSOLoginHandler)
}

// SSOStatusHandler GET /api/auth/sso/status → {"enabled": bool}
// 供登录页判断 SSO 是否已启用（唯一登录通道）。
func (m *Manager) SSOStatusHandler(ctx context.Context, c *app.RequestContext) {
	errors.Success(c, map[string]interface{}{
		"enabled": m.sso != nil,
	})
}

// SecretFromEncryptionKey 从部署加密密钥派生会话签名密钥，保证重启后会话不失效
func SecretFromEncryptionKey(encryptionKey string) string {
	sum := sha256.Sum256([]byte("zentao-max:auth-session:" + encryptionKey))
	return hex.EncodeToString(sum[:])
}

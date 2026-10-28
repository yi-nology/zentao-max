package zentao

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	neturl "net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yi-nology/zentao-max/backend/core/logger"

		"golang.org/x/net/publicsuffix"
)

// sessionTTL 会话被认为有效的最长时间，超过后强制重新登录。
// 实际禅道会话过期时间由服务端 PHP session.gc 决定（默认 1440s 不活动），
// 这里取一个保守值；配合 keepLogin cookie 应能维持较长时间。
const sessionTTL = 2 * time.Hour

// sessionLoginPageTTL 两次探测登录页之间的最小间隔，避免登录请求风暴。
const sessionLoginPageTTL = 30 * time.Second

// errSessionReloginRequired 表示当前会话已失效，需要重新登录后重试。
// 由 Do/DoJSON 在检测到 data.locate 指向 user-login 时返回。
var errSessionReloginRequired = errors.New("session expired, re-login required")

// errSessionAccessDenied 表示账号无权访问该端点（user-deny），
// 这是永久性错误，不应触发重试。
var errSessionAccessDenied = errors.New("access denied by zentao (user-deny)")

// SessionTransport 负责会话模式的登录、cookie 管理与 *.json 端点访问。
//
// 与 Token 模式的差异：
//   - 认证：POST /user-login.html（表单）建立 zentaosid cookie，而非换 Token
//   - 数据：GET /xxx.json（cookie 鉴权），而非 GET /api.php/v1/*（Token 头）
//   - 响应：双层 JSON 包裹 {status,data(stringified),md5}，需二次解析
//   - 鉴权失败：HTTP 200 + data.locate 指向 user-login，而非 401
type SessionTransport struct {
	server   string
	account  string
	password *SecureString
	realm    string // "kydc" 或 "local"

	httpClient *http.Client
	cookieJar  http.CookieJar

	// sessionExpiry 缓存的有效期（unix 纳秒）。0 表示未登录。
	sessionExpiry atomic.Int64
	refreshing    atomic.Bool
	mu            sync.Mutex

	// lastLoginAttempt 防止短时间内重复触发登录请求。
	lastLoginAttempt atomic.Int64
}

// NewSessionTransport 构造会话传输层。server 应已规范化（带 scheme、无尾斜杠）。
func NewSessionTransport(server, account, password, realm string) *SessionTransport {
	if realm == "" {
		realm = RealmKySSO
	}
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		// cookiejar.New 仅在 options 非法时返回错误，这里不会发生。
		logger.Error("Failed to create cookie jar, falling back to in-memory", "error", err)
		jar, _ = cookiejar.New(nil)
	}
	hc := &http.Client{
		Timeout: 120 * time.Second,
		Jar:     jar,
		Transport: &http.Transport{
			// 内网禅道常用自签名证书，与上游 SDK doGet / GetAllBugs 行为一致。
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	return &SessionTransport{
		server:     server,
		account:    account,
		password:   NewSecureString(password),
		realm:      realm,
		httpClient: hc,
		cookieJar:  jar,
	}
}

// UpdateConfig 热更新账号/密码/域，并使当前会话失效，下次请求会重新登录。
func (s *SessionTransport) UpdateConfig(server, account, password, realm string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if server != "" {
		s.server = server
	}
	s.account = account
	s.password.Set(password)
	if realm != "" {
		s.realm = realm
	}
	s.sessionExpiry.Store(0)
	// 清空 cookie jar，强制下次请求重建会话。
	// 必须同步重置 s.httpClient.Jar 和 s.cookieJar 字段，因为两者各自持有 jar 引用，
	// 只换字段会让 hasZentaoSID 查不到 httpClient 写进去的 cookie（看似登录成功但 cookie 丢失）。
	newJar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	s.cookieJar = newJar
	if s.httpClient != nil {
		s.httpClient.Jar = newJar
	}
}

// invalidateForRelogin 清空 cookie jar 与过期标记，强制下次 ensureLoggedIn 执行 login()。
// 仅供 doSessionJSON 在收到 user-login 重定向（真正的会话失效）时调用。
// 注意：必须同时重置 s.httpClient.Jar 和 s.cookieJar 字段，因为两者各自持有引用，
// 只换字段会让 hasZentaoSID 查不到 httpClient 写进去的 cookie。
func (s *SessionTransport) invalidateForRelogin() {
	s.sessionExpiry.Store(0)
	newJar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	s.cookieJar = newJar
	if s.httpClient != nil {
		s.httpClient.Jar = newJar
	}
}

// IsLoggedIn 返回会话是否仍然有效。
// 判定依据：未过有效期，或 cookie jar 仍持有 zentaosid（即使重新登录被 SSO 限流，
// 只要 cookie 还在，请求仍能成功）。
func (s *SessionTransport) IsLoggedIn() bool {
	exp := s.sessionExpiry.Load()
	if exp != 0 && time.Now().UnixNano() < exp {
		return true
	}
	// 过期或未登录标记：但仍可能持有有效 cookie（受限实例 SSO 会复用现有会话）。
	return s.hasZentaoSID()
}

// zentaoSIDExpiryTTL 是 SetZentaoSID 注入的 cookie "软过期"时间。
//
// 2 小时：足够覆盖单次开发/调试 session 长度；超时后 IsLoggedIn 会再
// 调 ensureLoggedIn，但因为 jar 里仍有 zentaosid cookie，所以是
// "cookie 复用" 而非 "重新 login"（不消耗受限实例 SSO 配额）。
//
// 真过期由受限实例自己判（返 user-login redirect → invalidateForRelogin →
// 走 login fallback）。
const zentaoSIDExpiryTTL = 2 * time.Hour

// SetZentaoSID 注入浏览器 zentaosid 到 cookie jar + 标记软过期。
//
// 详见 Client.SetZentaoSID 的完整说明。
//
// 注：Client.SetZentaoSID 是 public 入口，会在调完本方法后设置 Client.connected 标志；
// SessionTransport 自己不持有 connected，所以本方法不设它。
func (s *SessionTransport) SetZentaoSID(sid string) error {
	if sid == "" {
		return fmt.Errorf("zentaosid 不能为空")
	}
	if s.cookieJar == nil {
		return fmt.Errorf("cookie jar 未初始化")
	}
	u, err := neturl.Parse(s.server)
	if err != nil {
		return fmt.Errorf("解析 server URL 失败: %w", err)
	}

	// 把 zentaosid 写到 cookie jar。
	cookie := &http.Cookie{
		Name:     "zentaosid",
		Value:    sid,
		Path:     "/",
		Domain:   u.Host,
		Secure:   u.Scheme == "https",
		HttpOnly: true,
	}
	if u.Host != "" {
		s.cookieJar.SetCookies(u, []*http.Cookie{cookie})
	}

	// 验证 jar 里有 zentaosid（hasZentaoSID 通过 name+value 检测）。
	// 一些内网域名（非标准 public suffix）会拒绝 SetCookies，所以兜底重建 jar。
	if !s.hasZentaoSID() {
		newJar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
		newJar.SetCookies(u, []*http.Cookie{cookie})
		s.cookieJar = newJar
		if s.httpClient != nil {
			s.httpClient.Jar = newJar
		}
	}

	// 标记软过期；IsLoggedIn 软过期后仍可走 hasZentaoSID 路径
	s.sessionExpiry.Store(time.Now().Add(zentaoSIDExpiryTTL).UnixNano())
	return nil
}

// Realm 返回当前认证域。
func (s *SessionTransport) Realm() string { return s.realm }

// ensureLoggedIn 在会话失效时串行重新登录。
// 判定逻辑：
//  1. sessionExpiry 未过期 → 直接放行
//  2. cookie jar 仍持有 zentaosid → 放行（即使标记过期，cookie 可能仍有效，
//     由 DoJSON 在请求失败时触发 re-login）
//  3. 完全无 cookie → 真正登录
//
// 这样能避免受限实例 SSO 在短时间内重复登录被限流的问题：只要 cookie 还在，
// 就不主动重新登录；真正的失效由 DoJSON 检测到 data.locate→user-login 后触发。
func (s *SessionTransport) ensureLoggedIn() error {
	if s.IsLoggedIn() {
		return nil
	}
	if !s.refreshing.CompareAndSwap(false, true) {
		// 另一个 goroutine 正在登录，轮询等待（最多 15s）。
		for i := 0; i < 75; i++ {
			time.Sleep(200 * time.Millisecond)
			if !s.refreshing.Load() {
				if s.IsLoggedIn() {
					return nil
				}
				break
			}
		}
		if s.IsLoggedIn() {
			return nil
		}
		return errors.New("会话登录进行中，请稍后重试")
	}
	defer s.refreshing.Store(false)
	return s.login()
}

// login 执行三步登录流程：
//  1. GET /user-login.html 种 zentaosid cookie
//  2. GET /user-refreshRandom.html 拿到 rand（与 session 绑定）
//  3. POST /user-login.html 提交表单（含 account/password/verifyRand/realm 等）
//
// kydc 域下密码明文发送；local 域下需 md5(md5(password)+rand)。
func (s *SessionTransport) login() error {
	// 限流：30s 内不重复尝试登录，避免凭据错误时刷屏。
	now := time.Now().UnixNano()
	last := s.lastLoginAttempt.Load()
	if now-last < int64(sessionLoginPageTTL) {
		// 可能是上一个登录请求刚失败，但仍可能已成功；检查一次。
		if s.IsLoggedIn() {
			return nil
		}
	}
	s.lastLoginAttempt.Store(now)

	s.mu.Lock()
	server := s.server
	account := s.account
	password := s.password.Get()
	realm := s.realm
	s.mu.Unlock()

	if server == "" {
		return errors.New("禅道服务器地址为空")
	}

	// 1. 种 cookie。
	// 注意：GET /user-login.html 不能带 X-Requested-With 头，否则禅道把它当 AJAX
	// 请求处理，不会正常初始化 PHP 会话/种 zentaosid cookie。
	if _, err := s.httpGetPlain(context.Background(), server+"/user-login.html"); err != nil {
		return fmt.Errorf("加载登录页失败: %w", err)
	}

	// 2. 取 rand。这个端点本身就是 AJAX 用的，可以带 X-Requested-With。
	randBytes, err := s.httpGet(context.Background(), server+"/user-refreshRandom.html", nil)
	if err != nil {
		return fmt.Errorf("获取登录随机数失败: %w", err)
	}
	rand := strings.TrimSpace(string(randBytes))
	if rand == "" {
		return errors.New("登录随机数为空")
	}

	// 3. 构造密码。kydc 域明文，其它域按 md5(md5(pw)+rand)。
	passwordField := password
	if realm != RealmKySSO {
		passwordField = md5Hex(md5Hex(password) + rand)
	}

	// 4. POST 登录
	form := neturl.Values{}
	form.Set("account", account)
	form.Set("password", passwordField)
	form.Set("passwordStrength", "1")
	form.Set("referer", "/")
	form.Set("verifyRand", rand)
	form.Set("keepLogin[]", "on")
	form.Set("realm", realm)

	req, err := http.NewRequest(http.MethodPost, server+"/user-login.html", strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("构造登录请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Referer", server+"/user-login.html")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("登录请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("登录失败，HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	// 解析 {result, message?, locate?}
	var loginResp struct {
		Result  string `json:"result"`
		Message string `json:"message"`
		Locate  string `json:"locate"`
	}
	if err := json.Unmarshal(body, &loginResp); err != nil {
		return fmt.Errorf("解析登录响应失败: %w (body=%s)", err, truncate(string(body), 200))
	}
	if loginResp.Result != "success" {
		msg := loginResp.Message
		if msg == "" {
			msg = strings.TrimSpace(string(body))
		}
		return fmt.Errorf("登录失败: %s", msg)
	}

	// 校验 cookie jar 里确实拿到了 zentaosid。
	if !s.hasZentaoSID() {
		return errors.New("登录响应成功但未收到 zentaosid cookie")
	}

	s.sessionExpiry.Store(time.Now().Add(sessionTTL).UnixNano())
	logger.Info("Zentao session login successful",
		"account", account,
		"realm", realm,
	)
	return nil
}

// hasZentaoSID 检查 cookie jar 是否包含有效的 zentaosid。
func (s *SessionTransport) hasZentaoSID() bool {
	if s.cookieJar == nil || s.server == "" {
		return false
	}
	u, err := neturl.Parse(s.server)
	if err != nil {
		return false
	}
	for _, c := range s.cookieJar.Cookies(u) {
		if c.Name == "zentaosid" && c.Value != "" {
			return true
		}
	}
	return false
}

// zentaoEnvelope 是 *.json 端点的外层包裹。
type zentaoEnvelope struct {
	Status string `json:"status"`
	// Data 绝大多数情况下是字符串化的 JSON；少数端点（错误时）直接是对象。
	Data json.RawMessage `json:"data"`
	MD5  string          `json:"md5"`
}

// locateRedirect 是未认证/无权限时 data 字段的形状。
type locateRedirect struct {
	Locate string `json:"locate"`
}

// DoJSON 发起 GET 请求到 path（相对 server 根，如 "/bug-browse-1-0-all.json"），
// 解开外层 {status,data,md5} 包裹，把内层 data 反序列化到 out。
//
// 鉴权失败（data.locate 含 user-login）时返回 errSessionReloginRequired，
// 调用方（withSessionRetry）会重新登录后重试一次。
// 无权限（data.locate 含 user-deny）时返回 errSessionAccessDenied（不重试）。
func (s *SessionTransport) DoJSON(ctx context.Context, path string, out interface{}) error {
	inner, err := s.DoJSONRaw(ctx, path)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	// 用 UseNumber() 解析，使 pager 的数值字段能同时接受 int 和 string
	//（不同禅道实例/端点返回类型不一致，见 sessionPager 的 json.Number 字段）。
	if err := unmarshalUseNumber(inner, out); err != nil {
		return fmt.Errorf("解析 data 内层 JSON 失败 (path=%s): %w; raw=%s", path, err, truncate(string(inner), 300))
	}
	return nil
}

// DoJSONRaw 发起 GET 请求到 path，返回解包后的内层 data 原始 JSON 字节。
// 用途：SessionCache 直接缓存原始字节，避免 Go struct → json → Go struct 的
// round-trip 损失。业务侧仍用 DoJSON（已经填好 out）。
//
// 鉴权失败时同样返回 errSessionReloginRequired / errSessionAccessDenied。
func (s *SessionTransport) DoJSONRaw(ctx context.Context, path string) ([]byte, error) {
	if err := s.ensureLoggedIn(); err != nil {
		return nil, err
	}

	body, err := s.httpGet(ctx, s.server+path, nil)
	if err != nil {
		return nil, err
	}

	var env zentaoEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("解析禅道响应外层 JSON 失败 (path=%s): %w; body=%s", path, err, truncate(string(body), 200))
	}

	innerData := env.Data
	if len(innerData) > 0 && innerData[0] == '"' {
		// 字符串化的 JSON：先解出字符串，再当 JSON 用。
		var s2 string
		if err := json.Unmarshal(innerData, &s2); err != nil {
			return nil, fmt.Errorf("解析 data 字符串失败 (path=%s): %w", path, err)
		}
		innerData = json.RawMessage(s2)
	}

	// 先看是不是 locate 重定向（未认证/无权限）。这种情况不应继续。
	var loc locateRedirect
	if err := json.Unmarshal(innerData, &loc); err == nil && loc.Locate != "" {
		lower := strings.ToLower(loc.Locate)
		if strings.Contains(lower, "user-login") {
			s.sessionExpiry.Store(0)
			return nil, fmt.Errorf("%w (locate=%s)", errSessionReloginRequired, loc.Locate)
		}
		if strings.Contains(lower, "user-deny") {
			return nil, fmt.Errorf("%w (locate=%s)", errSessionAccessDenied, loc.Locate)
		}
		return nil, fmt.Errorf("访问被重定向: %s", loc.Locate)
	}

	return innerData, nil
}

// unmarshalUseNumber 用 json.Decoder + UseNumber() 解析，
// 让数值字段可以装入 json.Number（兼容 int/string 两种返回）。
func unmarshalUseNumber(data []byte, v interface{}) error {
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber()
	return dec.Decode(v)
}

// httpGetPlain 执行 GET 请求但不带 X-Requested-With 头。
// 仅供登录流程的 GET /user-login.html 使用——禅道会根据该头是否存在返回不同的
// 响应（HTML 页 vs AJAX 响应），只有不带该头的请求才会正常种 zentaosid cookie。
func (s *SessionTransport) httpGetPlain(ctx context.Context, fullURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s (url=%s)", resp.StatusCode, truncate(string(body), 200), fullURL)
	}
	return body, nil
}

// httpGet 执行 GET 请求，携带 cookie jar。返回响应体字节。
func (s *SessionTransport) httpGet(ctx context.Context, fullURL string, extraHeader http.Header) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	for k, vs := range extraHeader {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s (url=%s)", resp.StatusCode, truncate(string(body), 200), fullURL)
	}
	return body, nil
}

// GetRawJSON 跳过外层 {status,data,md5} 解包，直接返回 body 字节。
//
// 用途：部分端点（如 /effort-ajaxGetEfforts-{userID}-{year}.html）响应
// 是顶层 JSON 数组而不是双层包裹，DoJSONRaw 解外层会报
// "cannot unmarshal array into Go value of type zentao.zentaoEnvelope"。
//
// 鉴权失败的检测用 body 启发式：
//   - body[0] == '[' 或 '{' → 认为是 JSON 响应，直接返 body
//   - body[0] == '<' → 认为是 user-login HTML 重定向页，触发 invalidateForRelogin
//
// 不像 DoJSONRaw 那样解 locate —— 顶层数组端点不返 locate，调用方解析失败时
// 自然返原 err，缓存策略由 SessionCache 用 5min fresh / 1h stale TTL 控制。
func (s *SessionTransport) GetRawJSON(ctx context.Context, path string) ([]byte, error) {
	if err := s.ensureLoggedIn(); err != nil {
		return nil, err
	}
	body, err := s.httpGet(ctx, s.server+path, nil)
	if err != nil {
		return nil, err
	}
	// 启发式鉴权失败检测：HTML 重定向页（user-login 等）以 '<' 开头。
	if len(body) > 0 && body[0] == '<' {
		s.invalidateForRelogin()
		return nil, fmt.Errorf("%w (path=%s, body=%s)", errSessionReloginRequired, path, truncate(string(body), 100))
	}
	return body, nil
}

// IsSessionAuthError 判断错误是否为会话失效（可重新登录后重试）。
func IsSessionAuthError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, errSessionReloginRequired)
}

// PostForm 在会话模式下提交表单到 path（写操作）。
// 禅道 PHP 写端点（如 /bug-resolve-{id}.json）接受 application/x-www-form-urlencoded，
// 返回 {result:"success", message?, ...} 或 {result:"fail", message}。
// 成功返回 nil；失败返回包含后端 message 的错误。
// 与 DoJSON 不同：写端点的响应外层就是直接的 JSON 对象（不是 {status,data,md5} 包裹）。
func (s *SessionTransport) PostForm(ctx context.Context, path string, form neturl.Values) error {
	_, err := s.PostFormWithBody(ctx, path, form)
	return err
}

// PostFormWithBody 与 PostForm 相同，但返回原始响应体供调用方做业务结果判定。
// 判定规则见 PostFormWithBody 内部注释；HTML 响应不再一律视为成功。
func (s *SessionTransport) PostFormWithBody(ctx context.Context, path string, form neturl.Values) ([]byte, error) {
	if err := s.ensureLoggedIn(); err != nil {
		return nil, err
	}
	fullURL := s.server + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("写请求失败 (%s): %w", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return body, fmt.Errorf("写操作失败 HTTP %d: %s (path=%s)", resp.StatusCode, truncate(string(body), 200), path)
	}

	// 解析 {result, message?, ...}
	var writeResp struct {
		Result  string `json:"result"`
		Message string `json:"message"`
		Locate  string `json:"locate"`
	}
	if err := json.Unmarshal(body, &writeResp); err != nil {
		// HTML 响应：写端点常见两种形态——
		//   成功：跳转脚本或"保存成功"提示（如 /bug-create-{id}-moduleID=0.html 创建成功）；
		//   失败：弹回表单页（表单校验被服务端拒绝），此前一律按成功处理导致静默失败。
		// 这里不做硬判定，把 body 交调用方按端点语义判定。
		return body, nil
	}
	// 未认证：定位到登录页。
	if lower := strings.ToLower(writeResp.Locate); strings.Contains(lower, "user-login") {
		s.sessionExpiry.Store(0)
		return body, fmt.Errorf("%w (locate=%s)", errSessionReloginRequired, writeResp.Locate)
	}
	if writeResp.Result == "success" || writeResp.Result == "saved" || writeResp.Result == "" {
		return body, nil
	}
	msg := writeResp.Message
	if msg == "" {
		msg = truncate(string(body), 200)
	}
	return body, fmt.Errorf("写操作失败: %s", msg)
}

// IsSessionAccessDenied 判断错误是否为权限拒绝（不可重试）。
func IsSessionAccessDenied(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, errSessionAccessDenied)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

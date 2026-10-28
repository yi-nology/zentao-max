package zentao

import (
	"context"
	"fmt"
	"log"
	neturl "net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oekit/concurrency/async"

	"github.com/yi-nology/zentao-max/backend/core/logger"

	)

// Client 封装禅道会话模式客户端（PHP *.json 端点 + zentaosid cookie）。
// 适用于标准禅道开源版/企业版以及禁用 REST API 的实例（如受限实例 your-zentao-server.com）。
//
// Session 模式专属：
//   - cache：SessionCache 二级 cache（L1 in-memory + L2 SQLite + singleflight + stale fallback）。
//   - pool ：SessionWorkerPool 限流 + 写串行 + 读并发 4。
// 两者都 nil-safe：构造时若 server/account 为空则 nil，所有 doSessionJSON/Post 仍可工作。
type Client struct {
	account   string
	password  *SecureString
	server    string
	mu        sync.RWMutex
	connected atomic.Bool

	realm   string
	session *SessionTransport
	cache   *SessionCache
	pool    *SessionWorkerPool
}

// NewSessionClient 创建会话模式的禅道客户端。
// 登录走 /user-login.html（PHP 会话），数据访问走 *.json 端点。
// 适用于标准禅道实例以及禁用 REST API 的实例（如受限实例 your-zentao-server.com）。
//
// cacheDBPrefix 为 "" 时 SessionCache 只用 L1（不创建 SQLite 文件，适合单测）；
// poolConfig nil 时 pool 不启用（doSessionJSON/Post 直接调 session.DoJSON）。
func NewSessionClient(server, account, password, realm string, cacheDBPrefix string, poolConfig *SessionWorkerPoolConfig) *Client {
	if server != "" {
		if !strings.HasPrefix(server, "http://") && !strings.HasPrefix(server, "https://") {
			server = "http://" + server
		}
		server = strings.TrimSuffix(server, "/")
	}
	c := &Client{
		account:  account,
		password: NewSecureString(password),
		server:   server,
		realm:    realm,
		session:  NewSessionTransport(server, account, password, realm),
	}
	if server != "" && account != "" {
		c.cache = NewSessionCache(SessionCacheConfig{
			Server:   server,
			Account:  account,
			Realm:    realm,
			DBPrefix: cacheDBPrefix,
		})
	}
	if poolConfig != nil {
		p := NewSessionWorkerPool(*poolConfig)
		p.Start()
		c.pool = p
	}
	// 会话模式：不再启动时异步登录（避免账号被 SSO 锁定时反复重试加速锁定）。
	// 改为懒加载：首次数据请求时
	// 由 ensureLoggedIn 触发登录。connected 状态在登录成功后才置 true。
	// GoSafe：keepalive 长驻 goroutine panic 时收编 + 日志，不会静默死亡
	async.GoSafe(c.startSessionKeepalive)
	return c
}

// startSessionKeepalive 会话模式下的保活：每 30 分钟检查一次，
// 若会话过期则在下次请求时自动重新登录（ensureLoggedIn 已覆盖）。
// 这里仅负责把 connected 标志位同步成实际状态。
func (c *Client) startSessionKeepalive() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		if c.session == nil {
			return
		}
		c.connected.Store(c.session.IsLoggedIn())
	}
}

// UpdateSessionConfig 会话模式的热重载入口。同步触发一次登录验证凭据，
// 返回登录错误（成功返回 nil）。失败时 connected 标志为 false。
//
// 切账号时同时清空旧 cache（UpdateNamespace）+ 关掉旧 pool（重建）。
func (c *Client) UpdateSessionConfig(server, account, password, realm string) error {
	if server != "" {
		if !strings.HasPrefix(server, "http://") && !strings.HasPrefix(server, "https://") {
			server = "http://" + server
		}
		server = strings.TrimSuffix(server, "/")
	}

	c.mu.Lock()
	c.server = server
	c.account = account
	c.password.Set(password)
	if realm != "" {
		c.realm = realm
	}
	if c.session == nil {
		c.session = NewSessionTransport(server, account, password, realm)
	} else {
		c.session.UpdateConfig(server, account, password, realm)
	}
	// cache 切 namespace（旧 ns 数据 L1 自动清，L2 留作快速回切）
	if c.cache != nil {
		c.cache.UpdateNamespace(server, account, realm)
	}
	// pool 不持状态，命名空间无关；继续复用即可。
	c.mu.Unlock()

	// 同步登录：登录表单提交后需要立即知道凭据是否有效，
	// 不能异步（否则前端拿不到明确的成败反馈）。
	if err := c.session.ensureLoggedIn(); err != nil {
		c.connected.Store(false)
		return err
	}
	c.connected.Store(true)
	return nil
}

// Close 优雅关闭 Client 的 pool + cache。App 退出时调用。
func (c *Client) Close(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pool != nil {
		if err := c.pool.Shutdown(ctx); err != nil {
			return err
		}
	}
	if c.cache != nil {
		_ = c.cache.Close()
	}
	return nil
}

func (c *Client) GetServer() string {
	return c.server
}

// SetZentaoSID 注入浏览器 zentaosid 到 cookie jar，跳过 login 流程。
//
// 用法：把浏览器登录拿到的 zentaosid cookie 写入 ~/.zentao-max/zentaosid.json →
// server 启动时 wire.go 读这个文件 → 调 SetZentaoSID → 后续 request 直接用
// zentaosid 调禅道，不再触发 zentao client 的 login()，从而不消耗受限实例
// SSO "密码尝试次数太多" 配额。
//
// 行为：
//   - 成功：cookie jar 注入 zentaosid，sessionExpiry = now + sidExpiryTTL（2 小时）
//     connected 标志置 true（跟 SessionLoginSync 成功一致）
//   - zentaosid 后续被禅道 invalidate（返 user-login redirect）：doSessionJSONWithRetry
//     会自动 fallback 重新 login（1 次）。如果 login 也失败就走老路径返 err。
//   - 重复调：覆盖旧 cookie（同一 cookie name）。
func (c *Client) SetZentaoSID(sid string) error {
	if c.session == nil {
		return fmt.Errorf("session transport not initialized")
	}
	if err := c.session.SetZentaoSID(sid); err != nil {
		return err
	}
	c.connected.Store(true)
	return nil
}

// GetRawJSON 会话模式下 GET 一个不走双层 {status,data,md5} 包裹的端点。
//
// 适用：响应是顶层 JSON 数组（[...]) 或其他非包裹 shape 的端点。
// 例如 /effort-ajaxGetEfforts-{userID}-{year}.html 返顶层数组，普通 doSessionJSON
// 解 zentaoEnvelope 会失败。
//
// 鉴权失败用 body 启发式：HTML 重定向页（'<' 开头）→ invalidateForRelogin +
// 返 errSessionReloginRequired。其他情况直接返 body，调用方自己 json.Unmarshal。
func (c *Client) GetRawJSON(ctx context.Context, path string) ([]byte, error) {
	if c.session == nil {
		return nil, fmt.Errorf("session transport not initialized")
	}
	return c.session.GetRawJSON(ctx, path)
}

func (c *Client) IsConnected() bool {
	return c.connected.Load()
}

// MarkConnected 手动把 connected 标志置 true。
// 供心跳探测等实时验证连通性的路径回写标志（后台 ticker 周期同步可能滞后）。
func (c *Client) MarkConnected() {
	c.connected.Store(true)
}

// GetAccount 获取当前登录用户的账号
func (c *Client) GetAccount() string {
	return c.account
}

// GetRealm 返回会话模式的认证域（kydc / local）。
func (c *Client) GetRealm() string {
	return c.realm
}

// SessionLoginSync 同步执行一次会话登录。仅供 InitHandler.Login 在表单提交时
// 显式调用以获得同步的成败反馈。
func (c *Client) SessionLoginSync() error {
	if c.session == nil {
		return fmt.Errorf("会话客户端未初始化")
	}
	if err := c.session.ensureLoggedIn(); err != nil {
		c.connected.Store(false)
		return err
	}
	c.connected.Store(true)
	return nil
}

// doSessionJSON 在会话模式下请求 *.json 端点。鉴权失败（zentaosid 过期）时
// 强制重新登录并重试一次；权限拒绝等永久错误直接返回。
// path 形如 "/bug-browse-1029-0-all.json"，out 为内层 data 的目标结构指针。
//
// Session 模式下：先查 cache (SessionCache)，命中直接返；miss 走 worker pool
// (SessionWorkerPool)，pool 调 session.DoJSONRaw 拿原始字节并写 cache。pool 满
// 100ms 排队超时 → cache fallback（直接查 cache 的 stale 数据）。整套
// 击穿保护由 cache 的 singleflight + pool 的限流共同保证。
//
// Token 模式 / cache==nil / pool==nil：所有 cache+pool 路径 nil-safe 跳过，
// 行为与改造前一致。
func (c *Client) doSessionJSON(ctx context.Context, operation, path string, out interface{}) error {
	if c.session == nil {
		return fmt.Errorf("会话客户端未初始化")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	entity, scope := opToCacheKey(operation, path)

	// 走 cache + pool 的路径：cache 命中直接返；miss 走 pool 限流打禅道。
	if c.cache != nil {
		return c.doSessionJSONCached(ctx, operation, path, entity, scope, out)
	}
	// 退化：cache 不可用，至少走 pool 限流（如果有）
	if c.pool != nil {
		return c.pool.SubmitRead(ctx, operation,
			func(ctx context.Context) error {
				return c.doSessionJSONWithRetry(ctx, operation, path, out)
			},
			nil, // 没有 cache 就不接 fallback
		)
	}
	// 极致退化：直连
	return c.doSessionJSONWithRetry(ctx, operation, path, out)
}

// doV1JSON 在会话模式下请求 v1 REST API 端点（/api.php/v1/*）。
// 与 doSessionJSON 区别：v1 响应不是双层 envelope 而是单层 JSON，
// 所以走 GetRawJSON 跳过 envelope 解包。其它 cache+pool+限流逻辑完全一致。
//
// 用途：受限实例的 session 端点（testcase-browse 等）不支持 server-side 翻页，
// 但 v1 REST API 用 cookie 鉴权能真翻页。v1 path 走 /api.php/v1/testcases?product=&page=&limit=。
func (c *Client) doV1JSON(ctx context.Context, operation, path string, out interface{}) error {
	if c.session == nil {
		return fmt.Errorf("会话客户端未初始化")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	entity, scope := opToCacheKey(operation, path)

	if c.cache != nil {
		return c.doV1JSONCached(ctx, operation, path, entity, scope, out)
	}
	if c.pool != nil {
		return c.pool.SubmitRead(ctx, operation,
			func(ctx context.Context) error {
				return c.doV1JSONDirect(ctx, path, out)
			},
			nil,
		)
	}
	return c.doV1JSONDirect(ctx, path, out)
}

// doV1JSONCached 跟 doSessionJSONCached 一样的 cache+pool 包装，但 loader 调
// GetRawJSON（跳过 envelope）。不同 (product, page) 走不同 scope key。
func (c *Client) doV1JSONCached(
	ctx context.Context,
	operation, path string,
	entity sessionEntity,
	scope string,
	out interface{},
) error {
	loader := func(ctx context.Context) ([]byte, error) {
		raw, err := c.session.GetRawJSON(ctx, path)
		if err != nil {
			return nil, err
		}
		return raw, nil
	}
	safeLoader := func(ctx context.Context) (data []byte, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("v1 loader panicked: %v", r)
			}
		}()
		data, err = loader(ctx)
		if err != nil && (IsSessionAuthError(err) || IsSessionAccessDenied(err)) {
			err = fmt.Errorf("%w: %w", errDoNotServeStale, err)
		}
		return data, err
	}
	cacheFallback := func(ctx context.Context) error {
		return c.serveFromStaleCacheV1(ctx, entity, scope, out)
	}
	runWithCache := func(ctx context.Context) error {
		res, err := c.cache.GetOrLoad(ctx, entity, scope, safeLoader)
		if err != nil {
			return err
		}
		if out != nil && len(res.Data) > 0 {
			if uerr := unmarshalUseNumber(res.Data, out); uerr != nil {
				// v1 unmarshal fail 跟 session 端点同样降级: log warning + 返空.
				log.Printf("[doV1JSON] unmarshal fail: path=%s op=%s err: %v; raw=%s",
					path, operation, uerr, truncate(string(res.Data), 200))
				return nil
			}
		}
		return nil
	}
	if c.pool != nil {
		return c.pool.SubmitRead(ctx, operation, runWithCache, cacheFallback)
	}
	return runWithCache(ctx)
}

// doV1JSONDirect 纯直连 v1 API, 跳过 cache/pool.
func (c *Client) doV1JSONDirect(ctx context.Context, path string, out interface{}) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	body, err := c.session.GetRawJSON(ctx, path)
	if err != nil {
		if IsSessionAuthError(err) {
			c.session.invalidateForRelogin()
			if loginErr := c.session.ensureLoggedIn(); loginErr != nil {
				return fmt.Errorf("v1 get 失败，重新登录失败: %w，原始错误: %v", loginErr, err)
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			body, err = c.session.GetRawJSON(ctx, path)
		}
		if err != nil {
			return err
		}
	}
	if out == nil || len(body) == 0 {
		return nil
	}
	return unmarshalUseNumber(body, out)
}

// serveFromStaleCacheV1 pool 满降级: 查 cache 的 stale, 没有就 ErrPoolBusy.
func (c *Client) serveFromStaleCacheV1(
	ctx context.Context,
	entity sessionEntity,
	scope string,
	out interface{},
) error {
	if c.cache == nil {
		return ErrPoolBusy
	}
	_, err := c.cache.GetOrLoad(ctx, entity, scope, func(ctx context.Context) ([]byte, error) {
		return nil, fmt.Errorf("pool busy, served from stale")
	})
	if err == nil {
		return nil
	}
	// 任何 fallback 错误都视为 busy
	return ErrPoolBusy
}

// doSessionJSONCached 是 cache+pool 集成的主路径。
// pool 内的 fn 走 cache.GetOrLoad；cache 命中由 singleflight 保证只一个 goroutine 真正回源。
// pool 满 / 排队超时的 cacheFallback 直接查 cache 的 stale（不再调 loader）。
func (c *Client) doSessionJSONCached(
	ctx context.Context,
	operation, path string,
	entity sessionEntity,
	scope string,
	out interface{},
) error {
	loader := func(ctx context.Context) ([]byte, error) {
		raw, err := c.session.DoJSONRaw(ctx, path)
		if err != nil {
			return nil, err
		}
		return raw, nil
	}

	// 包装 loader 捕获 panic 转为 error，避免污染 pool worker
	//
	// 同时把"fatal session 错误"（登录失效 / 权限拒绝）wrap 上
	// errDoNotServeStale sentinel，让 SessionCache.doLoad 2c 路径跳过
	// L2 stale fallback，直接把原 err 透传给上层。否则 stale 兜底会让
	// 前端看到一个"旧空 list"，误以为"<user-account> 真的没产品"。
	safeLoader := func(ctx context.Context) (data []byte, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("session loader panicked: %v", r)
			}
		}()
		data, err = loader(ctx)
		if err != nil && (IsSessionAuthError(err) || IsSessionAccessDenied(err)) {
			err = fmt.Errorf("%w: %w", errDoNotServeStale, err)
		}
		return data, err
	}

	cacheFallback := func(ctx context.Context) error {
		// pool 满降级：直接查 cache 的 stale（甚至 L2 已经过期的也行）
		return c.serveFromStaleCache(ctx, entity, scope, out)
	}

	// 主流程：cache.GetOrLoad → 拿到 raw bytes → unmarshal 到 out。
	// 这样无论 cache hit 还是 miss，out 都被填一次。
	runWithCache := func(ctx context.Context) error {
		res, err := c.cache.GetOrLoad(ctx, entity, scope, safeLoader)
		if err != nil {
			return err
		}
		if out != nil && len(res.Data) > 0 {
			if uerr := unmarshalUseNumber(res.Data, out); uerr != nil {
				// 受限实例 SSO 限流时，部分端点（plans/testtasks/bugs/story 等）
				// 会返错端点的 body，导致 schema 不匹配 unmarshal 失败。
				// 这种情况下，与其让前端 500，不如返空 list 让页面正常展示空状态。
				// 但要 log warning 让 server 端能追查限流频率。
				log.Printf("[doSessionJSON] unmarshal fail (受限实例限流?): path=%s op=%s err: %v; raw=%s",
					path, operation, uerr, truncate(string(res.Data), 200))
				_ = uerr
				return nil // swallow, return zero-value out
			}
		}
		return nil
	}

	if c.pool != nil {
		return c.pool.SubmitRead(ctx, operation, runWithCache, cacheFallback)
	}
	return runWithCache(ctx)
}

// serveFromStaleCache 是 pool 满时的降级路径：
// 不调 loader（避免更拥堵），直接用 cache 已有的 stale 数据。
// 没有 stale 数据就返 ErrPoolBusy 让上层处理（前端展示 503）。
func (c *Client) serveFromStaleCache(
	ctx context.Context,
	entity sessionEntity,
	scope string,
	out interface{},
) error {
	if c.cache == nil {
		return ErrPoolBusy
	}
	// 用一个永远失败的 loader 触发 cache 内部 L2 stale fallback 路径
	_, err := c.cache.GetOrLoad(ctx, entity, scope, func(ctx context.Context) ([]byte, error) {
		return nil, fmt.Errorf("pool busy, served from stale")
	})
	if err == nil {
		return nil
	}
	// L2 也没 stale → 真无可用
	return ErrPoolBusy
}

// doSessionJSONUncached 与 doSessionJSON 相同，但绕过 SessionCache（仍走 pool 限流）。
//
// 用于分页实体的第 2+ 页：product-all / project-browse / execution-all 的各页 path
// 经 extractScopeFromPath 都得到相同 scope（首个数字段是 0 或固定值），若入缓存，
// 第 2+ 页会命中第 1 页的 30 分钟缓存，返回整页重复数据。第 1 页保持缓存（热路径），
// 续页直接回源。
func (c *Client) doSessionJSONUncached(ctx context.Context, operation, path string, out interface{}) error {
	if c.session == nil {
		return fmt.Errorf("会话客户端未初始化")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.pool != nil {
		return c.pool.SubmitRead(ctx, operation,
			func(ctx context.Context) error {
				return c.doSessionJSONWithRetry(ctx, operation, path, out)
			},
			nil, // 没有 cache 就不接 fallback
		)
	}
	return c.doSessionJSONWithRetry(ctx, operation, path, out)
}

// doSessionJSONWithRetry 是 doSessionJSON 去掉 cache/pool 包装的纯直连版。
// 鉴权失败时重新登录后重试一次。
func (c *Client) doSessionJSONWithRetry(ctx context.Context, operation, path string, out interface{}) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	err := c.session.DoJSON(ctx, path, out)
	if err == nil || !IsSessionAuthError(err) {
		return err
	}
	logger.Warn("Zentao session expired, re-login and retry",
		"operation", operation,
		"path", path,
		"error", err,
	)
	c.session.invalidateForRelogin()
	if loginErr := c.session.ensureLoggedIn(); loginErr != nil {
		return fmt.Errorf("%s失败，重新登录失败: %w，原始错误: %v", operation, loginErr, err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return c.session.DoJSON(ctx, path, out)
}

// doSessionPost 在会话模式下提交写操作表单。鉴权失效时重新登录后重试一次。
//
// Session 模式下：写必须真打到禅道（不走 cache），但走 pool 限流（写串行 1 worker
// 避免受限实例 SSO 标记异常）。成功后按 op 找 entity，cache.InvalidateEntity 强制
// 下次读回源。
func (c *Client) doSessionPost(ctx context.Context, operation, path string, form neturl.Values) error {
	if c.session == nil {
		return fmt.Errorf("会话客户端未初始化")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	doWrite := func(ctx context.Context) error {
		return c.doSessionPostWithRetry(ctx, operation, path, form)
	}

	// 写成功后的 cache 失效回调（在 pool worker 内执行，确保 invalidate 顺序在 write 后）
	var err error
	if c.pool != nil {
		err = c.pool.SubmitWrite(ctx, operation, func(ctx context.Context) error {
			if werr := doWrite(ctx); werr != nil {
				return werr
			}
			// 写成功 → 失效对应 entity 的 cache
			if c.cache != nil {
				if entity, ok := writeOpToEntity[operation]; ok {
					_ = c.cache.InvalidateEntity(ctx, entity)
				}
			}
			return nil
		})
	} else {
		err = doWrite(ctx)
		if err == nil && c.cache != nil {
			if entity, ok := writeOpToEntity[operation]; ok {
				_ = c.cache.InvalidateEntity(ctx, entity)
			}
		}
	}
	return err
}

// doSessionPostWithBody 与 doSessionPost 相同（pool 写串行 + 写后失效），
// 但返回响应体供调用方做业务结果穿透判定（HTML 响应不再默认成功）。
func (c *Client) doSessionPostWithBody(ctx context.Context, operation, path string, form neturl.Values) ([]byte, error) {
	if c.session == nil {
		return nil, fmt.Errorf("会话客户端未初始化")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var body []byte
	doWrite := func(ctx context.Context) error {
		b, err := c.doSessionPostWithRetryWithBody(ctx, operation, path, form)
		body = b
		return err
	}

	var err error
	if c.pool != nil {
		err = c.pool.SubmitWrite(ctx, operation, func(ctx context.Context) error {
			if werr := doWrite(ctx); werr != nil {
				return werr
			}
			if c.cache != nil {
				if entity, ok := writeOpToEntity[operation]; ok {
					_ = c.cache.InvalidateEntity(ctx, entity)
				}
			}
			return nil
		})
	} else {
		err = doWrite(ctx)
		if err == nil && c.cache != nil {
			if entity, ok := writeOpToEntity[operation]; ok {
				_ = c.cache.InvalidateEntity(ctx, entity)
			}
		}
	}
	return body, err
}

// doSessionPostWithRetryWithBody 是 doSessionPostWithRetry 的带响应体版本。
func (c *Client) doSessionPostWithRetryWithBody(ctx context.Context, operation, path string, form neturl.Values) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	body, err := c.session.PostFormWithBody(ctx, path, form)
	if err == nil || !IsSessionAuthError(err) {
		return body, err
	}
	logger.Warn("Zentao session expired on write, re-login and retry",
		"operation", operation,
		"path", path,
	)
	if err := c.session.ensureLoggedIn(); err != nil {
		return body, fmt.Errorf("写操作失败，重新登录失败: %w，原始错误: %v", err, err)
	}
	return c.session.PostFormWithBody(ctx, path, form)
}

// doSessionPostWithRetry 是 doSessionPost 去掉 cache/pool 包装的纯直连版。
func (c *Client) doSessionPostWithRetry(ctx context.Context, operation, path string, form neturl.Values) error {
	_, err := c.doSessionPostWithRetryWithBody(ctx, operation, path, form)
	return err
}

// writeOpToEntity 把写 op 名映射到 cache entity。
// 写成功后失效该 entity 的所有 scope cache。
//
// 规则：op 名按实体单词分（CreateBug/UpdateBug/DeleteBug/ResolveBug/CloseBug/...
// 都归 sessionEntityBugs）。
var writeOpToEntity = map[string]sessionEntity{
	// bugs
	"CreateBug": sessionEntityBugs, "UpdateBug": sessionEntityBugs, "DeleteBug": sessionEntityBugs,
	"ResolveBug": sessionEntityBugs, "CloseBug": sessionEntityBugs, "ConfirmBug": sessionEntityBugs,
	"AssignBug": sessionEntityBugs, "ActivateBug": sessionEntityBugs,
	// tasks
	"CreateTask": sessionEntityTasks, "UpdateTask": sessionEntityTasks, "DeleteTask": sessionEntityTasks,
	"StartTask": sessionEntityTasks, "FinishTask": sessionEntityTasks, "PauseTask": sessionEntityTasks,
	"AssignTask": sessionEntityTasks, "ActivateTask": sessionEntityTasks,
	// stories
	"CreateStory": sessionEntityStories, "UpdateStory": sessionEntityStories, "DeleteStory": sessionEntityStories,
	"ChangeStory": sessionEntityStories,
	// products / projects / programs / executions / builds / plans / releases
	"CreateProduct": sessionEntityProducts, "UpdateProduct": sessionEntityProducts, "DeleteProduct": sessionEntityProducts,
	"CreateProject": sessionEntityProjects, "UpdateProject": sessionEntityProjects, "DeleteProject": sessionEntityProjects,
	"CreateProgram": sessionEntityPrograms, "UpdateProgram": sessionEntityPrograms, "DeleteProgram": sessionEntityPrograms,
	"CreateExecution": sessionEntityExecutions, "UpdateExecution": sessionEntityExecutions, "DeleteExecution": sessionEntityExecutions,
	"CreateBuild": sessionEntityBuilds, "UpdateBuild": sessionEntityBuilds, "DeleteBuild": sessionEntityBuilds,
	"CreatePlan": sessionEntityPlans, "UpdatePlan": sessionEntityPlans, "DeletePlan": sessionEntityPlans,
	"LinkBugsToPlan": sessionEntityPlans, "UnlinkBugsFromPlan": sessionEntityPlans,
	"LinkStoriesToPlan": sessionEntityPlans, "UnlinkStoriesFromPlan": sessionEntityPlans,
	// cases
	"CreateCase": sessionEntityCases, "UpdateCase": sessionEntityCases, "DeleteCase": sessionEntityCases,
	// efforts
	"RecordEffort": sessionEntityEfforts,
	"BatchCreateEffort": sessionEntityEfforts,
	// users
	"CreateUser": sessionEntityUsers, "UpdateUser": sessionEntityUsers, "DeleteUser": sessionEntityUsers,
}

// opToCacheKey 把 doSessionJSON 的 (operation, path) 拆成 (entity, scope)。
// scope 是缓存的细粒度 key（all / byProduct:1029 / byExecution:42 等）。
// 当前用 path 的最后一段数字当 scope；不够精确但能区分跨产品的相同 op。
//
// Special case:
//   - testcase-browse 走分页段 URL（/testcase-browse-{pid}-0-all-{recTotal}-{recPerPage}-{page}.json），
//     同一 product 的不同 page 必须落到不同 cache entry, 把 page 编进 scope。
//   - v1 REST API (path 以 /api.php/v1 开头) 用 query string 翻页（?product=&page=&limit=），
//     不同 (product, page) 必须落到不同 cache entry, 整个 path 当 scope（path 包含 page/limit 区分）。
func opToCacheKey(operation, path string) (sessionEntity, string) {
	entity := opToEntity(operation)
	scope := extractScopeFromPath(path)
	if entity == sessionEntityCases && strings.HasPrefix(path, "/testcase-browse") {
		if page := extractTestcasePage(path); page > 0 {
			return entity, "byProduct:" + scopeNumberFrom(scope) + ":page:" + itoa(page)
		}
	}
	if strings.HasPrefix(path, "/api.php/v1/") {
		// v1 path 含完整 query string, 直接当 scope.
		return entity, path
	}
	return entity, scope
}

// extractTestcasePage 从 testcase-browse 的 path 提取 page 段。
//   - 裸路径 /testcase-browse-{productID}.json → 默认 page 1（首探）。
//   - 分页段 /testcase-browse-{pid}-{branch}-{browseType}-{recTotal}-{recPerPage}-{page}.json → 取最后一段数字。
//
// 实现: 统计 '-' 数量, 裸路径通常 2 个 dash, 分页段 7 个 dash。
// 只有 dash 数 >= 5 时才认为最后一段是 page, 否则裸路径最后那段是 productID。
func extractTestcasePage(path string) int {
	dashes := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '-' {
			dashes++
		}
	}
	if dashes < 5 {
		// 裸路径, 无法从 path 推断 page, 默认 1（首探）
		return 1
	}
	// 分页段: 最后一个 '-' 后面是 page
	idx := strings.LastIndex(path, "-")
	if idx < 0 || idx+1 >= len(path) {
		return 1
	}
	tail := path[idx+1:]
	tail = strings.TrimSuffix(tail, ".json")
	if tail == "" {
		return 1
	}
	for i := 0; i < len(tail); i++ {
		if tail[i] < '0' || tail[i] > '9' {
			return 1
		}
	}
	n := 0
	for i := 0; i < len(tail); i++ {
		n = n*10 + int(tail[i]-'0')
	}
	if n <= 0 || n > 10000 {
		return 1
	}
	return n
}

// scopeNumberFrom 从 extractScopeFromPath 返回的 "byScope:{n}" 提取数字。
// 找不到数字时返回 "0"。
func scopeNumberFrom(scope string) string {
	if strings.HasPrefix(scope, "byScope:") {
		return scope[len("byScope:"):]
	}
	return "0"
}

// itoa 简单 int 转 string (包内已有 strconv.Itoa, 这里保持单文件无外部依赖).
func itoa(n int) string { return strconv.Itoa(n) }

// opToEntity 把读 op 名映射到 sessionEntity。
func opToEntity(op string) sessionEntity {
	// 优先查 write 映射（write 用过，肯定覆盖读同前缀）
	if e, ok := writeOpToEntity[op]; ok {
		return e
	}
	// 读 op 命名规则：Get<Entities> → entities
	switch {
	case len(op) > 3 && op[:3] == "Get":
		rest := op[3:]
		// 处理复数：GetBugs → bugs, GetStories → stories
		switch rest {
		case "Bugs":
			return sessionEntityBugs
		case "Stories":
			return sessionEntityStories
		case "Tasks":
			return sessionEntityTasks
		case "Products":
			return sessionEntityProducts
		case "Projects":
			return sessionEntityProjects
		case "Programs":
			return sessionEntityPrograms
		case "Executions":
			return sessionEntityExecutions
		case "Builds":
			return sessionEntityBuilds
		case "Plans":
			return sessionEntityPlans
		case "Releases":
			return sessionEntityReleases
		case "Users":
			return sessionEntityUsers
		case "Cases":
			return sessionEntityCases
		case "TestTasks":
			return sessionEntityTestTasks
		case "Efforts":
			return sessionEntityEfforts
		case "CurrentUser":
			return sessionEntityUsers
		case "Dashboard":
			return sessionEntityDashboard
		}
	}
	// 未知 op 落到 "all" scope（cache key 不冲突即可）
	return sessionEntityBugs
}

// extractScopeFromPath 从 path 提取 scope 数字。
// 例子：/bug-browse-1029-0-all.json → "byProduct:1029"
//       /task-all-0-0--0-200-1.json → "all"（没明确产品）
//       /user-browse.json → "all"
//       /product-browse.json → "all"
func extractScopeFromPath(path string) string {
	// 简单策略：找 path 里第一个数字序列
	digits := ""
	for i := 0; i < len(path); i++ {
		if path[i] >= '0' && path[i] <= '9' {
			start := i
			for i < len(path) && path[i] >= '0' && path[i] <= '9' {
				digits += string(path[i])
				i++
			}
			_ = start
			if digits != "" && digits != "0" {
				return "byScope:" + digits
			}
			return "all"
		}
	}
	return "all"
}

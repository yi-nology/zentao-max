package zentao

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/oekit/concurrency/singleflight"
	_ "modernc.org/sqlite"
)

// sessionEntity 是 SessionCache 暴露给业务层的实体标识。
// 设计原则：业务侧只需声明"是什么实体"，不感知 session 命名空间。
type sessionEntity string

const (
	sessionEntityUsers      sessionEntity = "users"
	sessionEntityProducts   sessionEntity = "products"
	sessionEntityProjects   sessionEntity = "projects"
	sessionEntityExecutions sessionEntity = "executions"
	sessionEntityBuilds     sessionEntity = "builds"
	sessionEntityPlans      sessionEntity = "plans"
	sessionEntityBugs       sessionEntity = "bugs"
	sessionEntityStories    sessionEntity = "stories"
	sessionEntityTasks      sessionEntity = "tasks"
	sessionEntityPrograms   sessionEntity = "programs"
	sessionEntityReleases   sessionEntity = "releases"
	sessionEntityCases      sessionEntity = "cases"
	sessionEntityTestTasks  sessionEntity = "testtasks"
	sessionEntityEfforts    sessionEntity = "efforts"
	sessionEntityDashboard  sessionEntity = "dashboard"
)

// entityTTL 把每个 session entity 映射到 (freshTTL, staleTTL)。
//   - 基础数据（users/products/projects）变化慢，TTL 长；
//   - 动态数据（bugs/stories/tasks）变化快，TTL 短；
//   - staleTTL ≥ 1h，保证离线场景下至少能用一份合理数据兜底。
//
// 调整时同步改 session_cache_test.go 里的 entityTTLForTest 用例。
var entityTTL = map[sessionEntity]struct{ Fresh, Stale time.Duration }{
	sessionEntityUsers:      {30 * time.Minute, 24 * time.Hour},
	sessionEntityProducts:   {30 * time.Minute, 24 * time.Hour},
	sessionEntityProjects:   {30 * time.Minute, 24 * time.Hour},
	sessionEntityPrograms:   {30 * time.Minute, 24 * time.Hour},
	sessionEntityReleases:   {30 * time.Minute, 24 * time.Hour},
	sessionEntityExecutions: {10 * time.Minute, 6 * time.Hour},
	sessionEntityBuilds:     {10 * time.Minute, 6 * time.Hour},
	sessionEntityPlans:      {10 * time.Minute, 6 * time.Hour},
	sessionEntityBugs:       {2 * time.Minute, 1 * time.Hour},
	sessionEntityStories:    {2 * time.Minute, 1 * time.Hour},
	sessionEntityTasks:      {2 * time.Minute, 1 * time.Hour},
	sessionEntityCases:      {5 * time.Minute, 1 * time.Hour},
	sessionEntityTestTasks:  {5 * time.Minute, 1 * time.Hour},
	sessionEntityEfforts:    {5 * time.Minute, 1 * time.Hour},
	sessionEntityDashboard:  {5 * time.Minute, 1 * time.Hour},
}

// sessionCacheKey 是 SessionCache 的内部完整 key。
//   - ns    : (server, account, realm) 命名空间三元组的哈希；
//   - entity: 业务实体名；
//   - scope : 进一步区分的维度（"all" / "byProduct:1029" / "byExecution:42"）。
type sessionCacheKey struct {
	ns     string
	entity sessionEntity
	scope  string
}

func (k sessionCacheKey) String() string {
	return k.ns + "|" + string(k.entity) + "|" + k.scope
}

// l1Entry 是 L1 in-memory cache 的单条记录。
type l1Entry struct {
	data   []byte
	expiry time.Time
	loadAt time.Time
}

// l1Cache 是一个固定容量的内存 cache。超过 cap 时整体清空（FIFO 简化版）。
// Session 模式实际 key 数量级 ~ 几十到几百，远低于 1000 cap，完整 LRU 收益不显著。
type l1Cache struct {
	mu      sync.RWMutex
	entries map[sessionCacheKey]*l1Entry
	cap     int
}

func newL1Cache(cap int) *l1Cache {
	if cap <= 0 {
		cap = 1000
	}
	return &l1Cache{entries: make(map[sessionCacheKey]*l1Entry, cap), cap: cap}
}

func (c *l1Cache) Get(k sessionCacheKey) (*l1Entry, bool) {
	c.mu.RLock()
	e, ok := c.entries[k]
	c.mu.RUnlock()
	return e, ok
}

func (c *l1Cache) Set(k sessionCacheKey, e *l1Entry) {
	c.mu.Lock()
	if len(c.entries) >= c.cap {
		c.entries = make(map[sessionCacheKey]*l1Entry, c.cap)
	}
	c.entries[k] = e
	c.mu.Unlock()
}

func (c *l1Cache) Delete(k sessionCacheKey) {
	c.mu.Lock()
	delete(c.entries, k)
	c.mu.Unlock()
}

func (c *l1Cache) DeleteByEntity(ns string, entity sessionEntity) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	deleted := 0
	for k := range c.entries {
		if k.ns == ns && k.entity == entity {
			delete(c.entries, k)
			deleted++
		}
	}
	return deleted
}

func (c *l1Cache) ClearNamespace(ns string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	deleted := 0
	for k := range c.entries {
		if k.ns == ns {
			delete(c.entries, k)
			deleted++
		}
	}
	return deleted
}

// SessionCache 是 Session 模式专属的二级 cache。
//   - L1：in-memory，同进程内多端点共享，O(1) Get/Set；
//   - L2：SQLite 持久化（自管表，复用 modernc.org/sqlite 无 CGO 依赖），进程重启不丢 + 离线 fallback；
//   - 命名空间：所有 key 带 (server, account, realm) 三元组，切换账号自动隔离；
//   - singleflight：同 key 并发请求只触发一次 loader，防击穿；
//   - 写后失效：提供 Invalidate / InvalidateEntity / InvalidateAll 接口，由 doSessionPost 触发。
//
// 与 service.CacheService 的关系：
//   - service.CacheService 在业务层（VO 序列化后），per (entityType, productID)；
//   - SessionCache 在 client 层（原始端点响应），per (entity, scope)；
//   - 两层都存在但职责不同，详见 AGENTS.md "SessionCache / SessionWorkerPool" 章节。
type SessionCache struct {
	db *sql.DB // 可为 nil：纯 L1 + 内存 loader（适合单测 / 嵌入式）
	l1 *l1Cache
	sf singleflight.Flight[*CacheResult]

	// 命名空间
	server  string
	account string
	realm   string
	ns      string

	now func() time.Time
}

// SessionCacheConfig 构造 SessionCache 的配置。
//   - DBPrefix 必填：用 <DBPrefix>-session.db 作为 L2 文件名；
//   - L1Cap 0 = 默认 1000。
type SessionCacheConfig struct {
	Server   string
	Account  string
	Realm    string
	DBPrefix string // 例如 "~/.zentao-max/cache" → "~/.zentao-max/cache-session.db"
	L1Cap    int
}

// NewSessionCache 构造一个 Session 专属 cache。server/account 任一为空返回 nil
// （nil-safe 语义：调用方看到 nil 就跳过 cache 走直连）。
// DBPrefix 为空时只用 L1（适合单元测试）。
func NewSessionCache(cfg SessionCacheConfig) *SessionCache {
	if cfg.Server == "" || cfg.Account == "" {
		return nil
	}
	sc := &SessionCache{
		l1:      newL1Cache(cfg.L1Cap),
		server:  cfg.Server,
		account: cfg.Account,
		realm:   cfg.Realm,
		ns:      makeNamespace(cfg.Server, cfg.Account, cfg.Realm),
		now:     time.Now,
	}
	if cfg.DBPrefix != "" {
		db, err := openSessionCacheDB(cfg.DBPrefix)
		if err != nil {
			log.Printf("[SessionCache] L2 disabled, falling back to L1-only: prefix=%s err=%v", cfg.DBPrefix, err)
		} else {
			sc.db = db
		}
	}
	return sc
}

// Close 关闭 L2 SQLite。Client 退出时调用。
func (c *SessionCache) Close() error {
	if c == nil || c.db == nil {
		return nil
	}
	return c.db.Close()
}

// makeNamespace 把 (server, account, realm) 拼成短哈希，避免在 SQLite key 里泄漏账号原文。
func makeNamespace(server, account, realm string) string {
	h := sha1.New()
	h.Write([]byte(server))
	h.Write([]byte{0})
	h.Write([]byte(account))
	h.Write([]byte{0})
	h.Write([]byte(realm))
	return "ns:" + hex.EncodeToString(h.Sum(nil)[:8])
}

func (c *SessionCache) makeKey(entity sessionEntity, scope string) sessionCacheKey {
	if scope == "" {
		scope = "all"
	}
	return sessionCacheKey{ns: c.ns, entity: entity, scope: scope}
}

// sessionCacheSchema：L2 自管表，key 完整 (ns, entity, scope) 三元组。
const sessionCacheSchema = `
CREATE TABLE IF NOT EXISTS session_cache (
    ns         TEXT NOT NULL,
    entity     TEXT NOT NULL,
    scope      TEXT NOT NULL,
    data       BLOB NOT NULL,
    cached_at  INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    PRIMARY KEY (ns, entity, scope)
);
CREATE INDEX IF NOT EXISTS idx_session_cache_ns_entity ON session_cache(ns, entity);
CREATE INDEX IF NOT EXISTS idx_session_cache_expires ON session_cache(expires_at);
`

// openSessionCacheDB 打开/创建 session cache 专用的 SQLite 文件。
// 复用项目里 storage.Store 的 pure-Go 驱动，避免引入 CGO。
func openSessionCacheDB(prefix string) (*sql.DB, error) {
	if prefix == "" {
		return nil, errors.New("empty DBPrefix")
	}
	dbPath := prefix + "-session.db"
	if dir := filepath.Dir(dbPath); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create dir: %w", err)
		}
	}
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.ExecContext(context.Background(), sessionCacheSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}
	return db, nil
}

// CacheResult 是 GetOrLoad 的返回。
//   - FromCache=true & Stale=false：来自 L1 或 L2 新鲜数据；
//   - FromCache=true & Stale=true：来自 L2 过期数据（loader 失败时的 fallback）；
//   - FromCache=false & err==nil：来自 loader 当次回源；
//   - err != nil：完全没数据。
type CacheResult struct {
	Data      []byte
	FromCache bool
	Stale     bool
	CachedAt  time.Time
}

// Loader 从禅道拉取原始数据的回调。返回错误时 SessionCache 会尝试 stale fallback。
type Loader func(ctx context.Context) ([]byte, error)

// errDoNotServeStale 是 SessionCache 的内部 sentinel：
// 当 loader 失败原因属于"硬错误"（禅道登录失效 / 权限拒绝）时，
// doLoad 2c 路径应该跳过 L2 stale fallback，直接把原错误抛给上层，
// 而不是用旧数据兜底让调用方误以为"成功了但返回了空数据"。
//
// 触发场景（受限实例 SSO 实测）：用户账号被风控限流，server login 失败，
// 端点返 "密码尝试次数太多"。如果这时候走 stale fallback，前端 product 下拉
// 会显示一个旧时刻的"空 list"，看起来像"<user-account> 真的没产品"，调试时极难
// 区分。
//
// 区分原则：
//   - 网络瞬断 / 5xx 短暂错误 → 走 stale fallback（设计意图）
//   - 登录失效 / 权限拒绝 → 必须把错误透出，调用方需要知道"现在没数据是因为没登录"
//
// 用法：doSessionJSONCached 的 safeLoader 包装时检测 IsSessionAuthError /
// IsSessionAccessDenied，wrap 这个 sentinel；doLoad 用 errors.Is 识别并跳过
// stale fallback。
var errDoNotServeStale = errors.New("do not serve stale for fatal session error")

// GetOrLoad 是主入口：按 L1 → L2 → loader 顺序查找，每一层失败都自动降级。
// 并发同 key 只会触发一次 loader（singleflight）。
// ctx 取消会立即返回 ctx.Err()，不会触发任何 loader / cache 写。
func (c *SessionCache) GetOrLoad(ctx context.Context, entity sessionEntity, scope string, loader Loader) (*CacheResult, error) {
	if c == nil {
		data, err := loader(ctx)
		if err != nil {
			return nil, err
		}
		return &CacheResult{Data: data, FromCache: false}, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	key := c.makeKey(entity, scope)

	// 1. L1 hit 且新鲜
	if e, ok := c.l1.Get(key); ok {
		if c.now().Before(e.expiry) {
			return &CacheResult{
				Data:      e.data,
				FromCache: true,
				CachedAt:  e.loadAt,
			}, nil
		}
	}

	// 2. singleflight：同 key 只触发一次 loader（oekit 泛型 Flight，ctx 取消感知）
	return c.sf.Do(ctx, key.String(), func(ctx context.Context) (*CacheResult, error) {
		return c.doLoad(ctx, entity, key, loader)
	})
}

func (c *SessionCache) doLoad(ctx context.Context, entity sessionEntity, key sessionCacheKey, loader Loader) (*CacheResult, error) {
	// 2a. L2 fresh hit
	if c.db != nil {
		data, cachedAt, expired, l2err := c.l2Get(ctx, key, false)
		if l2err == nil && data != nil && !expired {
			c.l1.Set(key, &l1Entry{data: data, expiry: c.now().Add(ttlFresh(entity)), loadAt: cachedAt})
			return &CacheResult{Data: data, FromCache: true, CachedAt: cachedAt}, nil
		}
	}

	// 2b. loader 回源
	data, err := loader(ctx)
	if err == nil {
		now := c.now()
		c.l1.Set(key, &l1Entry{data: data, expiry: now.Add(ttlFresh(entity)), loadAt: now})
		if c.db != nil {
			if werr := c.l2Set(ctx, key, data, ttlFresh(entity)); werr != nil {
				log.Printf("[SessionCache] L2 write failed: ns=%s entity=%s scope=%s err=%v",
					key.ns, key.entity, key.scope, werr)
			}
		}
		return &CacheResult{Data: data, FromCache: false}, nil
	}

	// 2c. loader 失败，尝试 L2 stale fallback
	//
	// 跳过规则：fatal session 错误（登录失效 / 权限拒绝）必须透传给上层，
	// 不能用 stale 数据兜底，否则前端看到"空 list"会误以为是真实数据。
	// 详情见 errDoNotServeStale 注释。
	if c.db != nil && !errors.Is(err, errDoNotServeStale) {
		staleData, staleAt, _, serr := c.l2Get(ctx, key, true)
		if serr == nil && staleData != nil {
			log.Printf("[SessionCache] loader failed, serving L2 stale fallback: ns=%s entity=%s scope=%s cachedAt=%s err=%v",
				key.ns, key.entity, key.scope, staleAt.Format(time.RFC3339), err)
			// L1 也写一份（带 stale 过期时间），让后续请求能直接命中
			c.l1.Set(key, &l1Entry{
				data:   staleData,
				expiry: c.now().Add(ttlStale(entity)),
				loadAt: staleAt,
			})
			return &CacheResult{Data: staleData, FromCache: true, Stale: true, CachedAt: staleAt}, nil
		}
	}

	// 2d. 完全无数据
	return nil, err
}

func (c *SessionCache) l2Get(ctx context.Context, key sessionCacheKey, allowStale bool) ([]byte, time.Time, bool, error) {
	var data []byte
	var cachedAtUnix, expiresAtUnix int64
	err := c.db.QueryRowContext(ctx,
		`SELECT data, cached_at, expires_at FROM session_cache
		 WHERE ns = ? AND entity = ? AND scope = ?`,
		key.ns, string(key.entity), key.scope,
	).Scan(&data, &cachedAtUnix, &expiresAtUnix)
	if err == sql.ErrNoRows {
		return nil, time.Time{}, false, nil
	}
	if err != nil {
		return nil, time.Time{}, false, err
	}
	expired := c.now().Unix() > expiresAtUnix
	if expired && !allowStale {
		return nil, time.Time{}, true, nil
	}
	return data, time.Unix(cachedAtUnix, 0), expired, nil
}

func (c *SessionCache) l2Set(ctx context.Context, key sessionCacheKey, data []byte, ttl time.Duration) error {
	now := c.now()
	_, err := c.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO session_cache (ns, entity, scope, data, cached_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		key.ns, string(key.entity), key.scope, data, now.Unix(), now.Add(ttl).Unix(),
	)
	return err
}

func (c *SessionCache) l2Delete(ctx context.Context, key sessionCacheKey) error {
	_, err := c.db.ExecContext(ctx,
		`DELETE FROM session_cache WHERE ns = ? AND entity = ? AND scope = ?`,
		key.ns, string(key.entity), key.scope,
	)
	return err
}

func (c *SessionCache) l2DeleteByEntity(ctx context.Context, ns string, entity sessionEntity) error {
	_, err := c.db.ExecContext(ctx,
		`DELETE FROM session_cache WHERE ns = ? AND entity = ?`,
		ns, string(entity),
	)
	return err
}

func (c *SessionCache) l2DeleteByNamespace(ctx context.Context, ns string) error {
	_, err := c.db.ExecContext(ctx,
		`DELETE FROM session_cache WHERE ns = ?`,
		ns,
	)
	return err
}

func ttlFresh(e sessionEntity) time.Duration {
	if v, ok := entityTTL[e]; ok {
		return v.Fresh
	}
	return 5 * time.Minute
}

func ttlStale(e sessionEntity) time.Duration {
	if v, ok := entityTTL[e]; ok {
		return v.Stale
	}
	return 1 * time.Hour
}

// Invalidate 删除指定 (entity, scope) 的 L1 + L2 cache。
// 写操作（doSessionPost）成功后调用，强制下次 GetOrLoad 回源。
func (c *SessionCache) Invalidate(ctx context.Context, entity sessionEntity, scope string) error {
	if c == nil {
		return nil
	}
	key := c.makeKey(entity, scope)
	c.l1.Delete(key)
	if c.db != nil {
		return c.l2Delete(ctx, key)
	}
	return nil
}

// InvalidateEntity 删除某个 entity 的所有 scope（如 ResolveBug 后让所有 bugs:* 都失效）。
func (c *SessionCache) InvalidateEntity(ctx context.Context, entity sessionEntity) error {
	if c == nil {
		return nil
	}
	c.l1.DeleteByEntity(c.ns, entity)
	if c.db != nil {
		return c.l2DeleteByEntity(ctx, c.ns, entity)
	}
	return nil
}

// InvalidateAll 清空当前 session 命名空间下的所有 cache（L1 + L2）。
// UpdateSessionConfig 切换账号/服务器时调用。
func (c *SessionCache) InvalidateAll(ctx context.Context) error {
	if c == nil {
		return nil
	}
	c.l1.ClearNamespace(c.ns)
	if c.db != nil {
		return c.l2DeleteByNamespace(ctx, c.ns)
	}
	return nil
}

// UpdateNamespace 在 UpdateSessionConfig 时调用：更新 server/account/realm，
// 旧的 L1 cache 自动失效（命名空间变了，旧 key 不会被命中）。
// L2 不主动清（不再被访问，但保留以便将来快速切回；空间成本可接受）。
func (c *SessionCache) UpdateNamespace(server, account, realm string) {
	if c == nil {
		return
	}
	if server == "" || account == "" {
		return
	}
	newNS := makeNamespace(server, account, realm)
	if newNS == c.ns {
		return
	}
	c.l1.ClearNamespace(c.ns)
	c.server = server
	c.account = account
	c.realm = realm
	c.ns = newNS
}

// Stats 返回 cache 统计（仅用于监控 / debug）。
type SessionCacheStats struct {
	L1Size    int
	L2Enabled bool
	Server    string
	Account   string
	Realm     string
	Namespace string
}

func (c *SessionCache) Stats() SessionCacheStats {
	if c == nil {
		return SessionCacheStats{}
	}
	c.l1.mu.RLock()
	size := len(c.l1.entries)
	c.l1.mu.RUnlock()
	return SessionCacheStats{
		L1Size:    size,
		L2Enabled: c.db != nil,
		Server:    c.server,
		Account:   c.account,
		Realm:     c.realm,
		Namespace: c.ns,
	}
}

// Package storage 提供本地 SQLite 持久化存储，用于离线缓存禅道数据
package storage

import (
	"context"
	crand "crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动，无 CGO 依赖
)

// Store 离线缓存存储抽象
type Store interface {
	// Get 读取未过期的缓存数据。data 是反序列化后的 JSON。cachedAt 是缓存时间。
	// 未命中或已过期返回 nil, nil（不报错）
	Get(ctx context.Context, entityType string, productID int) (data []byte, cachedAt time.Time, err error)

	// GetStale 读取本进程写入的缓存（不管是否过期）：SWR「过期先返旧值 +
	// 后台刷新」用。跨进程（上次启动）写入的条目不在此范围——旧进程的数据
	// 可能来自当时的上游故障/部分爬取，直接当 SWR 旧值返回会污染用户可见
	// 数字（真实案例：重启后 dashboard bugs=0/stories=旧值持续 30 分钟）。
	GetStale(ctx context.Context, entityType string, productID int) (data []byte, cachedAt time.Time, err error)

	// GetAnyStale 读取任意进程写入的缓存（不管是否过期），仅用于 loader
	// 失败时的离线兜底——离线场景没有更好的数据，旧进程的缓存仍有价值。
	GetAnyStale(ctx context.Context, entityType string, productID int) (data []byte, cachedAt time.Time, err error)

	// Set 写入缓存数据（JSON bytes），过期时间为 now + ttl
	Set(ctx context.Context, entityType string, productID int, data []byte, ttl time.Duration) error

	// Invalidate 使指定实体的缓存失效（删除）
	Invalidate(ctx context.Context, entityType string, productID int) error

	// ClearAll 清空所有缓存
	ClearAll(ctx context.Context) error

	// Status 返回缓存统计（条目数、字节数、最后更新时间）
	Status(ctx context.Context) (*CacheStatus, error)

	// Close 关闭数据库
	Close() error
}

// CacheStatus 缓存状态
type CacheStatus struct {
	EntryCount   int       `json:"entryCount"`
	TotalBytes   int64     `json:"totalBytes"`
	LastUpdateAt time.Time `json:"lastUpdateAt"`
	DBPath       string    `json:"dbPath"`
}

// SQLiteStore 基于 SQLite 的离线缓存实现
type SQLiteStore struct {
	db     *sql.DB
	path   string
	bootID string // 本进程代际 ID：区分「本次启动写入」与「上次启动写入」的条目
	mu     sync.RWMutex
}

// NewSQLiteStore 创建一个新的 SQLite 缓存存储
// dbPath 为空时默认使用 ~/.zentao-max/cache.db
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	if dbPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("获取用户目录失败: %w", err)
		}
		dbPath = filepath.Join(home, ".zentao-max", "cache.db")
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("创建数据库目录失败: %w", err)
	}

	// 使用纯 Go 驱动，DSN 加超时和 WAL 模式优化并发
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite 失败: %w", err)
	}

	// SQLite 单写入并发优化
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	store := &SQLiteStore{db: db, path: dbPath, bootID: newBootID()}
	if err := store.initSchema(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

// newBootID 生成本进程代际 ID（启动随机串）。
// 用途见 GetStale/GetAnyStale 注释：跨重启的旧缓存不得作为 SWR「旧值」
// 直接返回，只能作离线兜底。
func newBootID() string {
	var b [8]byte
	if _, err := crand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS cache_entries (
    entity_type TEXT NOT NULL,
    product_id  INTEGER NOT NULL,
    data        BLOB NOT NULL,
    cached_at   INTEGER NOT NULL,
    expires_at  INTEGER NOT NULL,
    boot_id     TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (entity_type, product_id)
);

CREATE INDEX IF NOT EXISTS idx_cache_expires ON cache_entries(expires_at);
CREATE INDEX IF NOT EXISTS idx_cache_product ON cache_entries(product_id);
`

func (s *SQLiteStore) initSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, schemaSQL)
	if err != nil {
		return fmt.Errorf("初始化 schema 失败: %w", err)
	}
	// 迁移：老库（v0.10.1 及之前）没有 boot_id 列 → 补列，旧条目 boot_id=''，
	// 自然被「本进程代际」过滤掉（只可作离线兜底）。
	var hasBoot bool
	rows, qerr := s.db.QueryContext(ctx, `PRAGMA table_info(cache_entries)`)
	if qerr == nil {
		defer rows.Close()
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		for rows.Next() {
			if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
				break
			}
			if name == "boot_id" {
				hasBoot = true
			}
		}
	}
	if qerr == nil && !hasBoot {
		if _, aerr := s.db.ExecContext(ctx, `ALTER TABLE cache_entries ADD COLUMN boot_id TEXT NOT NULL DEFAULT ''`); aerr != nil {
			return fmt.Errorf("迁移 boot_id 列失败: %w", aerr)
		}
	}
	return nil
}

// Get 读取缓存（仅未过期，且必须是本进程写入）
func (s *SQLiteStore) Get(ctx context.Context, entityType string, productID int) ([]byte, time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var data []byte
	var cachedAtUnix, expiresAtUnix int64
	err := s.db.QueryRowContext(ctx,
		`SELECT data, cached_at, expires_at FROM cache_entries
		 WHERE entity_type = ? AND product_id = ? AND boot_id = ?`,
		entityType, productID, s.bootID,
	).Scan(&data, &cachedAtUnix, &expiresAtUnix)

	if err == sql.ErrNoRows {
		return nil, time.Time{}, nil
	}
	if err != nil {
		return nil, time.Time{}, err
	}

	// 检查是否过期
	if time.Now().Unix() > expiresAtUnix {
		return nil, time.Time{}, nil
	}
	return data, time.Unix(cachedAtUnix, 0), nil
}

// GetStale 读取本进程写入的缓存（不管是否过期）：SWR 旧值语义
func (s *SQLiteStore) GetStale(ctx context.Context, entityType string, productID int) ([]byte, time.Time, error) {
	return s.getStale(ctx, entityType, productID, true)
}

// GetAnyStale 读取任意进程写入的缓存（不管是否过期），仅离线兜底使用
func (s *SQLiteStore) GetAnyStale(ctx context.Context, entityType string, productID int) ([]byte, time.Time, error) {
	return s.getStale(ctx, entityType, productID, false)
}

func (s *SQLiteStore) getStale(ctx context.Context, entityType string, productID int, sameBootOnly bool) ([]byte, time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var data []byte
	var cachedAtUnix, expiresAtUnix int64
	query := `SELECT data, cached_at, expires_at FROM cache_entries
		 WHERE entity_type = ? AND product_id = ?`
	args := []interface{}{entityType, productID}
	if sameBootOnly {
		query += ` AND boot_id = ?`
		args = append(args, s.bootID)
	}
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&data, &cachedAtUnix, &expiresAtUnix)

	if err == sql.ErrNoRows {
		return nil, time.Time{}, nil
	}
	if err != nil {
		return nil, time.Time{}, err
	}
	// 注意：忽略 expiresAtUnix，故意返回过期数据
	_ = expiresAtUnix
	return data, time.Unix(cachedAtUnix, 0), nil
}

// Set 写入缓存
func (s *SQLiteStore) Set(ctx context.Context, entityType string, productID int, data []byte, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	cachedAt := now.Unix()
	expiresAt := now.Add(ttl).Unix()

	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO cache_entries (entity_type, product_id, data, cached_at, expires_at, boot_id)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		entityType, productID, data, cachedAt, expiresAt, s.bootID,
	)
	return err
}

// Invalidate 删除指定缓存
func (s *SQLiteStore) Invalidate(ctx context.Context, entityType string, productID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM cache_entries WHERE entity_type = ? AND product_id = ?`,
		entityType, productID,
	)
	return err
}

// ClearAll 清空所有缓存
func (s *SQLiteStore) ClearAll(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `DELETE FROM cache_entries`)
	return err
}

// Status 返回缓存统计
func (s *SQLiteStore) Status(ctx context.Context) (*CacheStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var entryCount int
	var totalBytes sql.NullInt64
	var lastUpdateUnix sql.NullInt64

	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*),
		        COALESCE(SUM(LENGTH(data)), 0),
		        MAX(cached_at)
		 FROM cache_entries`,
	).Scan(&entryCount, &totalBytes, &lastUpdateUnix)
	if err != nil {
		return nil, err
	}

	status := &CacheStatus{
		EntryCount: entryCount,
		TotalBytes: totalBytes.Int64,
		DBPath:     s.path,
	}
	if lastUpdateUnix.Valid {
		status.LastUpdateAt = time.Unix(lastUpdateUnix.Int64, 0)
	}
	return status, nil
}

// Close 关闭数据库
func (s *SQLiteStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// EntityType 常量，定义支持的缓存实体类型
const (
	EntityBugs      = "bugs"
	EntityStories   = "stories"
	EntityTasks     = "tasks"
	EntityDashboard = "dashboard"
)

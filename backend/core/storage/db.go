// Package storage 提供本地 SQLite 持久化存储，用于离线缓存禅道数据
package storage

import (
	"context"
	"database/sql"
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

	// GetStale 读取缓存数据（不管是否过期），用于离线 fallback
	// 未命中返回 nil, nil
	GetStale(ctx context.Context, entityType string, productID int) (data []byte, cachedAt time.Time, err error)

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
	db   *sql.DB
	path string
	mu   sync.RWMutex
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

	store := &SQLiteStore{db: db, path: dbPath}
	if err := store.initSchema(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS cache_entries (
    entity_type TEXT NOT NULL,
    product_id  INTEGER NOT NULL,
    data        BLOB NOT NULL,
    cached_at   INTEGER NOT NULL,
    expires_at  INTEGER NOT NULL,
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
	return nil
}

// Get 读取缓存（仅未过期）
func (s *SQLiteStore) Get(ctx context.Context, entityType string, productID int) ([]byte, time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var data []byte
	var cachedAtUnix, expiresAtUnix int64
	err := s.db.QueryRowContext(ctx,
		`SELECT data, cached_at, expires_at FROM cache_entries
		 WHERE entity_type = ? AND product_id = ?`,
		entityType, productID,
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

// GetStale 读取缓存（不管是否过期），用于离线 fallback
func (s *SQLiteStore) GetStale(ctx context.Context, entityType string, productID int) ([]byte, time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var data []byte
	var cachedAtUnix, expiresAtUnix int64
	err := s.db.QueryRowContext(ctx,
		`SELECT data, cached_at, expires_at FROM cache_entries
		 WHERE entity_type = ? AND product_id = ?`,
		entityType, productID,
	).Scan(&data, &cachedAtUnix, &expiresAtUnix)

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
		`INSERT OR REPLACE INTO cache_entries (entity_type, product_id, data, cached_at, expires_at)
		 VALUES (?, ?, ?, ?, ?)`,
		entityType, productID, data, cachedAt, expiresAt,
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

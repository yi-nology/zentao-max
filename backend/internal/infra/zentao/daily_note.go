package zentao

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// DailyNote 是用户在某天写的"今天干了什么"自由文本总结。
// 数据归本项目所有，跟禅道 effort (task 维度) 互补：effort 是"今天 X 任务 Y 小时"，
// daily note 是"今天整体感受 / 非 task 工作 / 明日计划"。
//
// 存储：自管 SQLite 表 daily_notes，复用 modernc.org/sqlite 无 CGO。
// 位置：DBPrefix + "-notes.db"（DBPrefix 跟 SessionCache 共享 "~/.zentao-max/cache"）。
// 命名空间：完全按 (user_account, date) 隔离；session 切账号不需要手动清（key 含 account）。
type DailyNote struct {
	User      string    `json:"user"`      // 禅道账号，如 <user-account>
	Date      string    `json:"date"`      // YYYY-MM-DD
	Content   string    `json:"content"`   // 自由文本，可空
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// DailyNoteStore 是 daily note 的 SQLite 持久化层。
// 线程安全；nil-safe（构造失败时 nil，调用方 nil 检查后走纯内存模式或报错）。
type DailyNoteStore struct {
	db *sql.DB
	mu sync.Mutex
}

const dailyNoteSchema = `
CREATE TABLE IF NOT EXISTS daily_notes (
    user_account TEXT NOT NULL,
    date         TEXT NOT NULL,
    content      TEXT NOT NULL DEFAULT '',
    zentao_effort_id INTEGER NOT NULL DEFAULT 0,
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL,
    PRIMARY KEY (user_account, date)
);
CREATE INDEX IF NOT EXISTS idx_daily_notes_user_date ON daily_notes(user_account, date);

-- 每个 user 1 个 special "📓 Daily Notes" task，zentao effort 挂到这个 task。
CREATE TABLE IF NOT EXISTS daily_note_tasks (
    user_account TEXT PRIMARY KEY,
    task_id      INTEGER NOT NULL,
    task_name    TEXT NOT NULL,
    created_at   INTEGER NOT NULL
);
`

// NewDailyNoteStore 打开/创建 daily note 专用 SQLite 文件。
// DBPrefix 为空时使用默认路径 ~/.zentao-max/cache-notes.db（跟 storage.NewSQLiteStore
// 共享 home 目录，文件名不同避免冲突）。
func NewDailyNoteStore(dbPrefix string) (*DailyNoteStore, error) {
	var dbPath string
	if dbPrefix == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home dir: %w", err)
		}
		dbPath = filepath.Join(home, ".zentao-max", "cache-notes.db")
	} else {
		dbPath = dbPrefix + "-notes.db"
	}
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
	if _, err := db.ExecContext(context.Background(), dailyNoteSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}
	return &DailyNoteStore{db: db}, nil
}

// Close 关闭 SQLite。App 退出时调用。
func (s *DailyNoteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// GetNote 读单条 note。user+date 任一为空返 (nil, nil)，不报错。
func (s *DailyNoteStore) GetNote(ctx context.Context, user, date string) (*DailyNote, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if user == "" || date == "" {
		return nil, nil
	}
	row := s.db.QueryRowContext(ctx,
		`SELECT user_account, date, content, zentao_effort_id, created_at, updated_at
		 FROM daily_notes WHERE user_account = ? AND date = ?`,
		user, date,
	)
	n := &DailyNote{}
	var createdAt, updatedAt int64
	var effortID int
	if err := row.Scan(&n.User, &n.Date, &n.Content, &effortID, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	n.CreatedAt = time.Unix(createdAt, 0)
	n.UpdatedAt = time.Unix(updatedAt, 0)
	// zentao_effort_id 不暴露在 VO（前端不需要）；保留数据库字段用于同步追踪
	return n, nil
}

// SetNote upsert。content 空字符串也允许（视为"今天没特别想写"，保留空行）。
// zentaoEffortID 是 zentao 端 effort 记录的 id（>0 表已同步；0 表只本地）。
func (s *DailyNoteStore) SetNote(ctx context.Context, user, date, content string, zentaoEffortID int) (*DailyNote, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("daily note store not initialized")
	}
	if user == "" || date == "" {
		return nil, errors.New("user and date are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	// upsert: 保留原 created_at，仅更新 content + updated_at
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO daily_notes (user_account, date, content, zentao_effort_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(user_account, date) DO UPDATE SET
		   content = excluded.content,
		   zentao_effort_id = excluded.zentao_effort_id,
		   updated_at = excluded.updated_at`,
		user, date, content, zentaoEffortID, now, now,
	)
	if err != nil {
		return nil, err
	}
	return &DailyNote{
		User: user, Date: date, Content: content,
		CreatedAt: time.Unix(now, 0), UpdatedAt: time.Unix(now, 0),
	}, nil
}

// SetZentaoEffortID 更新某条 note 的 zentao_effort_id（异步同步完后回填）。
func (s *DailyNoteStore) SetZentaoEffortID(ctx context.Context, user, date string, effortID int) error {
	if s == nil || s.db == nil {
		return errors.New("daily note store not initialized")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`UPDATE daily_notes SET zentao_effort_id = ? WHERE user_account = ? AND date = ?`,
		effortID, user, date,
	)
	return err
}

// DeleteNote 删一条 note。user+date 不存在时返 nil, nil（幂等）。
func (s *DailyNoteStore) DeleteNote(ctx context.Context, user, date string) error {
	if s == nil || s.db == nil {
		return nil
	}
	if user == "" || date == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM daily_notes WHERE user_account = ? AND date = ?`,
		user, date,
	)
	return err
}

// ListNotesByMonth 拉某用户某年某月的所有 note。
// year/month 为 0 表示不限（拉全部；用于调试或导出）。
func (s *DailyNoteStore) ListNotesByMonth(ctx context.Context, user string, year, month int) ([]*DailyNote, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if user == "" {
		return nil, nil
	}
	// 默认按 date 升序
	var rows *sql.Rows
	var err error
	if year > 0 && month > 0 {
		// date 是 'YYYY-MM-DD'，可以用 LIKE 'YYYY-MM-%'
		prefix := fmt.Sprintf("%04d-%02d-", year, month)
		rows, err = s.db.QueryContext(ctx,
			`SELECT user_account, date, content, zentao_effort_id, created_at, updated_at
			 FROM daily_notes
			 WHERE user_account = ? AND date LIKE ?
			 ORDER BY date ASC`,
			user, prefix+"%",
		)
	} else if year > 0 {
		prefix := fmt.Sprintf("%04d-", year)
		rows, err = s.db.QueryContext(ctx,
			`SELECT user_account, date, content, zentao_effort_id, created_at, updated_at
			 FROM daily_notes
			 WHERE user_account = ? AND date LIKE ?
			 ORDER BY date ASC`,
			user, prefix+"%",
		)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT user_account, date, content, zentao_effort_id, created_at, updated_at
			 FROM daily_notes
			 WHERE user_account = ?
			 ORDER BY date ASC`,
			user,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*DailyNote
	for rows.Next() {
		n := &DailyNote{}
		var createdAt, updatedAt int64
		var effortID int
		if err := rows.Scan(&n.User, &n.Date, &n.Content, &effortID, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		n.CreatedAt = time.Unix(createdAt, 0)
		n.UpdatedAt = time.Unix(updatedAt, 0)
		// zentao_effort_id 不暴露在 DailyNote VO（前端不需要），但扫描了保持列对齐
		out = append(out, n)
	}
	return out, rows.Err()
}

// SaveDailyNoteTask 存 (user → zentao task_id) 映射。
func (s *DailyNoteStore) SaveDailyNoteTask(ctx context.Context, user, taskName string, taskID int) error {
	if s == nil || s.db == nil {
		return errors.New("daily note store not initialized")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO daily_note_tasks (user_account, task_id, task_name, created_at)
		 VALUES (?, ?, ?, ?)`,
		user, taskID, taskName, time.Now().Unix(),
	)
	return err
}

// GetDailyNoteTaskID 取 (user → zentao task_id) 映射。task_id 0 表示未配置。
func (s *DailyNoteStore) GetDailyNoteTaskID(ctx context.Context, user string) (int, string, error) {
	if s == nil || s.db == nil {
		return 0, "", nil
	}
	var id int
	var name string
	err := s.db.QueryRowContext(ctx,
		`SELECT task_id, task_name FROM daily_note_tasks WHERE user_account = ?`,
		user,
	).Scan(&id, &name)
	if err == sql.ErrNoRows {
		return 0, "", nil
	}
	if err != nil {
		return 0, "", err
	}
	return id, name, nil
}

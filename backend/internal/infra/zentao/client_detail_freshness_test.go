package zentao

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// 详情读绕过 SessionCache 的回归测试。
//
// 起因：GetTaskByID 走 doSessionJSON（L1/L2 缓存，fresh TTL 2min，loader 失败时
// stale 兜底最长 1h）。外部通道（zt CLI 网页会话 / 禅道网页 UI）直写禅道后，
// MCP get_task_detail 在缓存窗口内仍报旧状态——用户取消任务 574165 成功后，
// 回查仍显示 status=wait，重启服务才一致。
//
// 钉死行为：task/bug/story 三类详情读必须回源（pool 限流仍在），上游数据变化
// 立即可见，不再受缓存窗口约束。

func TestClient_DetailReads_BypassCache(t *testing.T) {
	srv := newFakeZentaoServer(t)
	defer srv.Close()

	srv.setInner("/task-view-574165.json", `{"id":574165,"name":"e2e 任务","status":"wait"}`)
	srv.setInner("/bug-view-101.json", `{"bug":{"id":101,"title":"b","status":"active"}}`)
	srv.setInner("/story-view-301.json", `{"id":301,"title":"s","status":"changed"}`)

	c := NewSessionClient(srv.URL, "u", "p", "local", filepath.Join(t.TempDir(), "fresh.db"),
		&SessionWorkerPoolConfig{RatePerSec: 0})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = c.Close(ctx)
	})

	// 第一次读（回源并建立基线）
	task, err := c.GetTaskByID(context.Background(), 574165)
	if err != nil {
		t.Fatalf("GetTaskByID: %v", err)
	}
	if task.Status != "wait" {
		t.Fatalf("首次读应返回 wait, got %q", task.Status)
	}

	// 上游变化（模拟外部通道直写禅道，如 zt 网页会话取消任务）
	srv.setInner("/task-view-574165.json", `{"id":574165,"name":"e2e 任务","status":"cancel"}`)

	// 第二次读：即使远小于任何缓存 TTL，也必须看到新状态
	task2, err := c.GetTaskByID(context.Background(), 574165)
	if err != nil {
		t.Fatalf("GetTaskByID #2: %v", err)
	}
	if task2.Status != "cancel" {
		t.Errorf("详情读应绕过缓存回源最新状态, want cancel, got %q（回归：2min 缓存窗口内会读到旧值）", task2.Status)
	}

	// bug / story 详情同样必须回源
	if _, err := c.GetBugByID(context.Background(), 101); err != nil {
		t.Fatalf("GetBugByID: %v", err)
	}
	srv.setInner("/bug-view-101.json", `{"bug":{"id":101,"title":"b","status":"closed"}}`)
	bug2, err := c.GetBugByID(context.Background(), 101)
	if err != nil {
		t.Fatalf("GetBugByID #2: %v", err)
	}
	if bug2.Status != "closed" {
		t.Errorf("bug 详情读应回源最新状态, want closed, got %q", bug2.Status)
	}

	if _, err := c.GetStoryByID(context.Background(), 301); err != nil {
		t.Fatalf("GetStoryByID: %v", err)
	}
	srv.setInner("/story-view-301.json", `{"id":301,"title":"s","status":"closed"}`)
	story2, err := c.GetStoryByID(context.Background(), 301)
	if err != nil {
		t.Fatalf("GetStoryByID #2: %v", err)
	}
	if story2.Status != "closed" {
		t.Errorf("story 详情读应回源最新状态, want closed, got %q", story2.Status)
	}
}

// TestClient_DetailReads_Uncached_NoL2Entry 详情读不得向 SessionCache 写入任何
// L2 条目（否则下次即使改回缓存路径也会命中旧数据）。
func TestClient_DetailReads_Uncached_NoL2Entry(t *testing.T) {
	srv := newFakeZentaoServer(t)
	defer srv.Close()

	srv.setInner("/task-view-42.json", `{"id":42,"name":"t","status":"doing"}`)

	c := NewSessionClient(srv.URL, "u", "p", "local", filepath.Join(t.TempDir(), "nol2.db"),
		&SessionWorkerPoolConfig{RatePerSec: 0})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = c.Close(ctx)
	})

	if _, err := c.GetTaskByID(context.Background(), 42); err != nil {
		t.Fatalf("GetTaskByID: %v", err)
	}

	// L2 (sqlite) 不应有 tasks/task-view-42 条目
	var count int
	if err := c.cache.db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM session_cache WHERE entity = 'tasks' AND scope LIKE '%42%'`,
	).Scan(&count); err != nil {
		t.Fatalf("query session_cache: %v", err)
	}
	if count != 0 {
		t.Errorf("详情读不应写 L2 缓存, got %d rows for task-view-42", count)
	}
}

// ---- fake server 扩展：按 path 覆盖 GET inner JSON ----
//
// newFakeZentaoServer 的通用 GET 端点固定返回 bug-browse 形状的数据；详情读
// 测试需要按 path 注入可变的 inner JSON（模拟上游状态变化）。

func (s *fakeZentaoServer) setInner(path, inner string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inner == nil {
		s.inner = make(map[string]string)
	}
	s.inner[path] = inner
}

func (s *fakeZentaoServer) innerFor(path string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.inner[path]
	return v, ok
}

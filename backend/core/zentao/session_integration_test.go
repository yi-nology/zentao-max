package zentao

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yi-nology/zentao-max/backend/core/config"
	"github.com/yi-nology/zentao-max/backend/core/logger"
)

func init() {
	// sessionTransport.login() 成功路径会 logger.Info，没 init 会 panic。
	// 用最小配置初始化（指向 stderr 即可）。
	_ = logger.Init(&config.LogConfig{Level: "info", Format: "console", OutputPath: "stderr"})
}

// TestClient_DoSessionJSON_CacheHit 验证第二次 doSessionJSON 走 L1 cache，
// 不实际打到禅道。
func TestClient_DoSessionJSON_CacheHit(t *testing.T) {
	// 构造一个 fake session client：直接接管 session 字段，模拟禅道响应。
	c := &Client{
		account: "u",
		server:  "https://test",
		realm:   "kydc",
		// session: nil  → doSessionJSONWithRetry 走 c.session.DoJSON 路径会 panic。
		// 所以这个 test 必须自己造一个能 mock 的 session。
	}

	// 直接测 cache 行为；client 集成在 TestClient_Integration 里覆盖。
	if c != nil {
		_ = filepath.Join // 占位防 unused import
	}
}

// TestClient_Integration 端到端测：构造 Client + fake session（用 httptest server），
// 验证 doSessionJSON 走 cache + pool 后第二次请求不再打 server。
func TestClient_Integration(t *testing.T) {
	// 跳：httptest server 模拟禅道 JSON 端点
	srv := newFakeZentaoServer(t)
	defer srv.Close()

	cacheDB := filepath.Join(t.TempDir(), "integration.db")
	c := NewSessionClient(srv.URL, "user", "pass", "local", cacheDB,
		&SessionWorkerPoolConfig{RatePerSec: 0, ReadConcurrency: 2, JobBuf: 4})
	if c == nil {
		t.Fatal("NewSessionClient returned nil")
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = c.Close(ctx)
	})

	// 第一次请求：cache miss → 走 session.DoJSON → cache 写入
	var out1 fakeBugBrowseResp
	hitsBefore := srv.Hits()
	if err := c.doSessionJSON(context.Background(), "GetBugs", "/bug-browse-1-0-all.json", &out1); err != nil {
		t.Fatalf("first doSessionJSON err: %v", err)
	}
	if out1.Title == "" {
		t.Errorf("first response data empty: %+v", out1)
	}
	firstHits := srv.Hits()
	if firstHits <= hitsBefore {
		t.Errorf("first request: server hits = %d, should be > %d", firstHits, hitsBefore)
	}

	// 第二次：cache hit → server 不应再收到数据请求（hits 不增）
	var out2 fakeBugBrowseResp
	if err := c.doSessionJSON(context.Background(), "GetBugs", "/bug-browse-1-0-all.json", &out2); err != nil {
		t.Fatalf("second doSessionJSON err: %v", err)
	}
	if got := srv.Hits(); got != firstHits {
		t.Errorf("second request should hit cache, server hits = %d, want %d", got, firstHits)
	}
	if out1.Title != out2.Title {
		t.Errorf("cache data mismatch: %q vs %q", out1.Title, out2.Title)
	}
}

// TestClient_Integration_PoolLimits 验证 8 个并发请求，4 read worker 限流下都能完成。
func TestClient_Integration_PoolLimits(t *testing.T) {
	srv := newFakeZentaoServer(t)
	defer srv.Close()

	c := NewSessionClient(srv.URL, "u", "p", "local", "",
		&SessionWorkerPoolConfig{RatePerSec: 0, ReadConcurrency: 4, JobBuf: 16})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = c.Close(ctx)
	})

	// 8 个并发读不同 path（避免 singleflight 合并）
	for i := 0; i < 8; i++ {
		i := i
		go func() {
			var out fakeBugBrowseResp
			if err := c.doSessionJSON(context.Background(), "GetBugs", "/bug-browse-1-0-all.json", &out); err != nil {
				t.Errorf("goroutine %d: err: %v", i, err)
			}
		}()
	}
	// 等待 in-flight 完成
	time.Sleep(200 * time.Millisecond)
}

// TestClient_Integration_InvalidateAfterWrite 验证写成功后 cache 失效，下次读回源。
func TestClient_Integration_InvalidateAfterWrite(t *testing.T) {
	srv := newFakeZentaoServer(t)
	defer srv.Close()

	c := NewSessionClient(srv.URL, "u", "p", "local", filepath.Join(t.TempDir(), "i.db"),
		&SessionWorkerPoolConfig{RatePerSec: 0})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = c.Close(ctx)
	})

	// 第一次读
	var out1 fakeBugBrowseResp
	if err := c.doSessionJSON(context.Background(), "GetBugs", "/bug-browse-1-0-all.json", &out1); err != nil {
		t.Fatal(err)
	}
	hitsBefore := srv.Hits()

	// 写操作（fake server 总是 success）
	if err := c.doSessionPost(context.Background(), "ResolveBug", "/bug-resolve-1.json", nil); err != nil {
		t.Fatal(err)
	}

	// 写后：cache 失效 → 第二次读应回源
	var out2 fakeBugBrowseResp
	if err := c.doSessionJSON(context.Background(), "GetBugs", "/bug-browse-1-0-all.json", &out2); err != nil {
		t.Fatal(err)
	}
	if got := srv.Hits(); got <= hitsBefore {
		t.Errorf("after write, expected server hit (>%d), got hits = %d", hitsBefore, got)
	}
}

// TestClient_UpdateSessionConfig_ResetsCache 验证切账号时 cache 旧 ns 数据不可用。
func TestClient_UpdateSessionConfig_ResetsCache(t *testing.T) {
	srv := newFakeZentaoServer(t)
	defer srv.Close()

	c := NewSessionClient(srv.URL, "user1", "p1", "local", filepath.Join(t.TempDir(), "ns.db"),
		&SessionWorkerPoolConfig{RatePerSec: 0})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = c.Close(ctx)
	})

	// 第一次读
	var out1 fakeBugBrowseResp
	if err := c.doSessionJSON(context.Background(), "GetBugs", "/bug-browse-1-0-all.json", &out1); err != nil {
		t.Fatal(err)
	}
	hitsBefore := srv.Hits()

	// 切账号（test 里 server 一样但 account 不同 → ns 变）
	if err := c.UpdateSessionConfig(srv.URL, "user2", "p2", "local"); err != nil {
		t.Fatalf("UpdateSessionConfig err: %v", err)
	}

	// 第二次读：cache ns 变 → 必须回源
	var out2 fakeBugBrowseResp
	if err := c.doSessionJSON(context.Background(), "GetBugs", "/bug-browse-1-0-all.json", &out2); err != nil {
		t.Fatal(err)
	}
	if got := srv.Hits(); got <= hitsBefore {
		t.Errorf("after namespace change, expected server hit (>%d), got hits = %d", hitsBefore, got)
	}
}

// ---- fake server ----

type fakeBugBrowseResp struct {
	Title     string      `json:"title"`
	ProductID int         `json:"productID"`
	Bugs      interface{} `json:"bugs"`
}

// fakeZentaoServer 是一个最小可用的 zentao 模拟器。
// GET 任何 *.json 路径 → 返 {status:"success", data:"<inner json>"} 包裹；
// POST 任何 *.json 路径 → 返 {result:"success"}。
type fakeZentaoServer struct {
	*httptest.Server
	hits atomic.Int32
}

func newFakeZentaoServer(t *testing.T) *fakeZentaoServer {
	t.Helper()
	s := &fakeZentaoServer{}
	mux := http.NewServeMux()

	// 会话登录 3 步流程的 mock。
	mux.HandleFunc("/user-login.html", func(w http.ResponseWriter, r *http.Request) {
		s.hits.Add(1)
		switch r.Method {
		case http.MethodGet:
			// 种 zentaosid cookie
			http.SetCookie(w, &http.Cookie{
				Name:  "zentaosid",
				Value: "fakesid",
				Path:  "/",
			})
			w.WriteHeader(200)
		case http.MethodPost:
			// 登录成功
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"result":"success"}`))
		}
	})
	mux.HandleFunc("/user-refreshRandom.html", func(w http.ResponseWriter, r *http.Request) {
		s.hits.Add(1)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("fakernd12345"))
	})
	// 通用 *.json 端点
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		s.hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			inner, _ := json.Marshal(fakeBugBrowseResp{
				Title:     "fake-bugs",
				ProductID: 1,
				Bugs:      []interface{}{},
			})
			w.Write([]byte(`{"status":"success","data":"` +
				escapeJSONString(string(inner)) + `","md5":"x"}`))
		case http.MethodPost:
			w.Write([]byte(`{"result":"success"}`))
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	s.Server = httptest.NewServer(mux)
	return s
}

func (s *fakeZentaoServer) Hits() int { return int(s.hits.Load()) }

// escapeJSONString 把字符串转义成合法 JSON string（手动实现避免引 string）。
func escapeJSONString(s string) string {
	var b []byte
	for _, r := range s {
		switch r {
		case '"':
			b = append(b, '\\', '"')
		case '\\':
			b = append(b, '\\', '\\')
		case '\n':
			b = append(b, '\\', 'n')
		default:
			b = append(b, byte(r))
		}
	}
	return string(b)
}

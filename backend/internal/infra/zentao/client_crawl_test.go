package zentao

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// executionCrawlFakeServer 模拟 execution-all 翻页端点：
//   - 第 1..pageTotal 页各返回 pageSize 条（最后一页可为少）；
//   - 响应 pager 回显 recTotal/recPerPage/pageTotal；
//   - 记录每次请求的 path 便于断言页大小参数。
type executionCrawlFakeServer struct {
	*httptest.Server
	mu        sync.Mutex
	paths     []string
	recTotal  int
	pageTotal int
	perPage   int // 实际每页条数
	emptyPage int // >0 时该页起返回空（模拟上游提前收口）
}

var crawlPathRe = regexp.MustCompile(`-0--(\d+)-(\d+)-(\d+)\.json$`)

func newExecutionCrawlFakeServer(t *testing.T, recTotal, pageTotal, perPage, emptyPage int) *executionCrawlFakeServer {
	t.Helper()
	s := &executionCrawlFakeServer{recTotal: recTotal, pageTotal: pageTotal, perPage: perPage, emptyPage: emptyPage}
	mux := http.NewServeMux()
	// 会话登录三步
	mux.HandleFunc("/user-login.html", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			http.SetCookie(w, &http.Cookie{Name: "zentaosid", Value: "fakesid", Path: "/"})
			w.WriteHeader(200)
		case http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"result":"success"}`))
		}
	})
	mux.HandleFunc("/user-refreshRandom.html", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("fakernd12345"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		m := crawlPathRe.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		s.mu.Lock()
		s.paths = append(s.paths, r.URL.Path)
		s.mu.Unlock()

		page, _ := strconv.Atoi(m[3])
		items := []map[string]interface{}{}
		if !(s.emptyPage > 0 && page >= s.emptyPage) {
			start := (page - 1) * s.perPage
			n := s.perPage
			if start+n > s.recTotal {
				n = s.recTotal - start
			}
			if n < 0 {
				n = 0
			}
			for i := 0; i < n; i++ {
				id := start + i + 1
				items = append(items, map[string]interface{}{
					"id": id, "project": 100, "name": fmt.Sprintf("exec-%d", id), "status": "doing",
				})
			}
		}
		inner := map[string]interface{}{
			"executionStats": items,
			"pager": map[string]interface{}{
				"recTotal": s.recTotal, "recPerPage": s.perPage, "pageTotal": s.pageTotal, "pageID": page,
			},
		}
		b, _ := json.Marshal(inner)
		env, _ := json.Marshal(map[string]string{"status": "success", "data": string(b), "md5": "x"})
		w.Header().Set("Content-Type", "application/json")
		w.Write(env)
	})
	s.Server = httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return s
}

func (s *executionCrawlFakeServer) Paths() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.paths...)
}

// TestCrawlAllExecutions_CompletesWithBigPageSize 回归：2700+ 执行实例按
// 20 条/页需要 137 页，45s 预算只能翻 40~75 页导致产品维度数据系统性缺失。
// 现在按 100 条/页请求 + pager 驱动收口，小规模等价场景应完整拿全。
func TestCrawlAllExecutions_CompletesWithBigPageSize(t *testing.T) {
	srv := newExecutionCrawlFakeServer(t, 250, 3, 100, 0)
	c := NewSessionClient(srv.URL, "u", "p", "local", "", nil)

	crawl, err := c.GetAllExecutionsCrawlContext(context.Background())
	if err != nil {
		t.Fatalf("crawl: %v", err)
	}
	if len(crawl.Items) != 250 {
		t.Errorf("items = %d, want 250 (page size regression: 20/page would miss pages)", len(crawl.Items))
	}
	if crawl.RecTotal != 250 {
		t.Errorf("recTotal = %d, want 250", crawl.RecTotal)
	}
	if crawl.Incomplete {
		t.Error("complete crawl must not be marked Incomplete")
	}
	for _, p := range srv.Paths() {
		m := crawlPathRe.FindStringSubmatch(p)
		if m == nil {
			t.Fatalf("unexpected path %s", p)
		}
		if m[2] != "100" {
			t.Errorf("path %s requests recPerPage=%s, want 100", p, m[2])
		}
	}
}

// TestCrawlAllExecutions_MarksIncompleteWhenUpstreamStopsEarly 上游提前收口
// （recTotal 声明 250 但第 2 页为空）→ Incomplete=true，调用方据此禁止写缓存。
func TestCrawlAllExecutions_MarksIncompleteWhenUpstreamStopsEarly(t *testing.T) {
	srv := newExecutionCrawlFakeServer(t, 250, 3, 100, 2)
	c := NewSessionClient(srv.URL, "u", "p", "local", "", nil)

	crawl, err := c.GetAllExecutionsCrawlContext(context.Background())
	if err != nil {
		t.Fatalf("crawl: %v", err)
	}
	if len(crawl.Items) != 100 {
		t.Errorf("items = %d, want 100 (only first page)", len(crawl.Items))
	}
	if !crawl.Incomplete {
		t.Error("early-stop crawl must be marked Incomplete (recTotal 250 > collected 100)")
	}
}

// detailFakeServer 模拟详情端点：可返回正常 bug（用户引用为字符串账号），
// 或「成功信封 + 内层 result=fail + 拼接多余 JSON」的不存在实体形态。
type detailFakeServer struct {
	*httptest.Server
	body string
}

func newDetailFakeServer(t *testing.T, body string) *detailFakeServer {
	t.Helper()
	s := &detailFakeServer{body: body}
	mux := http.NewServeMux()
	mux.HandleFunc("/user-login.html", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			http.SetCookie(w, &http.Cookie{Name: "zentaosid", Value: "fakesid", Path: "/"})
			w.WriteHeader(200)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"result":"success"}`))
	})
	mux.HandleFunc("/user-refreshRandom.html", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("fakernd12345"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(s.body))
	})
	s.Server = httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return s
}

// TestGetBugByID_StringUserRef 回归：bug-view 的 openedBy/assignedTo 是字符串
// 账号，详情必须能解析（此前 get_bug_detail 对所有此类 bug 返回 400）。
func TestGetBugByID_StringUserRef(t *testing.T) {
	inner := `{"bug":{"id":437041,"title":"采集器上报","status":"active","openedBy":"lixiaoying","assignedTo":"zhangyi01"}}`
	env, _ := json.Marshal(map[string]string{"status": "success", "data": inner, "md5": "x"})
	srv := newDetailFakeServer(t, string(env))
	c := NewSessionClient(srv.URL, "u", "p", "local", "", nil)

	bug, err := c.GetBugByID(context.Background(), 437041)
	if err != nil {
		t.Fatalf("GetBugByID: %v", err)
	}
	if bug.ID != 437041 || bug.OpenedBy.Account != "lixiaoying" {
		t.Errorf("unexpected bug: %+v", bug)
	}
}

// TestGetBugByID_NonexistentReadableError 回归：不存在实体返回「成功信封 +
// 内层 result=fail + 额外拼接 JSON」时，必须给出可读错误而不是
// "解析禅道响应外层 JSON 失败 … invalid character '{' after top-level value"。
func TestGetBugByID_NonexistentReadableError(t *testing.T) {
	body := `{"status":"success","data":"{\"result\":\"fail\",\"message\":\"抱歉，您没有权限访问该对象\"}"}{"trailing":"garbage"}`
	srv := newDetailFakeServer(t, body)
	c := NewSessionClient(srv.URL, "u", "p", "local", "", nil)

	_, err := c.GetBugByID(context.Background(), 999999999)
	if err == nil {
		t.Fatal("expected error for nonexistent bug")
	}
	msg := err.Error()
	if strings.Contains(msg, "解析禅道响应外层 JSON 失败") || strings.Contains(msg, "after top-level value") {
		t.Errorf("error should be readable, got internal parse error: %s", msg)
	}
	if !strings.Contains(msg, "没有权限") {
		t.Errorf("error should carry upstream message, got: %s", msg)
	}
}

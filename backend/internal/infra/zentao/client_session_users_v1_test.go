package zentao

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	zentaosdk "github.com/yi-nology/zentao-max/backend/internal/infra/zentao/sdk"
)

// usersV1TestServer 是用户列表链路的最小禅道模拟器：
//   - /user-browse.json 可配置返回（envelope 包裹的用户 map / 0 字节）
//   - /api.php/v1/users 可配置返回（v1 单层 JSON 分页）
//   - /user-profile.json 返回当前用户
type usersV1TestServer struct {
	*httptest.Server
	browseBody  string // /user-browse.json 响应体（原样返回）
	v1Pages     [][]v1User
	v1Total     int
	profileJSON string
	v1Hits      int
	browseHits  int
}

func newUsersV1TestServer(t *testing.T) *usersV1TestServer {
	t.Helper()
	s := &usersV1TestServer{}
	mux := http.NewServeMux()

	// 登录三步
	mux.HandleFunc("/user-login.html", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			http.SetCookie(w, &http.Cookie{Name: "zentaosid", Value: "fakesid", Path: "/"})
			w.WriteHeader(200)
		case http.MethodPost:
			// local 域校验 md5(md5(pw)+rand)，这里只验格式不验值
			if err := r.ParseForm(); err != nil || r.PostForm.Get("account") == "" {
				w.Write([]byte(`{"result":"fail","message":"bad form"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"result":"success"}`))
		}
	})
	mux.HandleFunc("/user-refreshRandom.html", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("fakernd12345"))
	})

	mux.HandleFunc("/user-browse.json", func(w http.ResponseWriter, r *http.Request) {
		s.browseHits++
		w.WriteHeader(200)
		w.Write([]byte(s.browseBody))
	})
	mux.HandleFunc("/user-profile.json", func(w http.ResponseWriter, r *http.Request) {
		inner, _ := json.Marshal(map[string]interface{}{
			"title": "USER #5855 me/档案",
			"user": map[string]interface{}{
				"id": 5855, "account": "zhangyi01", "realname": "测试员", "dept": 8341,
			},
		})
		w.Write([]byte(`{"status":"success","data":"` + escapeJSONString(string(inner)) + `","md5":"x"}`))
	})
	mux.HandleFunc("/api.php/v1/users", func(w http.ResponseWriter, r *http.Request) {
		s.v1Hits++
		page := 1
		fmt.Sscanf(r.URL.Query().Get("page"), "%d", &page)
		users := []v1User{}
		if page >= 1 && page <= len(s.v1Pages) {
			users = s.v1Pages[page-1]
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"page": page, "total": s.v1Total, "limit": v1UsersPageSize, "users": users,
		})
	})

	s.Server = httptest.NewServer(mux)
	return s
}

func newUsersV1Client(t *testing.T, srv *usersV1TestServer) *Client {
	t.Helper()
	cacheDB := filepath.Join(t.TempDir(), "users-v1.db")
	c := NewSessionClient(srv.URL, "tester", "pw", "local", cacheDB, nil)
	if c == nil {
		t.Fatal("NewSessionClient returned nil")
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = c.Close(ctx)
	})
	return c
}

// 场景 1（主目标）：受限实例 /user-browse.json 返 200 + 0 字节（账号无用户管理权限），
// 但 /api.php/v1/users 可用 —— getUsersAllSession 应回退到 v1 拿到全量用户，
// 而不是只返回当前用户。pm.kylin.com 2026-10-30 实测行为。
func TestGetUsersAllSession_FallsBackToV1(t *testing.T) {
	srv := newUsersV1TestServer(t)
	srv.browseBody = "" // 受限实例：0 字节
	srv.v1Pages = [][]v1User{
		{{ID: 7223, Account: "zhiyijie", Realname: "支亦杰", Dept: 8637, Email: "zhiyijie@kylinos.cn"},
			{ID: 1, Account: "admin", Realname: "admin"}},
	}
	srv.v1Total = 2
	defer srv.Close()
	c := newUsersV1Client(t, srv)

	users, err := c.getUsersAllSession(context.Background())
	if err != nil {
		t.Fatalf("getUsersAllSession err: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("want 2 users from v1 fallback, got %d: %+v", len(users), users)
	}
	if users[0].Account != "zhiyijie" || users[1].Account != "admin" {
		t.Errorf("accounts mismatch: %+v", users)
	}
	if users[0].Email != "zhiyijie@kylinos.cn" {
		t.Errorf("email field lost: %+v", users[0])
	}
	if srv.v1Hits == 0 {
		t.Errorf("v1 endpoint should have been consulted")
	}
}

// 场景 2：/user-browse.json 正常（权限完整实例）时保持原行为，优先使用其
// 字段更全的数据，且不应请求 v1 端点。
func TestGetUsersAllSession_UserBrowseStillFirst(t *testing.T) {
	srv := newUsersV1TestServer(t)
	browse := map[string]interface{}{
		"title": "用户列表",
		"users": map[string]interface{}{
			"u1": map[string]interface{}{"id": 11, "account": "alpha", "realname": "甲", "gender": "m", "phone": "13800000001"},
			"u2": map[string]interface{}{"id": 12, "account": "beta", "realname": "乙", "gender": "f", "phone": "13800000002"},
		},
	}
	raw, _ := json.Marshal(browse)
	// 用 json.Marshal 生成 envelope：内嵌 escapeJSONString 不处理多字节中文，会截断
	env, _ := json.Marshal(map[string]string{"status": "success", "data": string(raw), "md5": "x"})
	srv.browseBody = string(env)
	srv.v1Pages = [][]v1User{{{ID: 99, Account: "shouldnot", Realname: "不应出现"}}}
	srv.v1Total = 1
	defer srv.Close()
	c := newUsersV1Client(t, srv)

	users, err := c.getUsersAllSession(context.Background())
	if err != nil {
		t.Fatalf("getUsersAllSession err: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("want 2 users from user-browse, got %d: %+v", len(users), users)
	}
	if users[0].Phone == "" {
		t.Errorf("user-browse rich fields (phone) should be preserved: %+v", users[0])
	}
	if srv.v1Hits != 0 {
		t.Errorf("v1 endpoint must not be hit when user-browse works, got %d hits", srv.v1Hits)
	}
}

// 场景 3：v1 total 超过单页 limit 时应翻页拉全。
func TestGetUsersAllSession_V1Paginates(t *testing.T) {
	oldLimit := v1UsersPageSize
	v1UsersPageSize = 2
	defer func() { v1UsersPageSize = oldLimit }()

	srv := newUsersV1TestServer(t)
	srv.browseBody = ""
	srv.v1Pages = [][]v1User{
		{{ID: 1, Account: "a", Realname: "甲"}, {ID: 2, Account: "b", Realname: "乙"}},
		{{ID: 3, Account: "c", Realname: "丙"}},
	}
	srv.v1Total = 3
	defer srv.Close()
	c := newUsersV1Client(t, srv)

	users, err := c.getUsersAllSession(context.Background())
	if err != nil {
		t.Fatalf("getUsersAllSession err: %v", err)
	}
	if len(users) != 3 {
		t.Fatalf("want 3 users across 2 pages, got %d: %+v", len(users), users)
	}
	if srv.v1Hits != 2 {
		t.Errorf("want exactly 2 v1 page fetches, got %d", srv.v1Hits)
	}
}

// 场景 4：v1 也拿不到（老实例无该端点）时，保留原有的"仅当前用户"兜底。
func TestGetUsersAllSession_LastResortCurrentUser(t *testing.T) {
	srv := newUsersV1TestServer(t)
	srv.browseBody = ""
	srv.v1Pages = nil // v1 返回空 users
	srv.v1Total = 0
	defer srv.Close()
	c := newUsersV1Client(t, srv)

	users, err := c.getUsersAllSession(context.Background())
	if err != nil {
		t.Fatalf("getUsersAllSession err: %v", err)
	}
	if len(users) != 1 || users[0].Account != "zhangyi01" {
		t.Fatalf("want current-user fallback, got %+v", users)
	}
}

var _ = zentaosdk.User{} // 保持 import 稳定

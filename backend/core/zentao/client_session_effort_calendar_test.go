package zentao

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// testZenSIDName 是测试里种 session cookie 用的固定 name（避 linter goconst）。
const testZenSIDName = "zentaosid"

// TestParseCalendarObjectType 验证 [T]/[B]/[S]/[P]/空 五种 title 前缀解析。
func TestParseCalendarObjectType(t *testing.T) {
	cases := []struct {
		title        string
		wantType     string
		wantClean    string
	}{
		{"[T]完善全景图文档", "task", "完善全景图文档"},
		{"[B]修复样式错位", "bug", "修复样式错位"},
		{"[S]需求评审", "story", "需求评审"},
		{"[P]项目维护", "project", "项目维护"},
		{"没有前缀的工时", "", "没有前缀的工时"},
		{"[]空前缀", "", "[]空前缀"},
		{"[X]未知前缀", "", "[X]未知前缀"}, // 未知前缀应保持原 title
		{"[TASK]多字符前缀", "", "[TASK]多字符前缀"},
	}
	for _, tc := range cases {
		t.Run(tc.title, func(t *testing.T) {
			gotType, gotClean := parseCalendarObjectType(tc.title)
			if gotType != tc.wantType {
				t.Errorf("type: got %q want %q", gotType, tc.wantType)
			}
			if gotClean != tc.wantClean {
				t.Errorf("clean: got %q want %q", gotClean, tc.wantClean)
			}
		})
	}
}

// TestParseCalendarEntries_Empty 验证空响应不报错。
func TestParseCalendarEntries_Empty(t *testing.T) {
	got, err := parseCalendarEntries(nil)
	if err != nil {
		t.Fatalf("empty: %v", err)
	}
	if got != nil {
		t.Errorf("empty: got %v want nil", got)
	}
	got, err = parseCalendarEntries([]byte(""))
	if err != nil {
		t.Fatalf("empty bytes: %v", err)
	}
	if got != nil {
		t.Errorf("empty bytes: got %v want nil", got)
	}
}

// TestParseCalendarEntries_BadJSON 验证脏数据不会 panic，能返清晰错误。
func TestParseCalendarEntries_BadJSON(t *testing.T) {
	_, err := parseCalendarEntries([]byte("{not valid json"))
	if err == nil {
		t.Fatal("want error on bad json, got nil")
	}
}

// TestParseCalendarEntries_Sample 验证真实样本（testdata/sample_effort_calendar.json）
// 能完整解析，所有字段都被还原，且 ObjectType 被正确抽取。
func TestParseCalendarEntries_Sample(t *testing.T) {
	raw, err := os.ReadFile("testdata/sample_effort_calendar.json")
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}

	entries, err := parseCalendarEntries(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// 样本是 316 条（一个用户整年 effort），不能丢
	if len(entries) < 100 {
		t.Errorf("entries too few: got %d, want >= 100", len(entries))
	}

	// 统计 ObjectType 分布，确保 [T] 和 [B] 都被识别
	var taskCount, bugCount, unknownCount int
	for _, e := range entries {
		switch e.ObjectType {
		case "task":
			taskCount++
		case "bug":
			bugCount++
		default:
			unknownCount++
		}
		// 所有 entry 必须有日期
		if e.Start == "" {
			t.Errorf("entry %d missing start", e.ID)
		}
		// Consumed 允许 0（占位 effort：登记了但没填工时，禅道支持这种用法）。
		// 只允许 <= 0 的告警：负数是脏数据，但 0 是合法占位。
		if e.Consumed < 0 {
			t.Errorf("entry %d consumed negative: %f", e.ID, e.Consumed)
		}
		// title 不能带 [T] / [B] 前缀（应该被剥掉）
		if len(e.Title) > 0 && e.Title[0] == '[' {
			t.Errorf("entry %d title not stripped: %q", e.ID, e.Title)
		}
	}
	if taskCount == 0 {
		t.Error("no task entries parsed")
	}
	if bugCount == 0 {
		t.Error("no bug entries parsed")
	}
	if unknownCount > 0 {
		t.Logf("warning: %d entries with unknown object type", unknownCount)
	}
	t.Logf("parsed: task=%d bug=%d unknown=%d total=%d",
		taskCount, bugCount, unknownCount, len(entries))
}

// TestCalendarEntry_JSONRoundTrip 验证 JSON 序列化字段名稳定（前端依赖 snake_case）。
func TestCalendarEntry_JSONRoundTrip(t *testing.T) {
	in := CalendarEntry{
		ID:         10086,
		Title:      "工时标题",
		ObjectType: "task",
		Start:      "2026-07-15",
		End:        "2026-07-15",
		Consumed:   4.5,
		URL:        "/effort-view-10086.html",
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// 字段名必须稳定（前端 types/api.ts 也用同样名字）
	for _, k := range []string{`"id":`, `"title":`, `"objectType":`, `"start":`, `"end":`, `"consumed":`, `"url":`} {
		if !contains(b, k) {
			t.Errorf("missing field %s in JSON: %s", k, b)
		}
	}
	var out CalendarEntry
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out != in {
		t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", out, in)
	}
}

func contains(haystack []byte, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// TestGetEffortCalendar_AuthError_NotCached 回归 V040+ 教训：
// 会话失效 / 权限拒绝时 GetEffortCalendar 的 loader 必须 wrap errDoNotServeStale，
// 让 SessionCache 跳过 L2 stale fallback 直接把 error 透传。
// 否则空数据会被当成功结果写进 L1/L2 cache，登录恢复后前端仍看到"空日历"。
func TestGetEffortCalendar_AuthError_NotCached(t *testing.T) {
	// 用一个会让 GetRawJSON 触发 errSessionReloginRequired 的 server：
	// 所有响应体以 '<' 开头 → GetRawJSON 启发式判定为 user-login redirect。
	mux := http.NewServeMux()
	mux.HandleFunc("/user-login.html", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			http.SetCookie(w, &http.Cookie{Name: testZenSIDName, Value: "s", Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
			w.WriteHeader(200)
		case http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":"success"}`))
		}
	})
	mux.HandleFunc("/user-refreshRandom.html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("r"))
	})
	// 关键：GetRawJSON 拉到的 body 以 '<' 开头 → 触发 errSessionReloginRequired。
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><body>session expired</body></html>`))
	})
	authSrv := httptest.NewServer(mux)
	defer authSrv.Close()

	cacheDB := filepath.Join(t.TempDir(), "effort-auth.db")
	c := NewSessionClient(authSrv.URL, "u", "p", "kydc", cacheDB, nil)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = c.Close(ctx)
	})

	// 调 GetEffortCalendar → 触发 loader → GetRawJSON 返 errSessionReloginRequired
	// → wrap errDoNotServeStale → GetOrLoad 直接把 err 透出（不进 cache）。
	_, err := c.GetEffortCalendar(context.Background(), 1, 2026)
	if err == nil {
		t.Fatal("expected error on auth failure, got nil")
	}
	if !errors.Is(err, errDoNotServeStale) {
		t.Errorf("err should wrap errDoNotServeStale, got: %v", err)
	}

	// 验证 cache 没被填：直接查 L1 应该有 0 条（use NewSessionCache 直接造一个看 Stats）。
	if c.cache == nil {
		t.Fatal("cache should be initialized")
	}
	stats := c.cache.Stats()
	if stats.L1Size != 0 {
		t.Errorf("auth error should NOT populate cache, L1Size=%d", stats.L1Size)
	}
}

// TestGetAllBugs_BrowseTypeAll 验证 GetAllBugs 走 browseType=all（禅道语义：含 closed）。
// 配合删掉的 GetAllBugsIncludeClosed，确保两个用例合并后行为不变。
func TestGetAllBugs_BrowseTypeAll(t *testing.T) {
	var got atomic.Value // string
	mux := http.NewServeMux()
	mux.HandleFunc("/user-login.html", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			http.SetCookie(w, &http.Cookie{Name: testZenSIDName, Value: "s", Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
			w.WriteHeader(200)
		case http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":"success"}`))
		}
	})
	mux.HandleFunc("/user-refreshRandom.html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("r"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".json") {
			got.Store(r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		inner, _ := json.Marshal(map[string]interface{}{
			"title": "fake", "productID": 1, "bugs": []interface{}{}, "pager": map[string]int{"recTotal": 0, "pageTotal": 1},
		})
		_, _ = w.Write([]byte(`{"status":"success","data":"` + escapeJSONString(string(inner)) + `","md5":"x"}`))
	})
	pathSrv := httptest.NewServer(mux)
	defer pathSrv.Close()

	cacheDB := filepath.Join(t.TempDir(), "bug-path.db")
	c := NewSessionClient(pathSrv.URL, "u", "p", "kydc", cacheDB, nil)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = c.Close(ctx)
	})

	if _, err := c.GetAllBugs(1); err != nil {
		t.Fatalf("GetAllBugs err: %v", err)
	}

	p, _ := got.Load().(string)
	if !strings.Contains(p, "-0-all-") {
		t.Errorf("GetAllBugs should use browseType=all (含 closed 语义), got path=%q", p)
	}
}

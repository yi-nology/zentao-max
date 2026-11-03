package update

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/yi-nology/zentao-max/backend/internal/config"
	"github.com/yi-nology/zentao-max/backend/pkg/logger"
)

func TestMain(m *testing.M) {
	// 静默降级路径会打内部日志；生产由 bootstrap 初始化，测试需自备
	_ = logger.Init(&config.LogConfig{Level: "warn", Format: "console"})
	os.Exit(m.Run())
}

func TestIsNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v0.5.0", "v0.4.1", true},
		{"0.5.0", "0.4.1", true},
		{"v0.4.1", "v0.4.1", false},
		{"v0.4.0", "v0.4.1", false},
		{"v1.0.0", "v0.9.9", true},
		{"0.10.0", "0.9.9", true}, // 数值比较而非字典序
		{"dev", "dev", false},
		{"v0.4.1", "dev", true}, // dev 构建落后于 release
		{"v0.4.1-beta", "v0.4.0", true},
	}
	for _, c := range cases {
		if got := isNewer(c.latest, c.current); got != c.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

func TestNormalizeVer(t *testing.T) {
	if v := normalizeVer("v0.4.1"); v != [3]int{0, 4, 1} {
		t.Errorf("normalizeVer(v0.4.1) = %v", v)
	}
	if v := normalizeVer("1.2.3-beta"); v != [3]int{1, 2, 3} {
		t.Errorf("normalizeVer(1.2.3-beta) = %v", v)
	}
}

// ─── 版本发布管理服务（release-service 源）───

// releaseTestServer 起一个可变响应体的模拟发布服务，返回服务地址与请求捕获指针。
func releaseTestServer(t *testing.T, payload *string, status *int) (url string, seen *map[string]string) {
	t.Helper()
	seen = &map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m := *seen
		m["path"] = r.URL.Path
		m["query"] = r.URL.RawQuery
		m["ua"] = r.Header.Get("User-Agent")
		m["clientVer"] = r.Header.Get("X-Client-Version")
		code := http.StatusOK
		if status != nil {
			code = *status
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_, _ = io.WriteString(w, *payload)
	}))
	t.Cleanup(srv.Close)
	return srv.URL, seen
}

func newReleaseSvc(url, slug, current string) *Service {
	return New(config.UpdateConfig{Enabled: true, ReleaseURL: url, Slug: slug}, current)
}

func TestQueryReleaseService(t *testing.T) {
	payload := `{
		"has_update": true, "force_update": false, "latest": "1.0.0",
		"latest_status": "released", "note_md": "## 1.0.0 更新\n- 首个版本",
		"released_at": "2026-10-30T15:00:00Z", "min_supported": "0.9.0",
		"channels": [{"key":"github","name":"GitHub Releases",
			"url":"https://example.com/releases/v1.0.0","status":"published"}]
	}`
	base, seen := releaseTestServer(t, &payload, nil)

	res, err := newReleaseSvc(base, "zentao-max", "0.5.0").queryReleaseService()
	if err != nil {
		t.Fatalf("queryReleaseService: %v", err)
	}
	m := *seen
	if m["path"] != "/products/zentao-max/upgrade-check" {
		t.Errorf("path = %q", m["path"])
	}
	if m["query"] != "current=0.5.0&locale=zh-CN" {
		t.Errorf("query = %q", m["query"])
	}
	if m["ua"] != "zentao-max/0.5.0" || m["clientVer"] != "0.5.0" {
		t.Errorf("ua = %q, clientVer = %q", m["ua"], m["clientVer"])
	}
	if res.latest != "1.0.0" || res.source != "release" {
		t.Errorf("latest = %q, source = %q", res.latest, res.source)
	}
	if res.hasUpdate == nil || !*res.hasUpdate {
		t.Errorf("hasUpdate 应采信服务端 true，got %v", res.hasUpdate)
	}
	if res.forceUpdate || res.minSupported != "0.9.0" || res.releasedAt == "" {
		t.Errorf("force = %v, minSupported = %q, releasedAt = %q", res.forceUpdate, res.minSupported, res.releasedAt)
	}
	if !strings.Contains(res.notes, "首个版本") {
		t.Errorf("notes = %q", res.notes)
	}
	if res.channelURL != "https://example.com/releases/v1.0.0" {
		t.Errorf("channelURL = %q", res.channelURL)
	}
	if res.downloadURL != "" {
		// 非 GitHub 渠道不解析直链，前端退化为跳转发布页
		t.Errorf("downloadURL 应为空，got %q", res.downloadURL)
	}
}

func TestQueryReleaseServiceChannelPriority(t *testing.T) {
	// 未发布渠道在前：应选 status=published 的渠道
	payload := `{
		"has_update": true, "latest": "1.0.0",
		"channels": [
			{"key":"inner","url":"https://example.com/inner","status":"draft"},
			{"key":"github","url":"https://example.com/pub","status":"published"}
		]
	}`
	base, _ := releaseTestServer(t, &payload, nil)

	res, err := newReleaseSvc(base, "zentao-max", "0.5.0").queryReleaseService()
	if err != nil {
		t.Fatalf("queryReleaseService: %v", err)
	}
	if res.channelURL != "https://example.com/pub" {
		t.Errorf("channelURL 应取 published 渠道，got %q", res.channelURL)
	}
}

func TestQueryReleaseServiceServerAuthoritative(t *testing.T) {
	// 服务端判定无更新（即使本地 isNewer 会判 true）——客户端必须采信
	payload := `{"has_update": false, "latest": "0.0.9"}`
	base, _ := releaseTestServer(t, &payload, nil)

	info := newReleaseSvc(base, "zentao-max", "0.1.0").doCheck()
	if info.HasUpdate {
		t.Errorf("服务端 has_update=false 应直接采信，got true")
	}
	if info.Source != "release" || info.Latest != "0.0.9" {
		t.Errorf("source = %q, latest = %q", info.Source, info.Latest)
	}
}

func TestQueryReleaseServiceForce(t *testing.T) {
	payload := `{"has_update": true, "force_update": true, "latest": "1.0.0", "min_supported": "0.9.0"}`
	base, _ := releaseTestServer(t, &payload, nil)

	info := newReleaseSvc(base, "zentao-max", "0.1.0").doCheck()
	if !info.ForceUpdate || info.MinSupported != "0.9.0" || !info.HasUpdate {
		t.Errorf("force = %v, minSupported = %q, hasUpdate = %v", info.ForceUpdate, info.MinSupported, info.HasUpdate)
	}
}

func TestQueryReleaseServiceErrors(t *testing.T) {
	t.Run("缺少latest", func(t *testing.T) {
		payload := `{"has_update": true}`
		base, _ := releaseTestServer(t, &payload, nil)
		_, err := newReleaseSvc(base, "zentao-max", "0.5.0").queryReleaseService()
		if err == nil || !strings.Contains(err.Error(), "latest") || !errors.Is(err, errSilentUpdate) {
			t.Errorf("want 带静默标记的 latest 缺失错误, got %v", err)
		}
	})
	t.Run("HTTP404", func(t *testing.T) {
		payload := `{"code":40404,"message":"资源不存在"}`
		code := http.StatusNotFound
		base, _ := releaseTestServer(t, &payload, &code)
		_, err := newReleaseSvc(base, "no-such", "0.5.0").queryReleaseService()
		if err == nil || !strings.Contains(err.Error(), "404") || !errors.Is(err, errSilentUpdate) {
			t.Errorf("want 带静默标记的 HTTP 404 错误, got %v", err)
		}
	})
}

func TestDoCheckStaleOnFailure(t *testing.T) {
	payload := `{"has_update": true, "latest": "1.0.0", "note_md": "## 更新", "force_update": true}`
	code := http.StatusOK
	base, _ := releaseTestServer(t, &payload, &code)
	svc := newReleaseSvc(base, "zentao-max", "0.5.0")

	if info := svc.CheckNow(); info.Latest != "1.0.0" || !info.ForceUpdate {
		t.Fatalf("首次检查失败: %+v", info)
	}
	// 发布服务非 200：静默降级——不对外暴露错误（LastError 为空），
	// 保留上次成功快照（含 force/notes），仅刷新检查时间
	code = http.StatusInternalServerError
	info := svc.CheckNow()
	if info.LastError != "" {
		t.Errorf("非 200 不得对外暴露错误, lastError = %q", info.LastError)
	}
	if info.Latest != "1.0.0" || !info.ForceUpdate || !info.HasUpdate || info.Source != "release" {
		t.Errorf("故障时快照应保留: %+v", info)
	}
	// 恢复后正常刷新
	code = http.StatusOK
	if info := svc.CheckNow(); info.LastError != "" || !info.HasUpdate {
		t.Errorf("恢复后应正常检查: %+v", info)
	}
}

func TestResolveGitHubAsset(t *testing.T) {
	platform := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, fmt.Sprintf(
			`{"assets":[{"name":"sha256sums.txt","browser_download_url":"https://github.com/acme/zentao-max/releases/download/v1.2.0/sha256sums.txt"},
			{"name":"zentao-max-1.2.0-%s.zip","browser_download_url":"https://github.com/acme/zentao-max/releases/download/v1.2.0/zentao-max-1.2.0-%s.zip"}]}`,
			platform, platform))
	}))
	defer srv.Close()
	oldBase := githubAPIBase
	githubAPIBase = srv.URL
	t.Cleanup(func() { githubAPIBase = oldBase })

	direct := "https://github.com/acme/zentao-max/releases/download/v1.2.0/zentao-max-1.2.0-" + platform + ".zip"
	cases := []struct {
		name, page, wantPath string
	}{
		{"tag", "https://github.com/acme/zentao-max/releases/tag/v1.2.0", "/repos/acme/zentao-max/releases/tags/v1.2.0"},
		{"latest", "https://github.com/acme/zentao-max/releases/latest", "/repos/acme/zentao-max/releases/latest"},
		{"bare", "https://github.com/acme/zentao-max/releases", "/repos/acme/zentao-max/releases/latest"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := New(config.UpdateConfig{}, "0.5.0").resolveGitHubAsset(c.page)
			if got != direct {
				t.Errorf("resolveGitHubAsset(%q) = %q, want %q", c.page, got, direct)
			}
			if gotPath != c.wantPath {
				t.Errorf("api path = %q, want %q", gotPath, c.wantPath)
			}
		})
	}

	t.Run("非GitHub链接", func(t *testing.T) {
		if got := New(config.UpdateConfig{}, "0.5.0").resolveGitHubAsset("https://example.com/x/releases/tag/v1"); got != "" {
			t.Errorf("want 空, got %q", got)
		}
	})
	t.Run("无匹配平台资产", func(t *testing.T) {
		srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"assets":[{"name":"zentao-max-1.2.0-PLAN9-POWER9.zip","browser_download_url":"https://github.com/a/b/d/p.zip"}]}`)
		}))
		defer srv2.Close()
		githubAPIBase = srv2.URL
		if got := New(config.UpdateConfig{}, "0.5.0").resolveGitHubAsset("https://github.com/a/b/releases/tag/v1.2.0"); got != "" {
			t.Errorf("无匹配资产应返回空, got %q", got)
		}
	})
}

func TestIsReleasePageURL(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
	}{
		{"https://github.com/o/r/releases/tag/v1", true},
		{"https://github.com/o/r/releases/latest", true},
		{"https://github.com/o/r/releases", true},
		{"https://github.com/o/r/releases/download/v1/zentao-max-1.0.0-linux-amd64.tar.gz", false},
		{"https://example.com/o/r/releases/tag/v1", false},
		{"not a url", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isReleasePageURL(c.raw); got != c.want {
			t.Errorf("isReleasePageURL(%q) = %v, want %v", c.raw, got, c.want)
		}
	}
}

func TestPickPlatformAsset(t *testing.T) {
	platform := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	assets := []ghAsset{
		{Name: "sha256sums.txt", URL: "https://x/s.txt"},
		{Name: "zentao-max-1.0.0-" + platform + ".zip", URL: "https://x/app.zip"},
	}
	if got := pickPlatformAsset(assets); got != "https://x/app.zip" {
		t.Errorf("pickPlatformAsset = %q", got)
	}
	if got := pickPlatformAsset(assets[:1]); got != "" {
		t.Errorf("无匹配应返回空, got %q", got)
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("短文本", 10); got != "短文本" {
		t.Errorf("未超长不应截断, got %q", got)
	}
	got := truncateRunes(strings.Repeat("更", 4001), 4000)
	if got != strings.Repeat("更", 4000)+"…" {
		t.Errorf("超长应按 rune 截断加省略号，len=%d", len([]rune(got)))
	}
}

// ─── 更新日志（changelog 代理）───

func TestChangelog(t *testing.T) {
	payload := `{"slug":"demo-app","items":[
		{"version":"1.0.0","released_at":"2026-10-30T15:00:00Z","note_md":"## 1.0.0 更新\n- 首个版本"},
		{"version":"0.9.0","released_at":"2026-10-29T10:00:00Z","note_md":""}]}`
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		_, _ = io.WriteString(w, payload)
	}))
	defer srv.Close()

	t.Run("映射与默认limit", func(t *testing.T) {
		svc := New(config.UpdateConfig{ReleaseURL: srv.URL, Slug: "zentao-max"}, "0.5.0")
		items, err := svc.Changelog(0)
		if err != nil {
			t.Fatalf("Changelog: %v", err)
		}
		if gotPath != "/products/zentao-max/changelog" {
			t.Errorf("path = %q", gotPath)
		}
		if gotQuery != "limit=20&locale=zh-CN" {
			t.Errorf("query = %q", gotQuery)
		}
		if len(items) != 2 || items[0].Version != "1.0.0" || items[0].ReleasedAt == "" {
			t.Fatalf("items = %+v", items)
		}
		if !strings.Contains(items[0].NoteMD, "首个版本") || items[1].NoteMD != "" {
			t.Errorf("noteMd 映射错误: %+v", items)
		}
	})

	t.Run("limit上限50", func(t *testing.T) {
		svc := New(config.UpdateConfig{ReleaseURL: srv.URL, Slug: "zentao-max"}, "0.5.0")
		if _, err := svc.Changelog(100); err != nil {
			t.Fatalf("Changelog: %v", err)
		}
		if gotQuery != "limit=50&locale=zh-CN" {
			t.Errorf("query = %q, want limit=50", gotQuery)
		}
	})

	t.Run("未配置release_url", func(t *testing.T) {
		if _, err := New(config.UpdateConfig{}, "0.5.0").Changelog(20); err == nil {
			t.Errorf("未配置 release_url 应报错")
		}
	})
}

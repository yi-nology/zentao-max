package zentao

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	zentaosdk "github.com/yi-nology/zentao-max/backend/core/zentao/sdk"
	"github.com/yi-nology/zentao-max/backend/core/config"
	"github.com/yi-nology/zentao-max/backend/core/logger"
)

// initTestLogger 用最小配置初始化全局 logger（会话登录成功路径会写日志）。
func initTestLogger(t *testing.T) {
	t.Helper()
	_ = logger.Init(&config.LogConfig{Level: "info", Format: "console"})
}

// 这些测试验证会话模式（session mode）的 .json → SDK 类型映射逻辑。
// 它们针对 testdata/ 目录下脱敏的真实样本（从 your-zentao-server.com 采集）运行，
// 不需要任何外部凭据，因此可以安全地在 CI 中运行。

// TestSessionBugMapping 验证 bug-browse.json 样本能正确映射到 SDK 的 zentaosdk.Bug。
// 关键点：openedBy/assignedTo 是字符串账号，应映射到 UserRef.Account。
func TestSessionBugMapping(t *testing.T) {
	raw, err := os.ReadFile("testdata/sample_bug_browse.json")
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}
	var wrapper struct {
		BugsSample []sessionBug  `json:"bugs_sample"`
		Pager      *sessionPager `json:"pager"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(wrapper.BugsSample) == 0 {
		t.Fatal("bugs_sample is empty")
	}
	b := wrapper.BugsSample[0].toSDK()

	if b.ID == 0 {
		t.Error("ID not mapped")
	}
	if b.Title == "" {
		t.Error("Title not mapped")
	}
	if b.OpenedBy.Account == "" {
		t.Error("OpenedBy.Account not mapped (string→UserRef 失败)")
	}
	if b.AssignedTo.Account == "" {
		t.Error("AssignedTo.Account not mapped (string→UserRef 失败)")
	}
	if b.Product == 0 {
		t.Error("Product not mapped")
	}
	if b.Status == "" {
		t.Error("Status not mapped")
	}
	if toInt(b.Severity) == 0 {
		t.Error("Severity not mapped")
	}
	t.Logf("mapped bug OK: id=%d title=%q openedBy=%s assignedTo=%s severity=%v pri=%v",
		b.ID, b.Title, b.OpenedBy.Account, b.AssignedTo.Account, b.Severity, b.Pri)
}

// TestSessionStoryMapping 验证需求样本映射（stories 是 map[id]story）。
func TestSessionStoryMapping(t *testing.T) {
	raw, err := os.ReadFile("testdata/sample_product_browse_story.json")
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}
	var wrapper struct {
		StoriesSample map[string]sessionStory `json:"stories_sample"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(wrapper.StoriesSample) == 0 {
		t.Fatal("stories_sample is empty")
	}
	for _, s := range wrapper.StoriesSample {
		mapped := s.toSDK()
		if mapped.ID == 0 {
			t.Error("ID not mapped")
		}
		if mapped.Title == "" {
			t.Error("Title not mapped")
		}
		t.Logf("mapped story OK: id=%d title=%q status=%s", mapped.ID, mapped.Title, mapped.Status)
		return
	}
}

// TestSessionProductMapping 验证产品样本映射（productStats 是 map[id]product）。
func TestSessionProductMapping(t *testing.T) {
	raw, err := os.ReadFile("testdata/sample_product_all.json")
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}
	var wrapper struct {
		Inner struct {
			ProductStats map[string]sessionProduct `json:"productStats"`
		} `json:"inner"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(wrapper.Inner.ProductStats) == 0 {
		t.Fatal("productStats is empty")
	}
	for _, p := range wrapper.Inner.ProductStats {
		mapped := p.toSDK()
		if mapped.ID == 0 || mapped.Name == "" {
			t.Errorf("product mapping incomplete: %+v", mapped)
		}
		t.Logf("mapped product OK: id=%d name=%q", mapped.ID, mapped.Name)
		return
	}
}

// TestSessionProjectMapping 验证项目样本映射。
func TestSessionProjectMapping(t *testing.T) {
	raw, err := os.ReadFile("testdata/sample_project_browse.json")
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}
	var wrapper struct {
		ProjectStatsSample map[string]sessionProject `json:"projectStats_sample"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(wrapper.ProjectStatsSample) == 0 {
		t.Fatal("projectStats_sample is empty")
	}
	for _, p := range wrapper.ProjectStatsSample {
		mapped := p.toSDK()
		if mapped.ID == 0 || mapped.Name == "" {
			t.Errorf("project mapping incomplete: %+v", mapped)
		}
		t.Logf("mapped project OK: id=%d name=%q model=%s product=%d", mapped.ID, mapped.Name, mapped.Model, p.Product)
		return
	}
}

// TestSessionProjectFilterByProduct 验证 sessionProject.Product 字段被正确解析。
// 这是修复 scheduler 页面"项目拉不全"的核心回归测试：
// 禅道 project-browse-0-all 端点实测不返回 product 字段（全 0），所以过滤走
// bug 反推路径（fetchProductProjectIDsViaBugs），保证 product→project 映射正确。
func TestSessionProjectFilterByProduct(t *testing.T) {
	raw := []byte(`{
		"projectStats": {
			"8819": {"id":8819,"name":"A","project":557},
			"5570": {"id":5570,"name":"B","project":0},
			"5571": {"id":5571,"name":"C","project":557},
			"5572": {"id":5572,"name":"D","project":999}
		}
	}`)
	var resp sessionProjectBrowseResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	all := make([]sessionProject, 0, len(resp.ProjectStats))
	for _, p := range resp.ProjectStats {
		all = append(all, p)
	}
	if len(all) != 4 {
		t.Fatalf("expected 4 projects, got %d", len(all))
	}
	// 模拟 productID=557 过滤
	const productID = 557
	var filtered []sessionProject
	for _, p := range all {
		if p.Product == productID {
			filtered = append(filtered, p)
		}
	}
	if len(filtered) != 2 {
		t.Fatalf("productID=%d expected 2 projects, got %d", productID, len(filtered))
	}
	ids := []int{filtered[0].ID, filtered[1].ID}
	if !((ids[0] == 8819 && ids[1] == 5571) || (ids[0] == 5571 && ids[1] == 8819)) {
		t.Errorf("productID=%d unexpected ids: %v", productID, ids)
	}
}

// TestSessionTaskMapping 验证任务样本映射。
func TestSessionTaskMapping(t *testing.T) {
	raw, err := os.ReadFile("testdata/sample_execution_task.json")
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}
	var wrapper struct {
		TasksSample map[string]sessionTask `json:"tasks_sample"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(wrapper.TasksSample) == 0 {
		t.Fatal("tasks_sample is empty")
	}
	for _, tk := range wrapper.TasksSample {
		mapped := tk.toSDK()
		if mapped.ID == 0 || mapped.Name == "" {
			t.Errorf("task mapping incomplete: %+v", mapped)
		}
		t.Logf("mapped task OK: id=%d name=%q status=%s", mapped.ID, mapped.Name, mapped.Status)
		return
	}
}

// TestSessionPagerParse 验证 pager 字段解析（分页依赖 recTotal/pageTotal）。
func TestSessionPagerParse(t *testing.T) {
	raw, err := os.ReadFile("testdata/sample_bug_browse.json")
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}
	var wrapper struct {
		Pager *sessionPager `json:"pager"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if wrapper.Pager == nil {
		t.Fatal("pager is nil")
	}
	if pagerInt(wrapper.Pager.RecTotal) == 0 {
		t.Error("RecTotal is 0 — pagination will break")
	}
	if pagerInt(wrapper.Pager.PageTotal) == 0 {
		t.Error("PageTotal is 0")
	}
	t.Logf("pager OK: recTotal=%d recPerPage=%d pageTotal=%d",
		pagerInt(wrapper.Pager.RecTotal), pagerInt(wrapper.Pager.RecPerPage), pagerInt(wrapper.Pager.PageTotal))
}

// TestContainsFold 验证 SearchBugs 的关键字匹配（小写 ASCII）。
func TestContainsFold(t *testing.T) {
	cases := []struct {
		s, sub string
		want   bool
	}{
		{"Login fails", "login", true},
		{"crash on startup", "STARTUP", true},
		{"", "x", false},
		{"abc", "", true},
		{"abc", "abcd", false},
	}
	for _, c := range cases {
		if got := containsFold(c.s, c.sub); got != c.want {
			t.Errorf("containsFold(%q,%q)=%v, want %v", c.s, c.sub, got, c.want)
		}
	}
}

// ---- 以下是环境变量门控的实时集成测试 ----
//
// 通过 ZENTAO_SESSION_URL / ZENTAO_SESSION_ACCOUNT / ZENTAO_SESSION_PASSWORD /
// ZENTAO_SESSION_PRODUCT_ID 触发，未设置时 t.Skip。用于验证 your-zentao-server.com 的
// 真实登录 + 数据访问链路。CI 默认不跑。
//
//	go test ./core/zentao/ -run TestSessionLive -v \
//	  -timeout 120s \
//	  ZENTAO_SESSION_URL=https://your-zentao-server.com \
//	  ZENTAO_SESSION_ACCOUNT=xxx ZENTAO_SESSION_PASSWORD=xxx \
//	  ZENTAO_SESSION_PRODUCT_ID=1029

func TestSessionLive(t *testing.T) {
	url := os.Getenv("ZENTAO_SESSION_URL")
	account := os.Getenv("ZENTAO_SESSION_ACCOUNT")
	password := os.Getenv("ZENTAO_SESSION_PASSWORD")
	productID := os.Getenv("ZENTAO_SESSION_PRODUCT_ID")
	if url == "" || account == "" || password == "" || productID == "" {
		t.Skip("ZENTAO_SESSION_* env not set, skipping live session test")
	}
	realm := os.Getenv("ZENTAO_SESSION_REALM")
	if realm == "" {
		realm = RealmKySSO
	}

	// 初始化 logger（login 成功路径会写日志）。
	initTestLogger(t)

	client := NewSessionClient(url, account, password, realm, "", nil)
	if err := client.SessionLoginSync(); err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if !client.IsConnected() {
		t.Fatal("login reported success but client not connected")
	}

	ctx := context.Background()
	pid := atoiOr(productID, 0)
	if pid == 0 {
		t.Fatal("invalid ZENTAO_SESSION_PRODUCT_ID")
	}

	// 1. 当前用户
	me, err := client.getCurrentUserSession(ctx)
	if err != nil {
		t.Errorf("GetCurrentUser failed: %v", err)
	} else {
		t.Logf("current user: account=%s realname=%s role=%s", me.Account, me.Realname, me.Role)
	}

	// 2. 产品列表
	prods, err := client.getProductsSession(ctx)
	if err != nil {
		t.Errorf("GetProducts failed: %v", err)
	} else {
		t.Logf("products: %d", len(prods))
	}

	// 3. Bug（按 productID）
	bugs, err := client.getAllBugsSession(ctx, pid)
	if err != nil {
		t.Errorf("GetAllBugs(product=%d) failed: %v", pid, err)
	} else {
		t.Logf("bugs for product %d: %d", pid, len(bugs))
	}

	// 4. Story
	stories, err := client.getAllStoriesSession(ctx, pid)
	if err != nil {
		t.Errorf("GetAllStories(product=%d) failed: %v", pid, err)
	} else {
		t.Logf("stories for product %d: %d", pid, len(stories))
	}

	// 5. 验证 Bug 字段映射完整性（至少首条）
	if len(bugs) > 0 {
		b := bugs[0]
		if b.OpenedBy.Account == "" || b.Title == "" {
			t.Errorf("bug field mapping incomplete: %+v", b)
		}
	}
}

// 确保 zentao 包被引用（filterBugs 等用到了）。
var _ = zentaosdk.BugSearchParams{}

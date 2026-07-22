package zentao

import (
	"context"
	"fmt"
	"strconv"

	zentaosdk "github.com/yi-nology/zentao-max/backend/core/zentao/sdk"
)

// ---- Bugs (session mode) ----
//
// 端点：/bug-browse-{productID}-0-{browseType}-0--{recTotal}-{recPerPage}-{page}.json
// 其中 browseType 常用 "all" / "unclosed" / "active"；recTotal 第一次未知，
// 先用 0 请求第一页拿到真实 recTotal，后续翻页带上真实值。
// （受限实例上 recTotal 留 0 也能正确返回第一页与 pager.recTotal。）

const sessionBugRecPerPage = 100

// sessionBugBrowseResp 对应 /bug-browse-*.json 内层 data。
type sessionBugBrowseResp struct {
	Title     string        `json:"title"`
	ProductID int           `json:"productID"`
	Bugs      []sessionBug  `json:"bugs"`
	Pager     *sessionPager `json:"pager"`
}

// sessionBug 是 *.json 端点里单条 bug 的原始结构。
// 与 SDK zentaosdk.Bug 的关键差异：openedBy/assignedTo 是账号字符串（SDK 是 UserRef 对象），
// openedBuild 是字符串（SDK 是 FlexibleString），severity/pri 是 int（SDK 是 interface{}）。
type sessionBug struct {
	ID           int    `json:"id"`
	Product      int    `json:"product"`
	Project      int    `json:"project"`
	Branch       int    `json:"branch"`
	Module       int    `json:"module"`
	Execution    int    `json:"execution"`
	Title        string `json:"title"`
	Keywords     string `json:"keywords"`
	Severity     int    `json:"severity"`
	Pri          int    `json:"pri"`
	Type         string `json:"type"`
	OS           string `json:"os"`
	Browser      string `json:"browser"`
	Hardware     string `json:"hardware"`
	Steps        string `json:"steps"`
	Task         int    `json:"task"`
	Story        int    `json:"story"`
	Confirmed    int    `json:"confirmed"`
	Status       string `json:"status"`
	OpenedBy     string `json:"openedBy"`
	OpenedDate   string `json:"openedDate"`
	OpenedBuild  string `json:"openedBuild"`
	AssignedTo   string `json:"assignedTo"`
	AssignedDate string `json:"assignedDate"`
	Deadline     string `json:"deadline"`
	ResolvedBy   string `json:"resolvedBy"`
	Resolution   string `json:"resolution"`
	ResolvedDate string `json:"resolvedDate"`
	ClosedBy     string `json:"closedBy"`
	ClosedDate   string `json:"closedDate"`
	LastEditedBy string `json:"lastEditedBy"`
	Deleted      string `json:"deleted"`
}

func (b sessionBug) toSDK() zentaosdk.Bug {
	return zentaosdk.Bug{
		ID:            b.ID,
		Project:       b.Project,
		Product:       b.Product,
		Branch:        b.Branch,
		Module:        b.Module,
		Execution:     b.Execution,
		Title:         b.Title,
		Keywords:      b.Keywords,
		Severity:      b.Severity,
		Pri:           b.Pri,
		Type:          b.Type,
		OS:            b.OS,
		Browser:       b.Browser,
		Hardware:      b.Hardware,
		Steps:         b.Steps,
		Task:          b.Task,
		Story:         b.Story,
		Confirmed:     b.Confirmed,
		Status:        b.Status,
		OpenedBy:      zentaosdk.UserRef{Account: b.OpenedBy},
		OpenedDate:    b.OpenedDate,
		OpenedBuild:   zentaosdk.FlexibleString(nonEmpty(b.OpenedBuild)),
		AssignedTo:    zentaosdk.UserRef{Account: b.AssignedTo},
		AssignedDate:  b.AssignedDate,
		Deadline:      b.Deadline,
		ResolvedBy:    b.ResolvedBy,
		Resolution:    b.Resolution,
		ResolvedDate:  b.ResolvedDate,
		ClosedBy:      b.ClosedBy,
		ClosedDate:    b.ClosedDate,
		LastEditedBy:  b.LastEditedBy,
		Deleted:       b.Deleted,
		StatusName:    b.Status,
		ExecutionName: "",
	}
}

// nonEmpty 返回单元素切片或 nil（用于 FlexibleString 字段）。
func nonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}

// fetchBugsPageSession 拉取 bug-browse 的单页。recTotal 传 0 表示首探。
func (c *Client) fetchBugsPageSession(ctx context.Context, productID, page int, browseType string, recTotal int) ([]zentaosdk.Bug, *sessionPager, error) {
	path := fmt.Sprintf("/bug-browse-%d-0-%s-0--%d-%d-%d.json",
		productID, browseType, recTotal, sessionBugRecPerPage, page)
	var resp sessionBugBrowseResp
	if err := c.doSessionJSON(ctx, "GetBugs", path, &resp); err != nil {
		return nil, nil, err
	}
	bugs := make([]zentaosdk.Bug, 0, len(resp.Bugs))
	for _, b := range resp.Bugs {
		bugs = append(bugs, b.toSDK())
	}
	return bugs, resp.Pager, nil
}

// getAllBugsSession 自动翻页拉取产品全部 bug（含 closed，browseType=all）。
// 优先复用上层 cache（与 Token 模式 GetAllBugsContext 一致）。
func (c *Client) getAllBugsSession(ctx context.Context, productID int) ([]zentaosdk.Bug, error) {
	var all []zentaosdk.Bug
	page := 1
	recTotal := 0
	for {
		if err := ctx.Err(); err != nil {
			return all, err
		}
		bugs, pager, err := c.fetchBugsPageSession(ctx, productID, page, "all", recTotal)
		if err != nil {
			return all, err
		}
		if pager != nil && recTotal == 0 {
			recTotal = pagerInt(pager.RecTotal)
		}
		all = append(all, bugs...)
		if len(bugs) < sessionBugRecPerPage {
			return all, nil
		}
		// 兜底：pager 缺失或已翻完。
		if pager != nil && page >= pagerInt(pager.PageTotal) {
			return all, nil
		}
		page++
		if page > 1000 { // 安全闸，避免死循环
			return all, fmt.Errorf("bug 翻页超过 1000 页，疑似异常")
		}
	}
}

func (c *Client) getBugsSession(ctx context.Context, productID, page, pageSize int) ([]zentaosdk.Bug, error) {
	bugs, _, err := c.fetchBugsPageSession(ctx, productID, page, "all", 0)
	_ = pageSize
	return bugs, err
}

func (c *Client) getBugsByProjectSession(ctx context.Context, productID, projectID, page, pageSize int) ([]zentaosdk.Bug, error) {
	all, err := c.getAllBugsSession(ctx, productID)
	if err != nil {
		return nil, err
	}
	out := make([]zentaosdk.Bug, 0)
	for _, b := range all {
		if b.Project == projectID {
			out = append(out, b)
		}
	}
	// page/pageSize 简单分页。
	return paginateBugs(out, page, pageSize), nil
}

func (c *Client) getAllBugsByProjectSession(ctx context.Context, projectID int) ([]zentaosdk.Bug, error) {
	// 产品维度未知，取全部产品全部 bug 后按 project 过滤。
	products, err := c.getProductsSession(ctx)
	if err != nil {
		return nil, err
	}
	var out []zentaosdk.Bug
	for _, p := range products {
		bugs, berr := c.getAllBugsSession(ctx, p.ID)
		if berr != nil {
			// 单产品失败不阻断整体（可能是权限不足），记日志继续。
			out = append(out, bugs...)
			continue
		}
		for _, b := range bugs {
			if b.Project == projectID {
				out = append(out, b)
			}
		}
	}
	return out, nil
}

func (c *Client) searchBugsSession(ctx context.Context, params zentaosdk.BugSearchParams) ([]zentaosdk.Bug, error) {
	// 复用 token 模式的内存过滤语义：先拉全部，再按条件筛。
	// 注意：ctx 必须真传下去，否则调用方取消 / 超时不会终止底层翻页循环。
	all, err := c.getAllBugsSession(ctx, params.ProductID)
	if err != nil {
		return nil, err
	}
	return filterBugs(all, params), nil
}

// paginateBugs 内存分页（1-based page）。
func paginateBugs(in []zentaosdk.Bug, page, pageSize int) []zentaosdk.Bug {
	if pageSize <= 0 {
		pageSize = 100
	}
	if page <= 0 {
		page = 1
	}
	start := (page - 1) * pageSize
	if start >= len(in) {
		return nil
	}
	end := start + pageSize
	if end > len(in) {
		end = len(in)
	}
	return in[start:end]
}

// filterBugs 在内存中按 BugSearchParams 过滤（与 SDK SearchBugs 语义一致）。
func filterBugs(in []zentaosdk.Bug, p zentaosdk.BugSearchParams) []zentaosdk.Bug {
	out := make([]zentaosdk.Bug, 0, len(in))
	for _, b := range in {
		if p.Status != "" && b.Status != p.Status {
			continue
		}
		if p.AssignedTo != "" && b.AssignedTo.Account != p.AssignedTo {
			continue
		}
		if p.Severity != 0 && toInt(b.Severity) != p.Severity {
			continue
		}
		if p.Pri != 0 && toInt(b.Pri) != p.Pri {
			continue
		}
		if p.Keyword != "" {
			kw := p.Keyword
			if !containsFold(b.Title, kw) && strconv.Itoa(b.ID) != kw {
				continue
			}
		}
		out = append(out, b)
	}
	return out
}

// toInt 把 interface{} 安全转 int（severity/pri 在 SDK 里是 interface{}）。
func toInt(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	}
	return 0
}

func containsFold(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	ls := lowerASCII(s)
	lsub := lowerASCII(substr)
	for i := 0; i+len(lsub) <= len(ls); i++ {
		if ls[i:i+len(lsub)] == lsub {
			return true
		}
	}
	return false
}

func lowerASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

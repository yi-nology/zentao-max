package zentao

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/yi-nology/common/biz/zentao"
)

// ---- Test Cases (session mode) ----
// 端点：/testcase-browse-{productID}.json（第一页）/ 带分页段翻页。
// 注意：case 的 id 在受限实例上是字符串 "case_2343233"，需特殊解析。
// 内层 cases 是 list（不是 map），pager.recTotal/pageTotal 用于翻页。

type sessionTestCaseBrowseResp struct {
	Title string        `json:"title"`
	Cases []sessionCase `json:"cases"`
	Pager *sessionPager `json:"pager"`
}

type sessionCase struct {
	// ID 在受限实例上是字符串 "case_2343233"，在标准实例上是 int。
	// 用 RawMessage 兼容两者，由 parseCaseID 提取数字。
	ID           json.RawMessage `json:"id"`
	Product      int             `json:"product"`
	Project      int             `json:"project"`
	Module       json.Number     `json:"module"`
	Story        json.Number     `json:"story"`
	Title        string          `json:"title"`
	Precondition string          `json:"precondition"`
	Keywords     string          `json:"keywords"`
	Pri          json.Number     `json:"pri"`
	Type         string          `json:"type"`
	Status       string          `json:"status"`
	OpenedBy     string          `json:"openedBy"`
	OpenedDate   string          `json:"openedDate"`
	LastEditedBy string          `json:"lastEditedBy"`
	Version      json.Number     `json:"version"`
	Deleted      string          `json:"deleted"`
}

func (c sessionCase) toSDK() zentao.Case {
	id := parseCaseID(c.ID)
	module, _ := c.Module.Int64()
	story, _ := c.Story.Int64()
	pri, _ := c.Pri.Int64()
	ver, _ := c.Version.Int64()
	return zentao.Case{
		ID:           id,
		Product:      c.Product,
		Project:      c.Project,
		Module:       int(module),
		Story:        int(story),
		Title:        c.Title,
		Precondition: c.Precondition,
		Keywords:     c.Keywords,
		Pri:          int(pri),
		Type:         c.Type,
		Status:       c.Status,
		OpenedBy:     zentao.UserRef{Account: c.OpenedBy},
		OpenedDate:   c.OpenedDate,
		LastEditedBy: &zentao.UserRef{Account: c.LastEditedBy},
		Version:      int(ver),
	}
}

// parseCaseID 从 case id 的原始 JSON（字符串或数字）提取数字部分。
// 受限实例返回 "case_2343233"，标准实例返回 int。
func parseCaseID(raw json.RawMessage) int {
	s := string(raw)
	// 去掉 JSON 字符串的引号
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	digits := ""
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits += string(r)
		}
	}
	if digits == "" {
		return 0
	}
	id, _ := strconv.Atoi(digits)
	return id
}

// getCasesByProductSession 拉取产品下的用例列表（受限 session 端点 /testcase-browse-{pid}.json）。
//
// Deprecated: 受限实例的 testcase-browse 端点不支持 server-side 翻页（分页段 URL 返空），
// 即便 pageSize=0 全量翻页也只能拿 page 1 的 20 条。请用 getCasesByProductV1 走 v1 REST API。
// 保留本方法仅为向后兼容, 不再被 GetCasesByProduct 调用。
//
//   - pageSize > 0: 单页模式, 按 page 拉取对应页（依赖 testcase-browse 分页段 URL, 受限实例不支持）。
//   - pageSize <= 0: 全量模式, 自动翻页取全部, 返回的 recTotal 是最终累加长度。
func (c *Client) getCasesByProductSession(ctx context.Context, productID, page, pageSize int) ([]zentao.Case, int, error) {
	// testcase-browse 分页段格式：/testcase-browse-{pid}-{branch}-{browseType}-{recTotal}-{recPerPage}-{page}.json
	// recPerPage 必须从 server Pager 取真实值（受限实例=20, 其它=20/50/100），不能 hardcode。
	defaultRecPerPage := 20
	if pageSize > 0 {
		if page <= 0 {
			page = 1
		}
		// 1) 先拿 recTotal + recPerPage（用第一页探一次）
		firstPath := fmt.Sprintf("/testcase-browse-%d.json", productID)
		var firstResp sessionTestCaseBrowseResp
		if err := c.doSessionJSON(ctx, "GetCasesByProduct", firstPath, &firstResp); err != nil {
			if IsSessionAccessDenied(err) {
				return nil, 0, nil
			}
			return nil, 0, err
		}
		recTotal := 0
		recPerPage := defaultRecPerPage
		if firstResp.Pager != nil {
			recTotal = pagerInt(firstResp.Pager.RecTotal)
			if rpp := pagerInt(firstResp.Pager.RecPerPage); rpp > 0 {
				recPerPage = rpp
			}
		}
		// page=1 时, 首探结果本身就是目标页, 直接用
		if page == 1 {
			out := make([]zentao.Case, 0, len(firstResp.Cases))
			for _, cs := range firstResp.Cases {
				out = append(out, cs.toSDK())
			}
			return out, recTotal, nil
		}
		// 越界保护：page 超过总页数时直接返空, total 给真实值
		if recTotal > 0 && page > (recTotal+recPerPage-1)/recPerPage {
			return nil, recTotal, nil
		}
		// 2) 拉目标页（带分页段）
		path := fmt.Sprintf("/testcase-browse-%d-0-all-%d-%d-%d.json", productID, recTotal, recPerPage, page)
		var resp sessionTestCaseBrowseResp
		if err := c.doSessionJSON(ctx, "GetCasesByProduct", path, &resp); err != nil {
			if IsSessionAccessDenied(err) {
				return nil, recTotal, nil
			}
			return nil, recTotal, err
		}
		out := make([]zentao.Case, 0, len(resp.Cases))
		for _, cs := range resp.Cases {
			out = append(out, cs.toSDK())
		}
		return out, recTotal, nil
	}

	// 全量模式：自动翻页拉完
	var out []zentao.Case
	recTotal := 0
	recPerPage := defaultRecPerPage
	for p := 1; ; p++ {
		var path string
		if recTotal == 0 {
			path = fmt.Sprintf("/testcase-browse-%d.json", productID)
		} else {
			path = fmt.Sprintf("/testcase-browse-%d-0-all-%d-%d-%d.json", productID, recTotal, recPerPage, p)
		}
		var resp sessionTestCaseBrowseResp
		if err := c.doSessionJSON(ctx, "GetCasesByProduct", path, &resp); err != nil {
			if IsSessionAccessDenied(err) {
				return out, recTotal, nil
			}
			return out, recTotal, err
		}
		if recTotal == 0 && resp.Pager != nil {
			recTotal = pagerInt(resp.Pager.RecTotal)
			if rpp := pagerInt(resp.Pager.RecPerPage); rpp > 0 {
				recPerPage = rpp
			}
		}
		for _, cs := range resp.Cases {
			out = append(out, cs.toSDK())
		}
		if len(resp.Cases) < recPerPage {
			return out, recTotal, nil
		}
		if resp.Pager != nil && p >= pagerInt(resp.Pager.PageTotal) {
			return out, recTotal, nil
		}
		if p > 500 {
			return out, recTotal, nil
		}
	}
}

// getAllCasesSession Deprecated: 请用 GetAllCases 走 v1 REST API。
func (c *Client) getAllCasesSession(ctx context.Context, productID int) ([]zentao.Case, error) {
	out, _, err := c.getCasesByProductSession(ctx, productID, 1, 0) // pageSize=0 表示翻页取全部
	return out, err
}

// ---- Test Cases (v1 REST API, cookie 鉴权) ----
// 受限实例的 testcase-browse 端点不支持 server-side 翻页 (翻页段 URL 返空),
// 但 /api.php/v1/testcases?product=&page=&limit= 用 cookie 鉴权能真翻页.
// 响应: {"page":"1","total":1235,"limit":20,"testcases":[{...}]} (page 是字符串).

// v1CasesResp 接收 v1 响应. Page 用 string 兼容受限实例返 "1"/"2".
type v1CasesResp struct {
	Page      string             `json:"page"`
	Total     int                `json:"total"`
	Limit     int                `json:"limit"`
	Testcases []v1CaseItem       `json:"testcases"`
}

type v1CaseItem struct {
	ID           json.RawMessage `json:"id"`
	Product      int             `json:"product"`
	Project      int             `json:"project"`
	Module       json.Number     `json:"module"`
	Story        json.Number     `json:"story"`
	Title        string          `json:"title"`
	Precondition string          `json:"precondition"`
	Keywords     string          `json:"keywords"`
	Pri          json.Number     `json:"pri"`
	Type         string          `json:"type"`
	Status       string          `json:"status"`
	OpenedBy     *v1UserRef      `json:"openedBy"`
	OpenedDate   string          `json:"openedDate"`
	LastEditedBy *v1UserRef      `json:"lastEditedBy"`
	Version      json.Number     `json:"version"`
}

type v1UserRef struct {
	Account  string `json:"account"`
	Realname string `json:"realname"`
}

func (c v1CaseItem) toSDK() zentao.Case {
	id := parseCaseID(c.ID)
	module, _ := c.Module.Int64()
	story, _ := c.Story.Int64()
	pri, _ := c.Pri.Int64()
	ver, _ := c.Version.Int64()
	out := zentao.Case{
		ID:           id,
		Product:      c.Product,
		Project:      c.Project,
		Module:       int(module),
		Story:        int(story),
		Title:        c.Title,
		Precondition: c.Precondition,
		Keywords:     c.Keywords,
		Pri:          int(pri),
		Type:         c.Type,
		Status:       c.Status,
		OpenedDate:   c.OpenedDate,
		Version:      int(ver),
	}
	if c.OpenedBy != nil {
		out.OpenedBy = zentao.UserRef{Account: c.OpenedBy.Account, Realname: c.OpenedBy.Realname}
	}
	if c.LastEditedBy != nil {
		out.LastEditedBy = &zentao.UserRef{Account: c.LastEditedBy.Account, Realname: c.LastEditedBy.Realname}
	}
	return out
}

// getCasesByProductV1 走 v1 REST API 拉取产品下的用例单页.
// path: /api.php/v1/testcases?product={pid}&page={page}&limit={limit}.
// 返 ([]Case, total, err). total 直接来自响应 total 字段.
func (c *Client) getCasesByProductV1(ctx context.Context, productID, page, pageSize int) ([]zentao.Case, int, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	if page <= 0 {
		page = 1
	}
	// v1 limit 上限通常 20-50, 超过 50 某些实例会返 404. 用 20 安全.
	if pageSize > 20 {
		pageSize = 20
	}
	path := fmt.Sprintf("/api.php/v1/testcases?product=%d&page=%d&limit=%d", productID, page, pageSize)
	var resp v1CasesResp
	if err := c.doV1JSON(ctx, "GetCasesByProduct", path, &resp); err != nil {
		if IsSessionAccessDenied(err) {
			return nil, 0, nil
		}
		return nil, 0, err
	}
	out := make([]zentao.Case, 0, len(resp.Testcases))
	for _, tc := range resp.Testcases {
		out = append(out, tc.toSDK())
	}
	return out, resp.Total, nil
}

// ---- Test Tasks (session mode) ----
// 端点：/testtask-browse-{productID}.json，内层 tasks 是 map[id]testtask。

type sessionTestTaskBrowseResp struct {
	Title string                     `json:"title"`
	Tasks map[string]sessionTestTask `json:"tasks"`
	Pager *sessionPager              `json:"pager"`
}

type sessionTestTask struct {
	ID        int         `json:"id"`
	Project   int         `json:"project"`
	Product   int         `json:"product"`
	Name      string      `json:"name"`
	Execution int         `json:"execution"`
	Build     json.Number `json:"build"`
	Type      string      `json:"type"`
	Owner     string      `json:"owner"`
	Pri       int         `json:"pri"`
	Begin     string      `json:"begin"`
	End       string      `json:"end"`
	Status    string      `json:"status"`
	Deleted   string      `json:"deleted"`
}

func (t sessionTestTask) toSDK() zentao.TestTask {
	build, _ := t.Build.Int64()
	return zentao.TestTask{
		ID:        t.ID,
		Project:   t.Project,
		Product:   t.Product,
		Name:      t.Name,
		Execution: t.Execution,
		Build:     int(build),
		Type:      t.Type,
		Owner:     zentao.UserRef{Account: t.Owner},
		Pri:       t.Pri,
		Begin:     t.Begin,
		End:       t.End,
		Status:    t.Status,
		Deleted:   t.Deleted,
	}
}

func (c *Client) getTestTasksSession(ctx context.Context, productID, page, pageSize int) ([]zentao.TestTask, error) {
	path := fmt.Sprintf("/testtask-browse-%d.json", productID)
	var resp sessionTestTaskBrowseResp
	if err := c.doSessionJSON(ctx, "GetTestTasks", path, &resp); err != nil {
		if IsSessionAccessDenied(err) {
			return nil, nil
		}
		return nil, err
	}
	all := make([]zentao.TestTask, 0, len(resp.Tasks))
	for _, t := range resp.Tasks {
		all = append(all, t.toSDK())
	}
	if pageSize <= 0 {
		return all, nil
	}
	start := (page - 1) * pageSize
	if start < 0 {
		start = 0
	}
	end := start + pageSize
	if end > len(all) {
		end = len(all)
	}
	if start >= len(all) {
		return nil, nil
	}
	return all[start:end], nil
}

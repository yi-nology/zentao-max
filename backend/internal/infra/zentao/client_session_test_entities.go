package zentao

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	zentaosdk "github.com/yi-nology/zentao-max/backend/internal/infra/zentao/sdk"
)

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

// ---- Test Cases (v1 REST API, cookie 鉴权) ----
// 受限实例的 testcase-browse 端点不支持 server-side 翻页 (翻页段 URL 返空),
// 但 /api.php/v1/testcases?product=&page=&limit= 用 cookie 鉴权能真翻页.
// 响应: {"page":"1","total":1235,"limit":20,"testcases":[{...}]} (page 是字符串).

// v1CasesResp 接收 v1 响应. Page 用 string 兼容受限实例返 "1"/"2".
type v1CasesResp struct {
	Page      string       `json:"page"`
	Total     int          `json:"total"`
	Limit     int          `json:"limit"`
	Testcases []v1CaseItem `json:"testcases"`
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

func (c v1CaseItem) toSDK() zentaosdk.Case {
	id := parseCaseID(c.ID)
	module, _ := c.Module.Int64()
	story, _ := c.Story.Int64()
	pri, _ := c.Pri.Int64()
	ver, _ := c.Version.Int64()
	out := zentaosdk.Case{
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
		out.OpenedBy = zentaosdk.UserRef{Account: c.OpenedBy.Account, Realname: c.OpenedBy.Realname}
	}
	if c.LastEditedBy != nil {
		out.LastEditedBy = &zentaosdk.UserRef{Account: c.LastEditedBy.Account, Realname: c.LastEditedBy.Realname}
	}
	return out
}

// getCasesByProductV1 走 v1 REST API 拉取产品下的用例单页.
// path: /api.php/v1/testcases?product={pid}&page={page}&limit={limit}.
// 返 ([]Case, total, err). total 直接来自响应 total 字段.
func (c *Client) getCasesByProductV1(ctx context.Context, productID, page, pageSize int) ([]zentaosdk.Case, int, error) {
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
	out := make([]zentaosdk.Case, 0, len(resp.Testcases))
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

func (t sessionTestTask) toSDK() zentaosdk.TestTask {
	build, _ := t.Build.Int64()
	return zentaosdk.TestTask{
		ID:        t.ID,
		Project:   t.Project,
		Product:   t.Product,
		Name:      t.Name,
		Execution: t.Execution,
		Build:     int(build),
		Type:      t.Type,
		Owner:     zentaosdk.UserRef{Account: t.Owner},
		Pri:       t.Pri,
		Begin:     t.Begin,
		End:       t.End,
		Status:    t.Status,
		Deleted:   t.Deleted,
	}
}

func (c *Client) getTestTasksSession(ctx context.Context, productID, page, pageSize int) ([]zentaosdk.TestTask, error) {
	path := fmt.Sprintf("/testtask-browse-%d.json", productID)
	var resp sessionTestTaskBrowseResp
	if err := c.doSessionJSON(ctx, "GetTestTasks", path, &resp); err != nil {
		if IsSessionAccessDenied(err) {
			return nil, nil
		}
		return nil, err
	}
	all := make([]zentaosdk.TestTask, 0, len(resp.Tasks))
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

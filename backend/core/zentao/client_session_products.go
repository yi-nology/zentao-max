package zentao

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	zentaosdk "github.com/yi-nology/zentao-max/backend/core/zentao/sdk"
)

// ---- Products ----

// sessionProductAllResp 对应 /product-all.json 内层 data。
type sessionProductAllResp struct {
	Title        string            `json:"title"`
	RecTotal     int               `json:"recTotal"`
	ProductStats sessionProductMap `json:"productStats"`
	Pager        *sessionPager     `json:"pager"`
}

// sessionProductMap 兼容 productStats 的两种返回形状：
// 正常页是 map[id]product，空页/错误页可能是 []。
type sessionProductMap map[string]sessionProduct

func (m *sessionProductMap) UnmarshalJSON(data []byte) error {
	// 数组：当作空 map 处理。
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err == nil {
		*m = sessionProductMap{}
		return nil
	}
	return json.Unmarshal(data, (*map[string]sessionProduct)(m))
}

// sessionProduct 是 product-all.json 中 productStats 的单个条目。
// 字段远多于 SDK 的 Product，我们只取业务需要的。
type sessionProduct struct {
	ID      int    `json:"id"`
	Program int    `json:"program"`
	Name    string `json:"name"`
	Code    string `json:"code"`
	Type    string `json:"type"`
	Status  string `json:"status"`
	Desc    string `json:"desc"`
	PO      string `json:"PO"`
}

func (p sessionProduct) toSDK() zentaosdk.Product {
	return zentaosdk.Product{
		ID:     p.ID,
		Name:   p.Name,
		Code:   p.Code,
		Type:   p.Type,
		Status: p.Status,
		Desc:   p.Desc,
	}
}

// getProductsSession 会话模式获取产品列表。
// 翻页段格式取自产品页真实 pager 链接（此前"保守猜测"的 /product-all-{page}.json
// 会把页码解析成 browseType，每页都返回第 1 页内容导致整表重复）：
//
//	/product-all-{browseType}-{orderBy}-{programID}-{recTotal}-{recPerPage}-{pageID}.json
func (c *Client) getProductsSession(ctx context.Context) ([]zentaosdk.Product, error) {
	var out []zentaosdk.Product
	seen := make(map[int]struct{})
	page := 1
	recTotal := 0
	const recPerPage = 20
	for {
		var path string
		if page == 1 {
			path = "/product-all.json"
		} else {
			path = fmt.Sprintf("/product-all-noclosed-program_asc-0-%d-%d-%d.json", recTotal, recPerPage, page)
		}
		var resp sessionProductAllResp
		// 第 2+ 页绕过缓存：各页 scope 相同，入缓存会命中第 1 页（见 doSessionJSONUncached）
		if page > 1 {
			if err := c.doSessionJSONUncached(ctx, "GetProducts", path, &resp); err != nil {
				return out, err
			}
		} else if err := c.doSessionJSON(ctx, "GetProducts", path, &resp); err != nil {
			return out, err
		}
		if recTotal == 0 && resp.Pager != nil {
			recTotal = pagerInt(resp.Pager.RecTotal)
		}
		dupBefore := len(out)
		for _, p := range resp.ProductStats {
			if _, ok := seen[p.ID]; ok {
				continue
			}
			seen[p.ID] = struct{}{}
			out = append(out, p.toSDK())
		}
		// 翻页未推进（同一页内容重复返回）或已到末页即收口，防死循环
		if len(out) == dupBefore && page > 1 {
			return out, nil
		}
		if resp.Pager == nil || page >= pagerInt(resp.Pager.PageTotal) || len(resp.ProductStats) == 0 {
			return out, nil
		}
		page++
		if page > 500 {
			return out, fmt.Errorf("product 翻页超过 500 页")
		}
	}
}

// ---- Projects ----

type sessionProjectBrowseResp struct {
	Title        string            `json:"title"`
	ProjectStats sessionProjectMap `json:"projectStats"`
	Projects     sessionProjectMap `json:"projects"` // 部分端点用 projects
	Pager        *sessionPager     `json:"pager"`
}

// sessionProjectMap 兼容 projectStats 数组/字典两种形状。
type sessionProjectMap map[string]sessionProject

func (m *sessionProjectMap) UnmarshalJSON(data []byte) error {
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err == nil {
		*m = sessionProjectMap{}
		return nil
	}
	return json.Unmarshal(data, (*map[string]sessionProject)(m))
}

type sessionProject struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Code    string `json:"code"`
	Model   string `json:"model"`
	Type    string `json:"type"`
	Status  string `json:"status"`
	PM      string `json:"PM"`
	Begin   string `json:"begin"`
	End     string `json:"end"`
	Product int    `json:"project"` // 关联产品 ID（禅道 projectStats.project 字段；0 表示无关联）
}

func (p sessionProject) toSDK() zentaosdk.Project {
	return zentaosdk.Project{
		ID:     p.ID,
		Name:   p.Name,
		Code:   p.Code,
		Model:  p.Model,
		Type:   p.Type,
		Status: p.Status,
		PM:     p.PM,
		Begin:  p.Begin,
		End:    p.End,
	}
}

func (c *Client) getProjectsByProductSession(ctx context.Context, productID int) ([]zentaosdk.Project, error) {
	if productID <= 0 {
		return c.getAllProjectsSession(ctx)
	}

	// 兜底：bug 反推
	all, err := c.getAllProjectsRaw(ctx)
	if err != nil {
		return nil, err
	}
	allSDK := make([]zentaosdk.Project, 0, len(all))
	byID := make(map[int]zentaosdk.Project, len(all))
	for _, p := range all {
		sdk := p.toSDK()
		allSDK = append(allSDK, sdk)
		byID[sdk.ID] = sdk
	}

	linkedProjectIDs, err := c.fetchProductProjectIDsViaBugs(ctx, productID)
	if err != nil {
		return []zentaosdk.Project{}, nil
	}
	out := make([]zentaosdk.Project, 0, len(linkedProjectIDs))
	for _, id := range linkedProjectIDs {
		if p, ok := byID[id]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}

// fetchProductProjectIDsViaBugs 通过 bug-browse 拉全量 bug 提取 product 关联的
// project ID 集合。禅道 project-browse 不返回 project 字段（实测 project=0），
// bug 端点 bug.Project 是 product→project 映射的唯一可靠路径。
func (c *Client) fetchProductProjectIDsViaBugs(ctx context.Context, productID int) ([]int, error) {
	bugs, err := c.getAllBugsSession(ctx, productID)
	if err != nil {
		return nil, err
	}
	seen := make(map[int]struct{}, 8)
	out := make([]int, 0, 8)
	for _, b := range bugs {
		if b.Project == 0 {
			continue
		}
		if _, ok := seen[b.Project]; ok {
			continue
		}
		seen[b.Project] = struct{}{}
		out = append(out, b.Project)
	}
	return out, nil
}

func (c *Client) getAllProjectsSession(ctx context.Context) ([]zentaosdk.Project, error) {
	raw, err := c.getAllProjectsRaw(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]zentaosdk.Project, 0, len(raw))
	for _, p := range raw {
		out = append(out, p.toSDK())
	}
	return out, nil
}

// getAllProjectsRaw 拉全量项目并保留 product 字段，供按产品过滤使用。
func (c *Client) getAllProjectsRaw(ctx context.Context) ([]sessionProject, error) {
	var out []sessionProject
	page := 1
	recTotal := 0
	const recPerPage = 100
	for {
		var path string
		if page == 1 {
			path = "/project-browse-0-all-0-order_asc-0-100-1.json"
		} else {
			path = fmt.Sprintf("/project-browse-0-all-0-order_asc-%d-%d-%d.json", recTotal, recPerPage, page)
		}
		var resp sessionProjectBrowseResp
		// 第 2+ 页绕过缓存：各页 scope 相同，入缓存会命中第 1 页（见 doSessionJSONUncached）
		if page > 1 {
			if err := c.doSessionJSONUncached(ctx, "GetAllProjects", path, &resp); err != nil {
				return out, err
			}
		} else if err := c.doSessionJSON(ctx, "GetAllProjects", path, &resp); err != nil {
			return out, err
		}
		if recTotal == 0 && resp.Pager != nil {
			recTotal = pagerInt(resp.Pager.RecTotal)
		}
		src := resp.ProjectStats
		if len(src) == 0 {
			src = resp.Projects
		}
		for _, p := range src {
			out = append(out, p)
		}
		if len(src) < recPerPage {
			return out, nil
		}
		if resp.Pager != nil && page >= pagerInt(resp.Pager.PageTotal) {
			return out, nil
		}
		page++
		if page > 500 {
			return out, fmt.Errorf("project 翻页超过 500 页")
		}
	}
}

func (c *Client) getProjectSession(ctx context.Context, projectID int) (*zentaosdk.Project, error) {
	projects, err := c.getAllProjectsSession(ctx)
	if err != nil {
		return nil, err
	}
	for i := range projects {
		if projects[i].ID == projectID {
			return &projects[i], nil
		}
	}
	return nil, fmt.Errorf("项目 %d 不存在", projectID)
}

// ---- Executions ----

type sessionExecutionAllResp struct {
	Title          string             `json:"title"`
	ExecutionStats []sessionExecution `json:"executionStats"`
	Executions     []sessionExecution `json:"executions"`
	Pager          *sessionPager      `json:"pager"`
}

type sessionExecution struct {
	ID        int    `json:"id"`
	Project   int    `json:"project"`
	Name      string `json:"name"`
	Code      string `json:"code"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	Begin     string `json:"begin"`
	End       string `json:"end"`
	RealBegan string `json:"realBegan"`
	RealEnd   string `json:"realEnd"`
	Desc      string `json:"desc"`
}

func (e sessionExecution) toSDK() zentaosdk.Execution {
	return zentaosdk.Execution{
		ID:        e.ID,
		Project:   e.Project,
		Name:      e.Name,
		Code:      e.Code,
		Type:      e.Type,
		Status:    e.Status,
		Begin:     e.Begin,
		End:       e.End,
		RealBegan: e.RealBegan,
		RealEnd:   e.RealEnd,
		Desc:      e.Desc,
	}
}

func (c *Client) getExecutionsSession(ctx context.Context, projectID int) ([]zentaosdk.Execution, error) {
	all, err := c.getAllExecutionsSession(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]zentaosdk.Execution, 0, len(all))
	for _, e := range all {
		if e.Project == projectID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (c *Client) getAllExecutionsSession(ctx context.Context) ([]zentaosdk.Execution, error) {
	var out []zentaosdk.Execution
	page := 1
	recTotal := 0
	const recPerPage = 20
	for {
		// execution-all 翻页段格式（取自执行列表页真实 pager 链接）：
		//   /execution-all-{browseType}-{orderBy}-0--{recTotal}-{recPerPage}-{pageID}.json
		// 此前 6 段格式 /execution-all-all-0--0-100-{page}.json 参数错位，
		// 实测返回的是项目形状数据（仅 1 条），导致按产品/项目查执行永远为空。
		// browseType 用 "all" 才能覆盖全部状态（受限实例默认 undone 只看进行中）。
		path := fmt.Sprintf("/execution-all-all-order_asc-0--%d-%d-%d.json", recTotal, recPerPage, page)
		var resp sessionExecutionAllResp
		// 第 2+ 页绕过缓存：各页 scope 相同，入缓存会命中第 1 页（见 doSessionJSONUncached）
		var respErr error
		if page > 1 {
			respErr = c.doSessionJSONUncached(ctx, "GetExecutions", path, &resp)
		} else {
			respErr = c.doSessionJSON(ctx, "GetExecutions", path, &resp)
		}
		if respErr != nil {
			return out, respErr
		}
		src := resp.ExecutionStats
		if len(src) == 0 {
			src = resp.Executions
		}
		if recTotal == 0 && resp.Pager != nil {
			recTotal = pagerInt(resp.Pager.RecTotal)
		}
		dupBefore := len(out)
		for _, e := range src {
			out = append(out, e.toSDK())
		}
		// 翻页未推进（同一页内容重复返回）即收口，防死循环
		if len(out) == dupBefore && page > 1 {
			return out, nil
		}
		if len(src) < recPerPage {
			return out, nil
		}
		if resp.Pager != nil && page >= pagerInt(resp.Pager.PageTotal) {
			return out, nil
		}
		page++
		if page > 2000 { // 安全闸
			return out, fmt.Errorf("execution 翻页超过 2000 页")
		}
	}
}

// sessionPager 是 *.json 端点内层 pager 字段的通用形状。
// 数值字段用 json.Number 兼容 int/string 两种返回（禅道不同实例/端点不一致）。
type sessionPager struct {
	RecTotal   json.Number `json:"recTotal"`
	RecPerPage json.Number `json:"recPerPage"`
	PageTotal  json.Number `json:"pageTotal"`
	PageID     json.Number `json:"pageID"`
	ModuleName string      `json:"moduleName"`
	MethodName string      `json:"methodName"`
}

// pagerInt 安全地把 json.Number 转 int（解析失败返回 0）。
func pagerInt(n json.Number) int {
	i, err := n.Int64()
	if err != nil {
		return 0
	}
	return int(i)
}

// executionsByProductSession 供 dashboard_service 的 GetExecutionsByProductContext 使用。
// 它先取所有项目，再按项目取 executions，组装成 ExecutionContext。
func (c *Client) executionsByProductSession(ctx context.Context, productID int) ([]ExecutionContext, error) {
	allExecs, err := c.getAllExecutionsSession(ctx)
	if err != nil {
		return nil, err
	}
	projects, err := c.getAllProjectsSession(ctx)
	if err != nil {
		return nil, err
	}
	projName := make(map[int]string, len(projects))
	for _, p := range projects {
		projName[p.ID] = p.Name
	}
	out := make([]ExecutionContext, 0, len(allExecs))
	for _, e := range allExecs {
		out = append(out, ExecutionContext{
			Exec:     e,
			ProjName: projName[e.Project],
			ExecName: e.Name,
		})
	}
	_ = productID // 会话模式下产品维度过滤由上层（bugs/stories/tasks）完成
	return out, nil
}

// strconv 辅助：从 map[string]T 里取键对应的 int 值（用于 productID 等）。
func atoiOr(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}

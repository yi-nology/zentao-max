package zentao

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/yi-nology/common/biz/zentao"
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

func (p sessionProduct) toSDK() zentao.Product {
	return zentao.Product{
		ID:     p.ID,
		Name:   p.Name,
		Code:   p.Code,
		Type:   p.Type,
		Status: p.Status,
		Desc:   p.Desc,
	}
}

// getProductsSession 会话模式获取产品列表。
// /product-all.json 单页即返回全部产品（实测受限实例 ~445 个产品，无翻页）。
// 若有翻页则按 pager.pageTotal 循环。
func (c *Client) getProductsSession(ctx context.Context) ([]zentao.Product, error) {
	var out []zentao.Product
	page := 1
	for {
		var path string
		if page == 1 {
			path = "/product-all.json"
		} else {
			// product-all 翻页段格式（保守）：后缀页码。
			path = fmt.Sprintf("/product-all-%d.json", page)
		}
		var resp sessionProductAllResp
		if err := c.doSessionJSON(ctx, "GetProducts", path, &resp); err != nil {
			return out, err
		}
		for _, p := range resp.ProductStats {
			out = append(out, p.toSDK())
		}
		// product-all 通常无翻页（单页返回全部），recPerPage 缺失即认为完整。
		if resp.Pager == nil || page >= pagerInt(resp.Pager.PageTotal) || len(resp.ProductStats) == 0 {
			return out, nil
		}
		page++
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

func (p sessionProject) toSDK() zentao.Project {
	return zentao.Project{
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

func (c *Client) getProjectsByProductSession(ctx context.Context, productID int) ([]zentao.Project, error) {
	if productID <= 0 {
		return c.getAllProjectsSession(ctx)
	}

	// 兜底：bug 反推
	all, err := c.getAllProjectsRaw(ctx)
	if err != nil {
		return nil, err
	}
	allSDK := make([]zentao.Project, 0, len(all))
	byID := make(map[int]zentao.Project, len(all))
	for _, p := range all {
		sdk := p.toSDK()
		allSDK = append(allSDK, sdk)
		byID[sdk.ID] = sdk
	}

	linkedProjectIDs, err := c.fetchProductProjectIDsViaBugs(ctx, productID)
	if err != nil {
		return []zentao.Project{}, nil
	}
	out := make([]zentao.Project, 0, len(linkedProjectIDs))
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

func (c *Client) getAllProjectsSession(ctx context.Context) ([]zentao.Project, error) {
	raw, err := c.getAllProjectsRaw(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]zentao.Project, 0, len(raw))
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
		if err := c.doSessionJSON(ctx, "GetAllProjects", path, &resp); err != nil {
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

func (c *Client) getProjectSession(ctx context.Context, projectID int) (*zentao.Project, error) {
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

func (e sessionExecution) toSDK() zentao.Execution {
	return zentao.Execution{
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

func (c *Client) getExecutionsSession(ctx context.Context, projectID int) ([]zentao.Execution, error) {
	all, err := c.getAllExecutionsSession(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]zentao.Execution, 0, len(all))
	for _, e := range all {
		if e.Project == projectID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (c *Client) getAllExecutionsSession(ctx context.Context) ([]zentao.Execution, error) {
	var out []zentao.Execution
	page := 1
	recTotal := 0
	const recPerPage = 100
	for {
		// execution-all 的分页 URL 格式（来自页面 pager link-creator）：
		//   /execution-all-undone-{orderBy}-0--{recTotal}-{recPerPage}-{page}.json
		// 第一页用裸路径 /execution-all.json 拿到真实 recTotal，后续页带完整段。
		// browseType 用 "all" 才能覆盖全部状态（受限实例默认 undone 只看进行中）。
		var path string
		if page == 1 {
			path = "/execution-all-all-0--0-100-1.json"
		} else {
			path = fmt.Sprintf("/execution-all-all-0--%d-%d-%d.json", recTotal, recPerPage, page)
		}
		var resp sessionExecutionAllResp
		if err := c.doSessionJSON(ctx, "GetExecutions", path, &resp); err != nil {
			return out, err
		}
		src := resp.ExecutionStats
		if len(src) == 0 {
			src = resp.Executions
		}
		if recTotal == 0 && resp.Pager != nil {
			recTotal = pagerInt(resp.Pager.RecTotal)
		}
		for _, e := range src {
			out = append(out, e.toSDK())
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

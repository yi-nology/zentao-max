package zentao

import (
	"context"

	zentaosdk "github.com/yi-nology/zentao-max/backend/core/zentao/sdk"
)

// GetBugs 获取产品的 Bug 列表
func (c *Client) GetBugs(productID int, page, pageSize int) ([]zentaosdk.Bug, error) {
	return c.getBugsSession(context.Background(), productID, page, pageSize)
}

// GetBugsByProject 根据项目 ID 过滤 Bug 列表
func (c *Client) GetBugsByProject(productID, projectID int, page, pageSize int) ([]zentaosdk.Bug, error) {
	return c.getBugsByProjectSession(context.Background(), productID, projectID, page, pageSize)
}

// GetBugsByStatus 根据状态过滤 Bug 列表
func (c *Client) GetBugsByStatus(productID int, status string, page, pageSize int) ([]zentaosdk.Bug, error) {
	all, err := c.getAllBugsSession(context.Background(), productID)
	if err != nil {
		return nil, err
	}
	filtered := make([]zentaosdk.Bug, 0)
	for _, b := range all {
		if b.Status == status {
			filtered = append(filtered, b)
		}
	}
	return paginateBugs(filtered, page, pageSize), nil
}

// SearchBugs 搜索 Bug（支持多条件过滤）
func (c *Client) SearchBugs(params zentaosdk.BugSearchParams) ([]zentaosdk.Bug, error) {
	return c.searchBugsSession(context.Background(), params)
}

// GetAllBugs 获取产品全部 Bug（自动翻页，含 closed / resolved 等所有状态）。
//
// 禅道 session 端点 /bug-browse-{pid}-0-{browseType}-... 的 browseType 实际语义：
//   - "unclosed"：只看未关闭（默认）
//   - "all"：全状态（含 closed / resolved）—— 本方法走这个
//   - "active"：激活态
//
// 注意：禅道不走 status query 参数，browseType 是 URL 段之一。
// 旧版 "GetAllBugsIncludeClosed" 跟本方法完全等价（都走 browseType=all），
// 已删除以免误导。
func (c *Client) GetAllBugs(productID int) ([]zentaosdk.Bug, error) {
	return c.getAllBugsSession(context.Background(), productID)
}

// GetAllBugsByProject 获取项目全部 Bug
func (c *Client) GetAllBugsByProject(projectID int) ([]zentaosdk.Bug, error) {
	return c.GetAllBugsByProjectWithProduct(0, projectID)
}

// GetAllBugsByProjectWithProduct 获取项目全部 Bug（指定产品ID）
// 兼容 projectID=0 的语义：只按产品过滤，不按项目过滤。
// 之前 session 模式漏处理 projectID=0，导致 b.Project==0 永远不成立、
// 全部 bug 被过滤掉（preview bug / bug-aging 报 0）。
func (c *Client) GetAllBugsByProjectWithProduct(productID int, projectID int) ([]zentaosdk.Bug, error) {
	if productID > 0 {
		all, err := c.getAllBugsSession(context.Background(), productID)
		if err != nil {
			return nil, err
		}
		if projectID <= 0 {
			return all, nil
		}
		out := make([]zentaosdk.Bug, 0, len(all)/4)
		for _, b := range all {
			if b.Project == projectID {
				out = append(out, b)
			}
		}
		return out, nil
	}
	return c.getAllBugsByProjectSession(context.Background(), projectID)
}

// GetBugsContext 获取 Bug 列表（支持 context 取消）
func (c *Client) GetBugsContext(ctx context.Context, productID int, page, pageSize int) ([]zentaosdk.Bug, error) {
	return c.getBugsSession(ctx, productID, page, pageSize)
}

// GetAllBugsContext 获取产品全部 Bug（支持 context 取消，自动翻页）
func (c *Client) GetAllBugsContext(ctx context.Context, productID int) ([]zentaosdk.Bug, error) {
	return c.getAllBugsSession(ctx, productID)
}

// GetAllBugsByProjectContext 获取项目全部 Bug（支持 context 取消）
func (c *Client) GetAllBugsByProjectContext(ctx context.Context, projectID int) ([]zentaosdk.Bug, error) {
	return c.getAllBugsByProjectSession(ctx, projectID)
}

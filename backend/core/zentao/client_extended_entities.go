package zentao

import (
	"context"
	"fmt"

	"github.com/yi-nology/common/biz/zentao"
)

// 本文件为 SDK 已支持但 wrapper 此前未暴露的 5 个实体补齐读取方法。
// 每个方法都做 token/session 双模式分发，签名与 SDK 对齐。

// ---- Cases ----

// GetCasesByProduct 拉取产品下的用例单页，并返回真实总数。
// 实现: 走 v1 REST API (/api.php/v1/testcases?product=&page=&limit=), 用 cookie 鉴权.
// 为什么不用 session 的 /testcase-browse-{pid}.json: 受限实例的 testcase-browse 端点
// 不支持 server-side 翻页 (分页段 URL 返空), 翻页必须用 v1 路径.
//
// 第二个返回值 recTotal 让 service 层能正确填充 PaginatedVO.Total，避免前端分页 total 永远是单页长度。
func (c *Client) GetCasesByProduct(productID, page, pageSize int) ([]zentao.Case, int, error) {
	if c.session == nil {
		return nil, 0, fmt.Errorf("会话客户端未初始化")
	}
	return c.getCasesByProductV1(context.Background(), productID, page, pageSize)
}

func (c *Client) GetAllCases(productID int) ([]zentao.Case, error) {
	if c.session == nil {
		return nil, fmt.Errorf("会话客户端未初始化")
	}
	// v1 limit 有上限 (受限实例 ≤ 20), 全量模式无意义, 保持单页能力, 调用方按需翻页.
	out, _, err := c.getCasesByProductV1(context.Background(), productID, 1, 20)
	return out, err
}

// ---- Plans ----

func (c *Client) GetPlans(productID, page, pageSize int) ([]zentao.Plan, error) {
	return c.getPlansSession(context.Background(), productID, page, pageSize)
}

// ---- Programs ----

func (c *Client) GetPrograms(page, pageSize int) ([]zentao.Program, error) {
	return c.getProgramsSession(context.Background(), page, pageSize)
}

// ---- Releases ----

func (c *Client) GetReleasesByProduct(productID, page, pageSize int) ([]zentao.Release, error) {
	return c.getReleasesByProductSession(context.Background(), productID, page, pageSize)
}

// ---- Test Tasks ----

func (c *Client) GetTestTasksByProduct(productID, page, pageSize int) ([]zentao.TestTask, error) {
	return c.getTestTasksSession(context.Background(), productID, page, pageSize)
}

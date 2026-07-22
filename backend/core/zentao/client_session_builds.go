package zentao

import (
	"context"
	"fmt"

	zentaosdk "github.com/yi-nology/zentao-max/backend/core/zentao/sdk"
)

// ---- Builds (session mode) ----
//
// 受限实例上 build 模块需要单独权限，普通账号通常拿不到（探测时返回空或 user-deny）。
// 这里实现尽力而为：权限允许时返回数据，拒绝时返回空切片（不报错），让 builds 页面
// 在 your-zentao-server.com 上优雅降级而不是 500。
//
// 端点：/build-{executionID}.json（按 execution 取 build）。
// project 维度的 build 端点不统一（projectbuild 需要额外权限），这里用 execution 反查：
// 先取 project 的 executions，再合并每个 execution 的 build。

type sessionBuildResp struct {
	Title  string         `json:"title"`
	Builds []sessionBuild `json:"builds"`
	Pager  *sessionPager  `json:"pager"`
}

type sessionBuild struct {
	ID        int    `json:"id"`
	Product   int    `json:"product"`
	Project   int    `json:"project"`
	Execution int    `json:"execution"`
	Name      string `json:"name"`
	ScmPath   string `json:"scmPath"`
	FilePath  string `json:"filePath"`
	Date      string `json:"date"`
	Stories   string `json:"stories"`
	Bugs      string `json:"bugs"`
	Builder   string `json:"builder"`
	Desc      string `json:"desc"`
	Deleted   string `json:"deleted"`
}

func (b sessionBuild) toSDK() zentaosdk.Build {
	return zentaosdk.Build{
		ID:        b.ID,
		Product:   b.Product,
		Project:   b.Project,
		Execution: b.Execution,
		Name:      b.Name,
		ScmPath:   b.ScmPath,
		FilePath:  b.FilePath,
		Date:      b.Date,
		Stories:   b.Stories,
		Bugs:      b.Bugs,
		Builder:   b.Builder,
		Desc:      b.Desc,
		Deleted:   b.Deleted,
	}
}

// fetchBuildsByExecutionSession 取单个 execution 的全部 build。
// 权限拒绝（user-deny）时返回空切片而非错误，便于上层优雅降级。
func (c *Client) fetchBuildsByExecutionSession(ctx context.Context, executionID int) ([]zentaosdk.Build, error) {
	path := fmt.Sprintf("/build-%d.json", executionID)
	var resp sessionBuildResp
	if err := c.doSessionJSON(ctx, "GetBuildsByExecution", path, &resp); err != nil {
		if IsSessionAccessDenied(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]zentaosdk.Build, 0, len(resp.Builds))
	for _, b := range resp.Builds {
		out = append(out, b.toSDK())
	}
	return out, nil
}

func (c *Client) getBuildsByExecutionSession(ctx context.Context, executionID, _, _ int) ([]zentaosdk.Build, error) {
	return c.fetchBuildsByExecutionSession(ctx, executionID)
}

func (c *Client) getBuildsByProjectSession(ctx context.Context, projectID, page, pageSize int) ([]zentaosdk.Build, error) {
	// 先取 project 的 executions，再合并每个 execution 的 build（去重 by ID）。
	execs, err := c.getExecutionsSession(ctx, projectID)
	if err != nil {
		return nil, err
	}
	seen := make(map[int]bool)
	var all []zentaosdk.Build
	for _, e := range execs {
		bs, berr := c.fetchBuildsByExecutionSession(ctx, e.ID)
		if berr != nil {
			continue
		}
		for _, b := range bs {
			if !seen[b.ID] {
				seen[b.ID] = true
				all = append(all, b)
			}
		}
	}
	return all, nil
}

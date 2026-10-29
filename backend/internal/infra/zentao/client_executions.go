package zentao

import (
	"context"

	zentaosdk "github.com/yi-nology/zentao-max/backend/internal/infra/zentao/sdk"
)

// GetExecutions 获取执行列表
func (c *Client) GetExecutions(projectID int, page, pageSize int) ([]zentaosdk.Execution, error) {
	all, err := c.getExecutionsSession(context.Background(), projectID)
	if err != nil {
		return nil, err
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

// GetAllExecutions 获取实例全部执行（execution-all 分页遍历，无项目/产品过滤）。
// 供 ExecutionService 无过滤分支使用——此前该分支直接返回空列表，
// 导致 REST /api/executions 与 MCP get_executions 不带参数时永远空数组。
func (c *Client) GetAllExecutions() ([]zentaosdk.Execution, error) {
	return c.getAllExecutionsSession(context.Background())
}

// GetExecutionsContext 获取执行列表（支持 context 取消）
func (c *Client) GetExecutionsContext(ctx context.Context, projectID int, page, pageSize int) ([]zentaosdk.Execution, error) {
	all, err := c.getExecutionsSession(ctx, projectID)
	if err != nil {
		return nil, err
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

// ExecutionContext 执行上下文（包含执行信息和关联的项目）
type ExecutionContext struct {
	Exec     zentaosdk.Execution
	ProjName string
	ExecName string
}

// GetExecutionsByProduct 按产品获取所有执行
func (c *Client) GetExecutionsByProduct(productID int) ([]ExecutionContext, error) {
	var result []ExecutionContext
	projects, err := c.GetProjectsByProduct(productID, 1, 200)
	if err != nil {
		return nil, err
	}
	for _, p := range projects {
		executions, err := c.GetExecutions(p.ID, 1, 200)
		if err != nil {
			continue
		}
		for _, e := range executions {
			result = append(result, ExecutionContext{
				Exec:     e,
				ProjName: p.Name,
				ExecName: e.Name,
			})
		}
	}
	return result, nil
}

// GetAllExecutionsContext 全执行列表（纯 Execution，不组装项目名映射）。
// executionsByProductSession 会额外做全项目翻页（万级项目 ~60s）来填 ProjName，
// 任务/工时等只需执行 ID+名称的遍历场景应走本方法，避免该成本。
func (c *Client) GetAllExecutionsContext(ctx context.Context) ([]zentaosdk.Execution, error) {
	return c.getAllExecutionsSession(ctx)
}

// GetExecutionsByProductContext 按产品获取所有执行（支持 context 取消）
func (c *Client) GetExecutionsByProductContext(ctx context.Context, productID int) ([]ExecutionContext, error) {
	return c.executionsByProductSession(ctx, productID)
}

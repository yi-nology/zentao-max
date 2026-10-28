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

// GetExecutionsByProductContext 按产品获取所有执行（支持 context 取消）
func (c *Client) GetExecutionsByProductContext(ctx context.Context, productID int) ([]ExecutionContext, error) {
	return c.executionsByProductSession(ctx, productID)
}

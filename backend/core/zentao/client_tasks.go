package zentao

import (
	"context"

	zentaosdk "github.com/yi-nology/zentao-max/backend/core/zentao/sdk"
)

// GetTasks 获取执行的任务列表
func (c *Client) GetTasks(executionID int, page, pageSize int) ([]zentaosdk.Task, error) {
	return c.getTasksSession(context.Background(), executionID, page, pageSize)
}

// GetAllTasksByProject 获取项目全部任务（自动翻页）
func (c *Client) GetAllTasksByProject(projectID int) ([]zentaosdk.Task, error) {
	return c.getAllTasksByProjectSession(context.Background(), projectID)
}

// GetAllTasksByProduct 获取产品全部任务（自动翻页）
func (c *Client) GetAllTasksByProduct(productID int) ([]zentaosdk.Task, error) {
	return c.getAllTasksByProductSession(context.Background(), productID)
}

// GetTasksContext 获取执行的任务列表（支持 context 取消）
func (c *Client) GetTasksContext(ctx context.Context, executionID int, page, pageSize int) ([]zentaosdk.Task, error) {
	return c.getTasksSession(ctx, executionID, page, pageSize)
}

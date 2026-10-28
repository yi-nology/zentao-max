package zentao

import (
	"context"

	zentaosdk "github.com/yi-nology/zentao-max/backend/internal/infra/zentao/sdk"
)

// GetStoriesByProduct 获取产品的需求列表
func (c *Client) GetStoriesByProduct(productID int, page, pageSize int) ([]zentaosdk.Story, error) {
	return c.getStoriesByProductSession(context.Background(), productID, page, pageSize)
}

// GetStoriesByProject 获取项目的需求列表
func (c *Client) GetStoriesByProject(projectID int, page, pageSize int) ([]zentaosdk.Story, error) {
	return c.getStoriesByProjectSession(context.Background(), projectID, page, pageSize)
}

// GetStoriesByExecution 获取执行的需求列表
func (c *Client) GetStoriesByExecution(executionID int, page, pageSize int) ([]zentaosdk.Story, error) {
	return c.getStoriesByExecutionSession(context.Background(), executionID, page, pageSize)
}

// GetAllStories 获取产品全部需求（自动翻页）
func (c *Client) GetAllStories(productID int) ([]zentaosdk.Story, error) {
	return c.getAllStoriesSession(context.Background(), productID)
}

// GetAllStoriesByProject 获取项目全部需求（自动翻页）
func (c *Client) GetAllStoriesByProject(projectID int) ([]zentaosdk.Story, error) {
	return c.getAllStoriesByProjectSession(context.Background(), projectID)
}

// GetStoriesByProductContext 获取产品的需求列表（支持 context 取消）
func (c *Client) GetStoriesByProductContext(ctx context.Context, productID int, page, pageSize int) ([]zentaosdk.Story, error) {
	return c.getStoriesByProductSession(ctx, productID, page, pageSize)
}

// GetAllStoriesContext 获取产品全部需求（支持 context 取消，自动翻页）
func (c *Client) GetAllStoriesContext(ctx context.Context, productID int) ([]zentaosdk.Story, error) {
	return c.getAllStoriesSession(ctx, productID)
}

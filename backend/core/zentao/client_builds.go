package zentao

import (
	"context"

	"github.com/yi-nology/common/biz/zentao"
)

// GetBuildsByProject 获取项目下的版本列表
func (c *Client) GetBuildsByProject(projectID int, page, pageSize int) ([]zentao.Build, error) {
	return c.getBuildsByProjectSession(context.Background(), projectID, page, pageSize)
}

// GetBuildsByExecution 获取执行下的版本列表
func (c *Client) GetBuildsByExecution(executionID int, page, pageSize int) ([]zentao.Build, error) {
	return c.getBuildsByExecutionSession(context.Background(), executionID, page, pageSize)
}

package zentao

import (
	"context"

	"github.com/yi-nology/common/biz/zentao"
)

// GetAllProjects 获取所有项目列表
func (c *Client) GetAllProjects(limit int) ([]zentao.Project, error) {
	all, err := c.getAllProjectsSession(context.Background())
	if err != nil {
		return nil, err
	}
	if limit > 0 && limit < len(all) {
		return all[:limit], nil
	}
	return all, nil
}

// GetProjectsByProduct 获取产品关联的项目列表
func (c *Client) GetProjectsByProduct(productID int, page, pageSize int) ([]zentao.Project, error) {
	all, err := c.getProjectsByProductSession(context.Background(), productID)
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

// GetProjectContext 获取项目详情（支持 context 取消）
func (c *Client) GetProjectContext(ctx context.Context, projectID int) (*zentao.Project, error) {
	return c.getProjectSession(ctx, projectID)
}

// GetProjectsByProductContext 获取产品关联的项目（支持 context 取消）
func (c *Client) GetProjectsByProductContext(ctx context.Context, productID int, page, pageSize int) ([]zentao.Project, error) {
	all, err := c.getProjectsByProductSession(ctx, productID)
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

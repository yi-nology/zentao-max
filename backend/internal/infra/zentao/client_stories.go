package zentao

import (
	"context"
	"encoding/json"
	"fmt"

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

// GetStoryByID 按 ID 拉取需求详情（/story-view-{id}.json 双层包裹，内层通常为
// {"story": {...}}；部分实例直接返回需求对象，两种形状都兼容）。
func (c *Client) GetStoryByID(ctx context.Context, storyID int) (*zentaosdk.Story, error) {
	if c.session == nil {
		return nil, fmt.Errorf("会话客户端未初始化")
	}
	var inner map[string]json.RawMessage
	if err := c.doSessionJSON(ctx, "GetStoryByID", fmt.Sprintf("/story-view-%d.json", storyID), &inner); err != nil {
		// 受限实例对无权限需求返回内层 {"result":"fail","message":"抱歉..."}，
		// 外层解包在该形态下报解析错误——转成可读的权限提示
		return nil, fmt.Errorf("需求 %d 详情获取失败（可能无权限访问该需求，或需求已删除）: %w", storyID, err)
	}
	raw, ok := inner["story"]
	if !ok {
		// 内层直接是需求对象（无 "story" 包裹键）：整包回编后解码
		if _, hasID := inner["id"]; !hasID {
			return nil, fmt.Errorf("需求 %d 不存在或无权限查看", storyID)
		}
		whole, merr := json.Marshal(inner)
		if merr != nil {
			return nil, merr
		}
		raw = whole
	}
	var st zentaosdk.Story
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, err
	}
	if st.ID == 0 {
		return nil, fmt.Errorf("需求 %d 不存在或无权限查看", storyID)
	}
	return &st, nil
}

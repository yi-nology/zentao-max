package zentao

import (
	"context"
	"encoding/json"
	"fmt"

	zentaosdk "github.com/yi-nology/zentao-max/backend/internal/infra/zentao/sdk"
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

// GetTaskByID 按 ID 拉取任务详情（/task-view-{id}.json 双层包裹，内层通常为
// {"task": {...}}；部分实例直接返回任务对象，两种形状都兼容）。
func (c *Client) GetTaskByID(ctx context.Context, taskID int) (*zentaosdk.Task, error) {
	if c.session == nil {
		return nil, fmt.Errorf("会话客户端未初始化")
	}
	var inner map[string]json.RawMessage
	if err := c.doSessionJSON(ctx, "GetTaskByID", fmt.Sprintf("/task-view-%d.json", taskID), &inner); err != nil {
		return nil, err
	}
	raw, ok := inner["task"]
	if !ok {
		// 内层直接是任务对象（无 "task" 包裹键）：整包回编后解码
		if _, hasID := inner["id"]; !hasID {
			return nil, fmt.Errorf("任务 %d 不存在或无权限查看", taskID)
		}
		whole, merr := json.Marshal(inner)
		if merr != nil {
			return nil, merr
		}
		raw = whole
	}
	var t zentaosdk.Task
	if err := json.Unmarshal(raw, &t); err != nil {
		return nil, err
	}
	if t.ID == 0 {
		return nil, fmt.Errorf("任务 %d 不存在或无权限查看", taskID)
	}
	return &t, nil
}

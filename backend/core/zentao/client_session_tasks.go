package zentao

import (
	"context"
	"fmt"

	zentaosdk "github.com/yi-nology/zentao-max/backend/core/zentao/sdk"
)

// ---- Tasks (session mode) ----
//
// 端点：/execution-task-{executionID}.json
// 内层 tasks 是 map[id]task，按 execution 取。单页即全部（受限实例 exec 4 有 97 条）。

type sessionExecutionTaskResp struct {
	Title       string                 `json:"title"`
	ExecutionID int                    `json:"executionID"`
	Tasks       map[string]sessionTask `json:"tasks"`
	HasTasks    bool                   `json:"hasTasks"`
	Pager       *sessionPager          `json:"pager"`
}

type sessionTask struct {
	ID           int     `json:"id"`
	Project      int     `json:"project"`
	Parent       int     `json:"parent"`
	Execution    int     `json:"execution"`
	Module       int     `json:"module"`
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Pri          int     `json:"pri"`
	Estimate     float64 `json:"estimate"`
	Consumed     float64 `json:"consumed"`
	Left         float64 `json:"left"`
	Deadline     string  `json:"deadline"`
	Status       string  `json:"status"`
	OpenedBy     string  `json:"openedBy"`
	OpenedDate   string  `json:"openedDate"`
	AssignedTo   string  `json:"assignedTo"`
	AssignedDate string  `json:"assignedDate"`
	EstStarted   string  `json:"estStarted"`
	RealStarted  string  `json:"realStarted"`
	FinishedBy   string  `json:"finishedBy"`
	FinishedDate string  `json:"finishedDate"`
	ClosedBy     string  `json:"closedBy"`
	ClosedDate   string  `json:"closedDate"`
	Story        int     `json:"story"`
	Deleted      string  `json:"deleted"`
}

func (t sessionTask) toSDK() zentaosdk.Task {
	return zentaosdk.Task{
		ID:           t.ID,
		Project:      t.Project,
		Execution:    t.Execution,
		Name:         t.Name,
		Type:         t.Type,
		Pri:          t.Pri,
		Status:       t.Status,
		AssignedTo:   t.AssignedTo,
		EstStarted:   t.EstStarted,
		Deadline:     t.Deadline,
		Estimate:     t.Estimate,
		Consumed:     t.Consumed,
		Left:         t.Left,
		OpenedBy:     t.OpenedBy,
		OpenedDate:   t.OpenedDate,
		FinishedBy:   t.FinishedBy,
		FinishedDate: t.FinishedDate,
		ClosedBy:     t.ClosedBy,
		ClosedDate:   t.ClosedDate,
		StatusName:   t.Status,
	}
}

func (c *Client) getTasksSession(ctx context.Context, executionID, page, pageSize int) ([]zentaosdk.Task, error) {
	path := fmt.Sprintf("/execution-task-%d.json", executionID)
	var resp sessionExecutionTaskResp
	if err := c.doSessionJSON(ctx, "GetTasks", path, &resp); err != nil {
		return nil, err
	}
	all := make([]zentaosdk.Task, 0, len(resp.Tasks))
	for _, t := range resp.Tasks {
		all = append(all, t.toSDK())
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

func (c *Client) getAllTasksByProjectSession(ctx context.Context, projectID int) ([]zentaosdk.Task, error) {
	execs, err := c.getExecutionsSession(ctx, projectID)
	if err != nil {
		return nil, err
	}
	var out []zentaosdk.Task
	for _, e := range execs {
		ts, terr := c.getTasksSession(ctx, e.ID, 1, 100000)
		if terr != nil {
			continue
		}
		out = append(out, ts...)
	}
	return out, nil
}

func (c *Client) getAllTasksByProductSession(ctx context.Context, productID int) ([]zentaosdk.Task, error) {
	projects, err := c.getProjectsByProductSession(ctx, productID)
	if err != nil {
		return nil, err
	}
	var out []zentaosdk.Task
	for _, p := range projects {
		ts, terr := c.getAllTasksByProjectSession(ctx, p.ID)
		if terr != nil {
			continue
		}
		out = append(out, ts...)
	}
	return out, nil
}

// sessionMyTaskResp 是 /my-task-{status}.json 端点的响应。
// 受限实例 / 标准 zentao 都支持（不依赖 execution）。
type sessionMyTaskResp struct {
	Title    string                 `json:"title"`
	Tasks    map[string]sessionTask `json:"tasks"`
	HasTasks bool                   `json:"hasTasks"`
}

// ListMyTasks 拉当前登录用户的 task 列表（不依赖 execution/product）。
//
// path 形式：/my-task-{status}.json，status ∈ all / wait / doing / done / closed。
// 用于 daily note 同步时找/确认 special "📓 Daily Notes" task。
//
// 失败模式：受限实例限流时返 0 字节或 wrong shape，调用方应 fallback 到本地 SQLite。
func (c *Client) ListMyTasks(ctx context.Context, status string) ([]zentaosdk.Task, error) {
	if c.session == nil {
		return nil, fmt.Errorf("会话客户端未初始化")
	}
	if status == "" {
		status = "all"
	}
	path := fmt.Sprintf("/my-task-%s.json", status)
	var resp sessionMyTaskResp
	if err := c.doSessionJSON(ctx, "ListMyTasks", path, &resp); err != nil {
		return nil, err
	}
	out := make([]zentaosdk.Task, 0, len(resp.Tasks))
	for _, t := range resp.Tasks {
		if t.Deleted == "1" {
			continue
		}
		out = append(out, t.toSDK())
	}
	return out, nil
}

// GetMyWorkTasks 拉当前登录用户被指派的任务（/my-work-task.json，tasks 为数组形状）。
//
// 与 ListMyTasks（/my-task-{status}.json，biz8.x 实测返回空）不同，
// my-work-task 是禅道首页"待处理→任务"的数据源，实测可用。
// 走 uncached：path 无数字段，cache scope 与其它 tasks 查询相同，入缓存会互相污染。
func (c *Client) GetMyWorkTasks(ctx context.Context) ([]zentaosdk.Task, error) {
	if c.session == nil {
		return nil, fmt.Errorf("会话客户端未初始化")
	}
	var resp struct {
		Tasks []sessionTask `json:"tasks"`
	}
	if err := c.doSessionJSONUncached(ctx, "GetMyWorkTasks", "/my-work-task.json", &resp); err != nil {
		return nil, err
	}
	out := make([]zentaosdk.Task, 0, len(resp.Tasks))
	for _, t := range resp.Tasks {
		if t.Deleted == "1" {
			continue
		}
		out = append(out, t.toSDK())
	}
	return out, nil
}

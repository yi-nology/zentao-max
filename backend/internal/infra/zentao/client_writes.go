package zentao

import (
	"context"
	"fmt"
	neturl "net/url"

	zentaosdk "github.com/yi-nology/zentao-max/backend/internal/infra/zentao/sdk"
)

// 本文件为 wrapper 补齐写操作（session 模式）。
// 通过 doSessionPost 走 PHP 表单端点（/{module}-{action}-{id}.json）。

// ---- Bug writes ----

func (c *Client) ResolveBug(bugID int, req zentaosdk.BugResolveRequest) error {
	form := neturl.Values{}
	form.Set("resolution", req.Resolution)
	form.Set("resolvedBuild", req.ResolvedBuild)
	if req.Comment != "" {
		form.Set("comment", req.Comment)
	}
	return c.doSessionPost(context.Background(), "ResolveBug",
		fmt.Sprintf("/bug-resolve-%d.json", bugID), form)
}

func (c *Client) CloseBug(bugID int, req zentaosdk.BugCloseRequest) error {
	form := neturl.Values{}
	if req.Comment != "" {
		form.Set("comment", req.Comment)
	}
	return c.doSessionPost(context.Background(), "CloseBug",
		fmt.Sprintf("/bug-close-%d.json", bugID), form)
}

func (c *Client) AssignBug(bugID int, req zentaosdk.BugAssignRequest) error {
	form := neturl.Values{}
	form.Set("assignedTo", req.AssignedTo)
	if req.Comment != "" {
		form.Set("comment", req.Comment)
	}
	return c.doSessionPost(context.Background(), "AssignBug",
		fmt.Sprintf("/bug-assignTo-%d.json", bugID), form)
}

func (c *Client) ConfirmBug(bugID int, req zentaosdk.BugConfirmRequest) error {
	form := neturl.Values{}
	if req.Comment != "" {
		form.Set("comment", req.Comment)
	}
	return c.doSessionPost(context.Background(), "ConfirmBug",
		fmt.Sprintf("/bug-confirm-%d.json", bugID), form)
}

func (c *Client) ActivateBug(bugID int, req zentaosdk.BugActivateRequest) error {
	form := neturl.Values{}
	if req.AssignedTo != "" {
		form.Set("assignedTo", req.AssignedTo)
	}
	if req.Comment != "" {
		form.Set("comment", req.Comment)
	}
	return c.doSessionPost(context.Background(), "ActivateBug",
		fmt.Sprintf("/bug-activate-%d.json", bugID), form)
}

// ---- Task writes ----

func (c *Client) StartTask(taskID int, req zentaosdk.TaskStartRequest) error {
	form := neturl.Values{}
	if req.RealStarted != "" {
		form.Set("realStarted", req.RealStarted)
	}
	form.Set("consumed", fmt.Sprintf("%g", req.Consumed))
	form.Set("left", fmt.Sprintf("%g", req.Left))
	if req.Comment != "" {
		form.Set("comment", req.Comment)
	}
	return c.doSessionPost(context.Background(), "StartTask",
		fmt.Sprintf("/task-start-%d.json", taskID), form)
}

func (c *Client) FinishTask(taskID int, req zentaosdk.TaskFinishRequest) error {
	form := neturl.Values{}
	form.Set("consumed", fmt.Sprintf("%g", req.Consumed))
	if req.FinishedDate != "" {
		form.Set("finishedDate", req.FinishedDate)
	}
	if req.Comment != "" {
		form.Set("comment", req.Comment)
	}
	return c.doSessionPost(context.Background(), "FinishTask",
		fmt.Sprintf("/task-finish-%d.json", taskID), form)
}

func (c *Client) PauseTask(taskID int, req zentaosdk.TaskPauseRequest) error {
	form := neturl.Values{}
	if req.Comment != "" {
		form.Set("comment", req.Comment)
	}
	return c.doSessionPost(context.Background(), "PauseTask",
		fmt.Sprintf("/task-pause-%d.json", taskID), form)
}

func (c *Client) AssignTask(taskID int, req zentaosdk.TaskAssignRequest) error {
	form := neturl.Values{}
	form.Set("assignedTo", req.AssignedTo)
	form.Set("left", fmt.Sprintf("%g", req.Left))
	if req.Comment != "" {
		form.Set("comment", req.Comment)
	}
	return c.doSessionPost(context.Background(), "AssignTask",
		fmt.Sprintf("/task-assignedTo-%d.json", taskID), form)
}

// ---- Story writes ----

func (c *Client) ChangeStory(storyID int, spec, verify string) error {
	form := neturl.Values{}
	form.Set("spec", spec)
	form.Set("verify", verify)
	return c.doSessionPost(context.Background(), "ChangeStory",
		fmt.Sprintf("/story-change-%d.json", storyID), form)
}

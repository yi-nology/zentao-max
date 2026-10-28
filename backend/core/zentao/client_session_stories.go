package zentao

import (
	"context"
	"encoding/json"
	"fmt"

	zentaosdk "github.com/yi-nology/zentao-max/backend/core/zentao/sdk"
)

// ---- Stories (session mode) ----
//
// 端点：/product-browse-{productID}-story.json
// 注意：/story-browse-{pid}.json 在受限实例上返回 user-deny（权限不足），
// 实际可用的是 product-browse-{pid}-story。响应里 stories 是 map[id]story。

type sessionProductStoryResp struct {
	Title          string          `json:"title"`
	Stories        sessionStoryMap `json:"stories"`
	StoryStages    interface{}     `json:"storyStages"`
	IsProjectStory interface{}     `json:"isProjectStory"`
}

// sessionStoryMap 兼容 stories 的 map/array 两种返回形状
// （部分执行的 /execution-story-{id}.json 返回数组而非 map，实测）。
type sessionStoryMap map[string]sessionStory

func (m *sessionStoryMap) UnmarshalJSON(data []byte) error {
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err == nil {
		*m = sessionStoryMap{}
		return nil
	}
	return json.Unmarshal(data, (*map[string]sessionStory)(m))
}

type sessionStory struct {
	ID         int     `json:"id"`
	Product    int     `json:"product"`
	Module     int     `json:"module"`
	Plan       string  `json:"plan"`
	Source     string  `json:"source"`
	SourceNote string  `json:"sourceNote"`
	Title      string  `json:"title"`
	Type       string  `json:"type"`
	Category   string  `json:"category"`
	Status     string  `json:"status"`
	Stage      string  `json:"stage"`
	Pri        int     `json:"pri"`
	Estimate   float64 `json:"estimate"`
	OpenedBy   string  `json:"openedBy"`
	OpenedDate string  `json:"openedDate"`
	AssignedTo string  `json:"assignedTo"`
	ClosedBy   string  `json:"closedBy"`
	ClosedDate string  `json:"closedDate"`
	Parent     int     `json:"parent"`
	Vision     string  `json:"vision"`
	Deleted    string  `json:"deleted"`
}

func (s sessionStory) toSDK() zentaosdk.Story {
	return zentaosdk.Story{
		ID:         s.ID,
		Product:    s.Product,
		Module:     s.Module,
		Plan:       s.Plan,
		Source:     s.Source,
		SourceNote: s.SourceNote,
		Title:      s.Title,
		Type:       s.Type,
		Status:     s.Status,
		Stage:      s.Stage,
		Pri:        s.Pri,
		Estimate:   s.Estimate,
		OpenedBy:   s.OpenedBy,
		OpenedDate: s.OpenedDate,
		AssignedTo: s.AssignedTo,
		ClosedBy:   s.ClosedBy,
		ClosedDate: s.ClosedDate,
		Parent:     s.Parent,
		Vision:     s.Vision,
		Deleted:    s.Deleted,
	}
}

func (c *Client) getStoriesByProductSession(ctx context.Context, productID, page, pageSize int) ([]zentaosdk.Story, error) {
	all, err := c.getAllStoriesSession(ctx, productID)
	if err != nil {
		return nil, err
	}
	out := make([]zentaosdk.Story, 0, len(all))
	start := (page - 1) * pageSize
	if start < 0 {
		start = 0
	}
	end := start + pageSize
	if end > len(all) {
		end = len(all)
	}
	if start < len(all) {
		out = append(out, all[start:end]...)
	}
	return out, nil
}

func (c *Client) getAllStoriesSession(ctx context.Context, productID int) ([]zentaosdk.Story, error) {
	path := fmt.Sprintf("/product-browse-%d-story.json", productID)
	var resp sessionProductStoryResp
	if err := c.doSessionJSON(ctx, "GetAllStories", path, &resp); err != nil {
		return nil, err
	}
	out := make([]zentaosdk.Story, 0, len(resp.Stories))
	for _, s := range resp.Stories {
		out = append(out, s.toSDK())
	}
	return out, nil
}

func (c *Client) getStoriesByProjectSession(ctx context.Context, projectID, page, pageSize int) ([]zentaosdk.Story, error) {
	// 通过 execution 反查：项目下所有 execution 的 story 合并。
	execs, err := c.getExecutionsSession(ctx, projectID)
	if err != nil {
		return nil, err
	}
	seen := make(map[int]bool)
	var out []zentaosdk.Story
	for _, e := range execs {
		ss, serr := c.getStoriesByExecutionSession(ctx, e.ID, 1, 10000)
		if serr != nil {
			continue
		}
		for _, s := range ss {
			if !seen[s.ID] {
				seen[s.ID] = true
				out = append(out, s)
			}
		}
	}
	start := (page - 1) * pageSize
	if start < 0 {
		start = 0
	}
	end := start + pageSize
	if end > len(out) {
		end = len(out)
	}
	if start >= len(out) {
		return nil, nil
	}
	return out[start:end], nil
}

func (c *Client) getStoriesByExecutionSession(ctx context.Context, executionID, page, pageSize int) ([]zentaosdk.Story, error) {
	// /execution-story-{execID}.json 在受限实例上可用，结构同 product-browse-story。
	path := fmt.Sprintf("/execution-story-%d.json", executionID)
	var resp sessionProductStoryResp
	if err := c.doSessionJSON(ctx, "GetStoriesByExecution", path, &resp); err != nil {
		return nil, err
	}
	all := make([]zentaosdk.Story, 0, len(resp.Stories))
	for _, s := range resp.Stories {
		all = append(all, s.toSDK())
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

func (c *Client) getAllStoriesByProjectSession(ctx context.Context, projectID int) ([]zentaosdk.Story, error) {
	return c.getStoriesByProjectSession(ctx, projectID, 1, 100000)
}

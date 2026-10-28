package zentao

import (
	"context"
	"encoding/json"
	"fmt"

	zentaosdk "github.com/yi-nology/zentao-max/backend/internal/infra/zentao/sdk"
)

// ---- Efforts (session mode) ----
//
// 工时记录的端点在受限实例上较特殊：/task-recordEstimate-{taskID} 是 AJAX 表单页，
// 不直接返回 JSON 数据。会话模式下尝试两个来源：
//  1. /task-recordEstimate-{taskID}.json（部分实例会返回 efforts 数组）
//  2. 失败/空则返回空切片（your-zentao-server.com 实测拿不到，dashboard 的 timelog 统计
//     会显示 0，不影响 bug/story/task 主路径）
//
// 解析需兼容三种 shape（与 SDK getRawBytes 逻辑对齐）：
//   - {effort: {1: {...}}} map keyed by index
//   - {effort: [...]} array
//   - 空对象

type sessionEffortWrapper struct {
	Effort json.RawMessage `json:"effort"`
}

type sessionEffortEntry struct {
	ID         int     `json:"id"`
	ObjectType string  `json:"objectType"`
	ObjectID   int     `json:"objectID"`
	Product    string  `json:"product"`
	Project    int     `json:"project"`
	Execution  int     `json:"execution"`
	Account    string  `json:"account"`
	Work       string  `json:"work"`
	Date       string  `json:"date"`
	Left       float64 `json:"left"`
	Consumed   float64 `json:"consumed"`
	Begin      string  `json:"begin"`
	End        string  `json:"end"`
	Deleted    string  `json:"deleted"`
}

func (e sessionEffortEntry) toSDK() zentaosdk.EffortEntry {
	return zentaosdk.EffortEntry{
		ID:         e.ID,
		ObjectType: e.ObjectType,
		ObjectID:   e.ObjectID,
		Product:    e.Product,
		Project:    e.Project,
		Execution:  e.Execution,
		Account:    e.Account,
		Work:       e.Work,
		Date:       e.Date,
		Left:       e.Left,
		Consumed:   e.Consumed,
		Begin:      e.Begin,
		End:        e.End,
		Deleted:    e.Deleted,
	}
}

func (c *Client) getTaskEffortsSession(ctx context.Context, taskID int) ([]zentaosdk.EffortEntry, error) {
	path := fmt.Sprintf("/task-recordEstimate-%d.json", taskID)
	var wrapper sessionEffortWrapper
	if err := c.doSessionJSON(ctx, "GetTaskEfforts", path, &wrapper); err != nil {
		if IsSessionAccessDenied(err) {
			return nil, nil
		}
		return nil, err
	}
	return parseSessionEffort(wrapper.Effort), nil
}

// parseSessionEffort 兼容 effort 字段的三种 shape。
func parseSessionEffort(raw json.RawMessage) []zentaosdk.EffortEntry {
	if len(raw) == 0 {
		return nil
	}
	// 尝试 map[string]sessionEffortEntry
	var m map[string]sessionEffortEntry
	if err := json.Unmarshal(raw, &m); err == nil {
		out := make([]zentaosdk.EffortEntry, 0, len(m))
		for _, e := range m {
			if e.Deleted == "1" {
				continue
			}
			out = append(out, e.toSDK())
		}
		return out
	}
	// 尝试 []sessionEffortEntry
	var arr []sessionEffortEntry
	if err := json.Unmarshal(raw, &arr); err == nil {
		out := make([]zentaosdk.EffortEntry, 0, len(arr))
		for _, e := range arr {
			if e.Deleted == "1" {
				continue
			}
			out = append(out, e.toSDK())
		}
		return out
	}
	return nil
}

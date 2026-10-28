package mcp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	zentaosdk "github.com/yi-nology/zentao-max/backend/core/zentao/sdk"
)

// MCP 写工具处理器：写禅道日志（挂任务）、Bug 备注、Bug 状态流转。
// 权限由 MCP 自身的 token 鉴权 + 只读模式（IsWriteAction）控制。

func intFromParams(params map[string]interface{}, key string) (int, error) {
	v, ok := params[key]
	if !ok {
		return 0, fmt.Errorf("缺少必填参数 %s", key)
	}
	if f, ok := v.(float64); ok {
		id := int64(f)
		if id <= 0 {
			return 0, fmt.Errorf("无效的 %s: %v", key, v)
		}
		return int(id), nil
	}
	id, err := strconv.Atoi(fmt.Sprintf("%v", v))
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("无效的 %s: %v", key, v)
	}
	return id, nil
}

func stringFromParams(params map[string]interface{}, key string) string {
	if v, ok := params[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func floatFromParams(params map[string]interface{}, key string) (float64, error) {
	v, ok := params[key]
	if !ok {
		return 0, fmt.Errorf("缺少必填参数 %s", key)
	}
	if f, ok := v.(float64); ok {
		return f, nil
	}
	f, err := strconv.ParseFloat(fmt.Sprintf("%v", v), 64)
	if err != nil {
		return 0, fmt.Errorf("无效的 %s: %v", key, v)
	}
	return f, nil
}

// handleRecordEffort 写禅道日志：在指定日期为任务记录一条工时（挂任务）。
// 参数：taskId（必填）、consumed（必填 >0）、left（必填，绝对值）、work（必填）、date（默认今天）。
func (s *MCPServer) handleRecordEffort(params map[string]interface{}) (interface{}, error) {
	taskID, err := intFromParams(params, "taskId")
	if err != nil {
		return nil, err
	}
	consumed, err := floatFromParams(params, "consumed")
	if err != nil {
		return nil, err
	}
	if consumed <= 0 {
		return nil, fmt.Errorf("consumed 必须大于 0（禅道拒绝 0 工时）")
	}
	left, err := floatFromParams(params, "left")
	if err != nil {
		return nil, err
	}
	work := stringFromParams(params, "work")
	if work == "" {
		return nil, fmt.Errorf("work（工作内容）不能为空")
	}
	date := stringFromParams(params, "date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	if err := s.taskService.RecordEffort(taskID, date, consumed, left, work); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("已记录工时：%s %gh → 任务 #%d（写后请在工时日历回查核对）", date, consumed, taskID),
		"data": map[string]interface{}{
			"taskId": taskID, "date": date, "consumed": consumed, "left": left, "work": work,
		},
	}, nil
}

// handleAddBugComment 给 Bug 添加备注。
// 复用已实测可用的 bug-edit 全量通道：先取 Bug 原值，再带 comment 提交。
func (s *MCPServer) handleAddBugComment(params map[string]interface{}) (interface{}, error) {
	bugID, err := intFromParams(params, "bugId")
	if err != nil {
		return nil, err
	}
	comment := stringFromParams(params, "comment")
	if comment == "" {
		return nil, fmt.Errorf("comment 不能为空")
	}

	bug, err := s.client.GetBugByID(context.Background(), bugID)
	if err != nil {
		return nil, err
	}
	// bug-edit 全量更新：必带 title/openedBuild/execution 原值；&#039; 先还原防二次转义
	req := zentaosdk.BugUpdateRequest{
		Title:      replaceHTMLEscape(bug.Title),
		Type:       bug.Type,
		Severity:   toInt(bug.Severity),
		Pri:        toInt(bug.Pri),
		Steps:      bug.Steps,
		AssignedTo: accountOf(bug.AssignedTo),
		OpenedBuild: func() []string {
			if len(bug.OpenedBuild) > 0 {
				return bug.OpenedBuild
			}
			return []string{"trunk"}
		}(),
		Execution: bug.Execution,
		Comment:   comment,
	}
	if _, err := s.client.UpdateBug(bugID, req); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("备注已提交到 Bug #%d（success≠落库，请回查核对）", bugID),
	}, nil
}

// handleTransitionBug 执行 Bug 状态流转（confirm/resolve/close/activate/assign）。
// 走 client 既有的 session 写通道（resolve/close 已实测落库）。
func (s *MCPServer) handleTransitionBug(params map[string]interface{}) (interface{}, error) {
	bugID, err := intFromParams(params, "bugId")
	if err != nil {
		return nil, err
	}
	comment := stringFromParams(params, "comment")
	action := stringFromParams(params, "action")

	switch action {
	case "confirm":
		err = s.client.ConfirmBug(bugID, zentaosdk.BugConfirmRequest{Comment: comment})
	case "resolve":
		resolution := stringFromParams(params, "resolution")
		if resolution == "" {
			return nil, fmt.Errorf("resolve 必须指定 resolution（fixed/bydesign/duplicate/notrepro/postponed/willnotfix/external）")
		}
		err = s.client.ResolveBug(bugID, zentaosdk.BugResolveRequest{
			Resolution:    resolution,
			ResolvedBuild: stringFromParams(params, "resolvedBuild"),
			Comment:       comment,
		})
	case "close":
		err = s.client.CloseBug(bugID, zentaosdk.BugCloseRequest{Comment: comment})
	case "activate":
		err = s.client.ActivateBug(bugID, zentaosdk.BugActivateRequest{
			AssignedTo: stringFromParams(params, "assignedTo"),
			Comment:    comment,
		})
	case "assign":
		assignedTo := stringFromParams(params, "assignedTo")
		if assignedTo == "" {
			return nil, fmt.Errorf("assign 必须指定 assignedTo（禅道账号）")
		}
		err = s.client.AssignBug(bugID, zentaosdk.BugAssignRequest{AssignedTo: assignedTo, Comment: comment})
	default:
		return nil, fmt.Errorf("不支持的 action: %s（支持 confirm/resolve/close/activate/assign）", action)
	}
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("状态流转成功（%s），Bug #%d（请回查核对状态）", action, bugID),
	}, nil
}

// ---- 小工具：SDK 松散类型归一 ----

func toInt(v interface{}) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case string:
		n, _ := strconv.Atoi(t)
		return n
	}
	return 0
}

func accountOf(ref zentaosdk.UserRef) string {
	return ref.Account
}

func replaceHTMLEscape(s string) string {
	out := strings.ReplaceAll(s, "&#039;", "'")
	out = strings.ReplaceAll(out, "&quot;", "\"")
	out = strings.ReplaceAll(out, "&amp;", "&")
	return out
}

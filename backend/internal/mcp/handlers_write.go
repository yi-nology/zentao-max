package mcp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	myzentao "github.com/yi-nology/zentao-max/backend/internal/infra/zentao"
	zentaosdk "github.com/yi-nology/zentao-max/backend/internal/infra/zentao/sdk"
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
	// 走 effort-batchCreate 批量端点（实测产生日志条目）。
	// task-recordEstimate 端点只更新任务"预计剩余"，不产生日志条目，语义错误。
	execution := 0
	if v, ok := params["execution"].(float64); ok && v > 0 {
		execution = int(v)
	}
	if execution == 0 {
		// 从我负责的任务里补 execution
		tasks, terr := s.client.GetMyWorkTasks(context.Background())
		if terr == nil {
			for _, t := range tasks {
				if t.ID == taskID && t.Execution != 0 {
					execution = t.Execution
					break
				}
			}
		}
	}
	if execution == 0 {
		return nil, fmt.Errorf("无法确定任务 #%d 所属执行：请传 execution 参数（受限实例要求工时必须挂执行）", taskID)
	}
	if err := s.client.BatchCreateEffort(context.Background(), date, []myzentao.BatchCreateEffortItem{{
		ObjectType: fmt.Sprintf("task_%d", taskID),
		Execution:  execution,
		Consumed:   consumed,
		Left:       left,
		Work:       work,
	}}); err != nil {
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

// handleCreateBug 创建禅道 Bug。
// 复用已实测可用的 bug-create 通道（openedBuild[]、项目反查、受限实例定制必填
// txjg[]/Evaluationanalysis 均由 client 处理）；成功返回新 Bug ID（来自跳转地址）。
func (s *MCPServer) handleCreateBug(params map[string]interface{}) (interface{}, error) {
	productID, err := intFromParams(params, "productId")
	if err != nil {
		return nil, err
	}
	title := stringFromParams(params, "title")
	if title == "" {
		return nil, fmt.Errorf("title 不能为空")
	}
	severity := 3
	if v, ok := params["severity"].(float64); ok && v >= 1 && v <= 4 {
		severity = int(v)
	}
	pri := 3
	if v, ok := params["pri"].(float64); ok && v >= 1 && v <= 4 {
		pri = int(v)
	}
	bugType := stringFromParams(params, "type")
	if bugType == "" {
		bugType = "codeerror"
	}
	openedBuild := stringFromParams(params, "openedBuild")
	if openedBuild == "" {
		openedBuild = "trunk"
	}
	execution := 0
	if v, ok := params["execution"].(float64); ok && v > 0 {
		execution = int(v)
	}
	module := 0
	if v, ok := params["module"].(float64); ok && v > 0 {
		module = int(v)
	}

	bug, err := s.client.CreateBug(productID, zentaosdk.BugCreateRequest{
		Title:       title,
		Steps:       stringFromParams(params, "steps"),
		Severity:    severity,
		Pri:         pri,
		Type:        bugType,
		AssignedTo:  stringFromParams(params, "assignedTo"),
		OpenedBuild: []string{openedBuild},
		Execution:   execution,
		Module:      module,
	})
	if err != nil {
		return nil, err
	}
	idMsg := ""
	if bug.ID > 0 {
		idMsg = fmt.Sprintf("，新 Bug ID #%d", bug.ID)
	}
	return map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("Bug 创建成功%s（success≠落库，请用 get_bug_detail 回查核对）", idMsg),
		"data":    bug,
	}, nil
}

// handleTransitionTask 任务状态流转（start/finish/pause/assign）。
// 走 client 既有的 session 写通道；finish 建议带 consumed（累计值），
// 剩余工时的同步更新由 record_effort 承担（task-finish 端点不带 left 字段）。
func (s *MCPServer) handleTransitionTask(params map[string]interface{}) (interface{}, error) {
	taskID, err := intFromParams(params, "taskId")
	if err != nil {
		return nil, err
	}
	comment := stringFromParams(params, "comment")
	action := stringFromParams(params, "action")

	switch action {
	case "start":
		req := zentaosdk.TaskStartRequest{Comment: comment}
		if v, ok := params["left"].(float64); ok && v >= 0 {
			req.Left = v
		}
		err = s.client.StartTask(taskID, req)
	case "finish":
		req := zentaosdk.TaskFinishRequest{
			FinishedDate: stringFromParams(params, "finishedDate"),
			Comment:      comment,
		}
		if v, ok := params["consumed"].(float64); ok && v >= 0 {
			req.Consumed = v
		}
		if req.Consumed == 0 {
			return nil, fmt.Errorf("finish 必须携带 consumed（累计消耗工时，>0）；只想记工时/更新剩余请改用 record_effort")
		}
		err = s.client.FinishTask(taskID, req)
	case "pause":
		err = s.client.PauseTask(taskID, zentaosdk.TaskPauseRequest{Comment: comment})
	case "assign":
		assignedTo := stringFromParams(params, "assignedTo")
		if assignedTo == "" {
			return nil, fmt.Errorf("assign 必须指定 assignedTo（禅道账号）")
		}
		req := zentaosdk.TaskAssignRequest{AssignedTo: assignedTo, Comment: comment}
		if v, ok := params["left"].(float64); ok && v >= 0 {
			req.Left = v
		}
		err = s.client.AssignTask(taskID, req)
	default:
		return nil, fmt.Errorf("不支持的 action: %s（支持 start/finish/pause/assign）", action)
	}
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("任务流转成功（%s），任务 #%d（请用 get_task_detail 回查核对状态）", action, taskID),
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

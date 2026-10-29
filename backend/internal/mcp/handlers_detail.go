package mcp

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

// MCP 读工具处理器：单实体详情、我的任务、工时日历。
// 与列表工具（handlers.go）的区别：这些走 client 直连句柄，
// 覆盖 service 层未封装的单实体端点。

// handleGetBugDetail 获取单个 Bug 详情.
func (s *MCPServer) handleGetBugDetail(params map[string]interface{}) (interface{}, error) {
	bugID, err := intFromParams(params, "bugId")
	if err != nil {
		return nil, err
	}
	bug, err := s.client.GetBugByID(context.Background(), bugID)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("Bug #%d 详情", bugID),
		"data":    bug,
	}, nil
}

// handleGetTaskDetail 获取单个任务详情.
func (s *MCPServer) handleGetTaskDetail(params map[string]interface{}) (interface{}, error) {
	taskID, err := intFromParams(params, "taskId")
	if err != nil {
		return nil, err
	}
	task, err := s.client.GetTaskByID(context.Background(), taskID)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("任务 #%d 详情", taskID),
		"data":    task,
	}, nil
}

// handleGetStoryDetail 获取单个需求详情.
func (s *MCPServer) handleGetStoryDetail(params map[string]interface{}) (interface{}, error) {
	storyID, err := intFromParams(params, "storyId")
	if err != nil {
		return nil, err
	}
	story, err := s.client.GetStoryByID(context.Background(), storyID)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("需求 #%d 详情", storyID),
		"data":    story,
	}, nil
}

// handleGetMyTasks 获取我负责/参与的任务列表.
func (s *MCPServer) handleGetMyTasks(_ map[string]interface{}) (interface{}, error) {
	tasks, err := s.client.GetMyWorkTasks(context.Background())
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("我的任务共 %d 个", len(tasks)),
		"data":    tasks,
	}, nil
}

// handleGetEffortCalendar 获取指定用户全年工时日志。
// userID 是禅道 user.id（非账号），传 account 时经用户列表反查映射。
func (s *MCPServer) handleGetEffortCalendar(params map[string]interface{}) (interface{}, error) {
	userID := 0
	if v, ok := params["userId"]; ok {
		id, err := strconv.Atoi(fmt.Sprintf("%v", v))
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("无效的 userId: %v", v)
		}
		userID = id
	}
	account := stringFromParams(params, "account")
	if userID == 0 && account == "" {
		// 都不传：默认查当前禅道登录账号
		account = s.client.GetAccount()
		if account == "" {
			return nil, fmt.Errorf("请传 account 或 userId 指定要查询的用户")
		}
	}
	if userID == 0 {
		users, err := s.userService.GetUsersAll()
		if err != nil {
			return nil, fmt.Errorf("用户列表查询失败（用于账号→ID 映射）: %w", err)
		}
		for _, u := range users {
			if u.Account == account {
				userID = u.ID
				break
			}
		}
		if userID == 0 {
			return nil, fmt.Errorf("未找到账号 %q 对应的禅道用户，可用 get_users 核对", account)
		}
	}
	year := time.Now().Year()
	if v, ok := params["year"]; ok {
		y, err := strconv.Atoi(fmt.Sprintf("%v", v))
		if err != nil || y < 2000 || y > 2100 {
			return nil, fmt.Errorf("无效的 year: %v", v)
		}
		year = y
	}
	entries, err := s.client.GetEffortCalendar(context.Background(), userID, year)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("%d 年共 %d 条工时记录（用户 %s）", year, len(entries), account),
		"data":    slimViaJSON(entries, []string{"work"}),
	}, nil
}

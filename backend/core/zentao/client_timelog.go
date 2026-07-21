package zentao

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/yi-nology/common/biz/zentao"
)

// effortItem 工时记录项
type effortItem struct {
	ID        int     `json:"id"`
	TaskID    int     `json:"taskId"`
	TaskName  string  `json:"taskName"`
	TaskType  string  `json:"taskType"`
	Product   string  `json:"product"`
	Project   string  `json:"project"`
	Execution string  `json:"execution"`
	Account   string  `json:"account"`
	Date      string  `json:"date"`
	Consumed  float64 `json:"consumed"`
	Left      float64 `json:"left"`
	Work      string  `json:"work"`
}

// statItem 统计项
type statItem struct {
	Name  string  `json:"name"`
	Hours float64 `json:"hours"`
	Count int     `json:"count"`
}

// dailyStat 每日统计
type dailyStat struct {
	Date  string  `json:"date"`
	Hours float64 `json:"hours"`
	Count int     `json:"count"`
}

func mapToSlice(m map[string]*statItem) []statItem {
	result := make([]statItem, 0, len(m))
	for _, v := range m {
		result = append(result, *v)
	}
	return result
}

// GetTaskEfforts 获取任务的工时记录
func (c *Client) GetTaskEfforts(taskID int) ([]zentao.EffortEntry, error) {
	return c.getTaskEffortsSession(context.Background(), taskID)
}

// GetTaskEffortsContext 获取任务的工时记录（支持 context 取消）
func (c *Client) GetTaskEffortsContext(ctx context.Context, taskID int) ([]zentao.EffortEntry, error) {
	return c.getTaskEffortsSession(ctx, taskID)
}

// GetTimelogAnalysis 获取工时统计分析
//
// productId 缺省/0 时走 user 维度（effort-calendar），避免受限 session 模式下
// product 过滤吞掉 user 的全部工时。其他维度（project/execution/assignedTo/date range）
// 都仍然支持过滤。
func (c *Client) GetTimelogAnalysis(productID, projectID, executionID, assignedTo, dateFrom, dateTo string) (map[string]interface{}, error) {
	ctx := context.Background()

	prodID, _ := strconv.Atoi(productID)

	// productId 缺省/0 → fallback user 维度
	if prodID <= 0 {
		return c.getTimelogAnalysisByUser(ctx, assignedTo, dateFrom, dateTo)
	}

	var filterProjectID, filterExecutionID int
	if projectID != "" {
		filterProjectID, _ = strconv.Atoi(projectID)
	}
	if executionID != "" {
		filterExecutionID, _ = strconv.Atoi(executionID)
	}

	productName := fmt.Sprintf("产品%d", prodID)
	if product, perr := c.GetProductContext(ctx, prodID); perr == nil && product != nil {
		productName = product.Name
	}

	projects, err := c.GetProjectsByProductContext(ctx, prodID, 1, 100)
	if err != nil {
		return nil, fmt.Errorf("获取项目列表失败: %w", err)
	}

	if filterProjectID > 0 {
		filtered := make([]zentao.Project, 0)
		for _, p := range projects {
			if p.ID == filterProjectID {
				filtered = append(filtered, p)
				break
			}
		}
		projects = filtered
	}

	type execWithContext struct {
		Exec     zentao.Execution
		ProjName string
	}

	var allExecs []execWithContext

	if filterExecutionID > 0 {
		projName := ""
		for _, p := range projects {
			projName = p.Name
			break
		}
		allExecs = append(allExecs, execWithContext{
			Exec:     zentao.Execution{ID: filterExecutionID},
			ProjName: projName,
		})
	} else {
		for _, proj := range projects {
			execs, eerr := c.GetExecutionsContext(ctx, proj.ID, 1, 100)
			if eerr != nil {
				continue
			}
			for _, e := range execs {
				allExecs = append(allExecs, execWithContext{Exec: e, ProjName: proj.Name})
			}
		}
	}

	type taskContext struct {
		Task     zentao.Task
		ProjName string
		ExecName string
	}
	var allTaskCtx []taskContext

	for _, ec := range allExecs {
		tasks, terr := c.GetTasksContext(ctx, ec.Exec.ID, 1, 500)
		if terr != nil {
			continue
		}
		for _, t := range tasks {
			if toFloat64(t.Consumed) > 0 {
				allTaskCtx = append(allTaskCtx, taskContext{
					Task:     t,
					ProjName: ec.ProjName,
					ExecName: ec.Exec.Name,
				})
			}
		}
	}

	var allEfforts []effortItem

	for _, tc := range allTaskCtx {
		efforts, ferr := c.GetTaskEffortsContext(ctx, tc.Task.ID)
		if ferr != nil {
			continue
		}

		for _, e := range efforts {
			if assignedTo != "" && e.Account != assignedTo {
				continue
			}
			if dateFrom != "" && e.Date < dateFrom {
				continue
			}
			if dateTo != "" && e.Date > dateTo {
				continue
			}

			allEfforts = append(allEfforts, effortItem{
				ID:        e.ID,
				TaskID:    tc.Task.ID,
				TaskName:  tc.Task.Name,
				TaskType:  tc.Task.Type,
				Product:   productName,
				Project:   tc.ProjName,
				Execution: tc.ExecName,
				Account:   e.Account,
				Date:      e.Date,
				Consumed:  e.Consumed,
				Left:      e.Left,
				Work:      e.Work,
			})
		}
	}

	var totalHours float64
	byProjectMap := make(map[string]*statItem)
	byTypeMap := make(map[string]*statItem)
	byDateMap := make(map[string]*dailyStat)

	typeNames := map[string]string{
		"devel": "开发", "design": "设计", "test": "测试",
		"study": "研究", "discuss": "讨论", "ui": "界面",
		"affair": "事务", "misc": "其他",
	}

	for _, e := range allEfforts {
		totalHours += e.Consumed

		if _, ok := byProjectMap[e.Project]; !ok {
			byProjectMap[e.Project] = &statItem{Name: e.Project}
		}
		byProjectMap[e.Project].Hours += e.Consumed
		byProjectMap[e.Project].Count++

		typeName := e.TaskType
		if tn, ok := typeNames[e.TaskType]; ok {
			typeName = tn
		}
		if _, ok := byTypeMap[typeName]; !ok {
			byTypeMap[typeName] = &statItem{Name: typeName}
		}
		byTypeMap[typeName].Hours += e.Consumed
		byTypeMap[typeName].Count++

		if _, ok := byDateMap[e.Date]; !ok {
			byDateMap[e.Date] = &dailyStat{Date: e.Date}
		}
		byDateMap[e.Date].Hours += e.Consumed
		byDateMap[e.Date].Count++
	}

	byProject := mapToSlice(byProjectMap)
	byType := mapToSlice(byTypeMap)

	byDate := make([]dailyStat, 0, len(byDateMap))
	for _, v := range byDateMap {
		byDate = append(byDate, *v)
	}

	effortsData := make([]map[string]interface{}, len(allEfforts))
	for i, e := range allEfforts {
		effortsData[i] = map[string]interface{}{
			"id":        e.ID,
			"taskId":    e.TaskID,
			"taskName":  e.TaskName,
			"taskType":  e.TaskType,
			"product":   e.Product,
			"project":   e.Project,
			"execution": e.Execution,
			"account":   e.Account,
			"date":      e.Date,
			"consumed":  e.Consumed,
			"left":      e.Left,
			"work":      e.Work,
		}
	}

	return map[string]interface{}{
		"totalHours":  totalHours,
		"effortCount": len(allEfforts),
		"taskCount":   len(allTaskCtx),
		"byProject":   byProject,
		"byType":      byType,
		"byDate":      byDate,
		"efforts":     effortsData,
	}, nil
}

// getTimelogAnalysisByUser 在 productId 缺省时走 user 维度 effort-calendar。
//
// 受限 session 模式下 <user-account> 的工时跟 product 不强关联，product 过滤经常
// 吞掉 user 的全部工时。这里直接从 effort-ajaxGetEfforts-{userID}-{year}.html
// 拉数据（user 维度，不限 product），按 dateFrom/dateTo/assignedTo 过滤，
// 重新聚合成前端 TimelogAnalysis 期望的 shape。
//
// 注意：effort-calendar 没有 project/execution/account/work 字段（CalendarEntry
// 只有 id/title/objectType/start/end/consumed/url），这些字段会标记为
// "（user 维度）" / 空，前端 Timelog UI 显示时这些列会空/占位。如需完整字段，
// 需要走 task-effort 端点（GetTaskEffortsContext）再关联 task 详情。
func (c *Client) getTimelogAnalysisByUser(ctx context.Context, assignedTo, dateFrom, dateTo string) (map[string]interface{}, error) {
	// 1. 取当前 user（user 维度要 userID）
	me, err := c.GetCurrentUser()
	if err != nil || me == nil {
		return map[string]interface{}{
			"totalHours": 0, "effortCount": 0, "taskCount": 0,
			"byProject": []interface{}{}, "byType": []interface{}{}, "byDate": []interface{}{},
			"efforts": []interface{}{},
		}, nil
	}
	// 2. 从 dateFrom 提取 year（跨年时按主年份拉；同年内 filter by dateFrom/dateTo 已够）
	year := 0
	if dateFrom != "" && len(dateFrom) >= 4 {
		if y, err := strconv.Atoi(dateFrom[:4]); err == nil {
			year = y
		}
	}
	if year == 0 {
		year = CurrentYear()
	}

	// 3. 拉整年 entries
	entries, err := c.GetEffortCalendar(ctx, me.ID, year)
	if err != nil {
		return map[string]interface{}{
			"totalHours": 0, "effortCount": 0, "taskCount": 0,
			"byProject": []interface{}{}, "byType": []interface{}{}, "byDate": []interface{}{},
			"efforts": []interface{}{},
		}, nil
	}

	// 4. 过滤 + 聚合
	var totalHours float64
	byProjectMap := make(map[string]*statItem)
	byTypeMap := make(map[string]*statItem)
	byDateMap := make(map[string]*dailyStat)
	taskSet := make(map[int]struct{})
	effortsData := make([]map[string]interface{}, 0, len(entries))

	typeNames := map[string]string{
		"task": "任务", "bug": "Bug", "story": "需求", "project": "项目",
	}

	for _, e := range entries {
		if dateFrom != "" && e.Start < dateFrom {
			continue
		}
		if dateTo != "" && e.Start > dateTo {
			continue
		}
		if assignedTo != "" && me.Account != "" && assignedTo != me.Account {
			continue
		}

		totalHours += e.Consumed
		taskSet[e.ID] = struct{}{}

		// byProject 走 user 维度的占位
		projName := "（user 维度）"
		if _, ok := byProjectMap[projName]; !ok {
			byProjectMap[projName] = &statItem{Name: projName}
		}
		byProjectMap[projName].Hours += e.Consumed
		byProjectMap[projName].Count++

		// byType 用 objectType（task/bug/...）
		typeName := e.ObjectType
		if tn, ok := typeNames[e.ObjectType]; ok {
			typeName = tn
		}
		if typeName == "" {
			typeName = "其他"
		}
		if _, ok := byTypeMap[typeName]; !ok {
			byTypeMap[typeName] = &statItem{Name: typeName}
		}
		byTypeMap[typeName].Hours += e.Consumed
		byTypeMap[typeName].Count++

		// byDate
		if _, ok := byDateMap[e.Start]; !ok {
			byDateMap[e.Start] = &dailyStat{Date: e.Start}
		}
		byDateMap[e.Start].Hours += e.Consumed
		byDateMap[e.Start].Count++

		effortsData = append(effortsData, map[string]interface{}{
			"id":        e.ID,
			"taskId":    0, // effort-calendar 不暴露 taskID
			"taskName":  e.Title,
			"taskType":  typeName,
			"product":   "（user 维度）",
			"project":   projName,
			"execution": "（user 维度）",
			"account":   me.Account,
			"date":      e.Start,
			"consumed":  e.Consumed,
			"left":      0.0,
			"work":      "",
		})
	}

	byProject := mapToSlice(byProjectMap)
	byType := mapToSlice(byTypeMap)
	byDate := make([]dailyStat, 0, len(byDateMap))
	for _, d := range byDateMap {
		byDate = append(byDate, *d)
	}
	// 按日期排序
	sort.SliceStable(byDate, func(i, j int) bool { return byDate[i].Date < byDate[j].Date })

	return map[string]interface{}{
		"totalHours":  totalHours,
		"effortCount": len(effortsData),
		"taskCount":   len(taskSet),
		"byProject":   byProject,
		"byType":      byType,
		"byDate":      byDate,
		"efforts":     effortsData,
	}, nil
}

// toFloat64 把 SDK 里 interface{} 类型的 Consumed/Left 安全转 float64。
// session 模式映射时这些字段已是 float64；token 模式下 SDK 可能返回 float64 或 json.Number。
func toFloat64(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return f
	}
	return 0
}

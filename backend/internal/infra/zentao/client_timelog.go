package zentao

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/oekit/concurrency/async"

	"github.com/yi-nology/zentao-max/backend/pkg/logger"
	zentaosdk "github.com/yi-nology/zentao-max/backend/internal/infra/zentao/sdk"
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
func (c *Client) GetTaskEfforts(taskID int) ([]zentaosdk.EffortEntry, error) {
	return c.getTaskEffortsSession(context.Background(), taskID)
}

// GetTaskEffortsContext 获取任务的工时记录（支持 context 取消）
func (c *Client) GetTaskEffortsContext(ctx context.Context, taskID int) ([]zentaosdk.EffortEntry, error) {
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

	// 耗时/正确性总闸：工时分析三层嵌套（执行→任务→工时）曾是最重遍历——
	// 旧实现按"项目→GetExecutionsContext"收集，而 session 模式下每个项目的
	// GetExecutions 都返回全量执行且无去重：同一执行被收集 N_proj 次，工时
	// 被逐任务重复拉取 N_proj 次，totalHours/byProject/byDate 全部虚增。
	// 改为单次全执行遍历（GetAllExecutionsContext 内置 45s 翻页闸）；
	// 任务/工时两层共享一个 60s 闸，在执行列表完成后再起算——否则列表先
	// 吃掉大半预算，任务/工时层只剩十几秒（实测截断到 4 个任务、工时为 0）。

	type execWithContext struct {
		Exec zentaosdk.Execution
	}

	var allExecs []execWithContext

	if filterExecutionID > 0 {
		allExecs = append(allExecs, execWithContext{
			Exec: zentaosdk.Execution{ID: filterExecutionID},
		})
	} else {
		allExecList, err := c.GetAllExecutionsContext(ctx)
		if err != nil {
			return nil, fmt.Errorf("获取执行列表失败: %w", err)
		}
		for _, e := range allExecList {
			if filterProjectID > 0 && e.Project != filterProjectID {
				continue
			}
			allExecs = append(allExecs, execWithContext{Exec: e})
		}
	}

	type taskContext struct {
		Task     zentaosdk.Task
		ExecName string
	}
	var allTaskCtx []taskContext

	// 每层独立 45s 预算：共享单闸时先跑的层会耗尽预算，后一层永远 0 收集
	taskDeadline := time.Now().Add(45 * time.Second)
	for _, ec := range allExecs {
		if time.Now().After(taskDeadline) {
			logger.Warn("工时分析任务收集超时，返回已收集部分", "executions", len(allExecs))
			break
		}
		tasks, terr := c.GetTasksContext(ctx, ec.Exec.ID, 1, 500)
		if terr != nil {
			continue
		}
		for _, t := range tasks {
			if toFloat64(t.Consumed) > 0 {
				allTaskCtx = append(allTaskCtx, taskContext{
					Task:     t,
					ExecName: ec.Exec.Name,
				})
			}
		}
	}

	var allEfforts []effortItem

	// 工时明细层并发拉取（cap 4 = SessionWorkerPool 读 worker 数）：逐任务
	// 请求串行跑不完预算（实测截断到 0 条工时）；并发 4 后闸内吞吐 ×4。
	// 每层独立 45s 预算：共享单闸时先跑的层会耗尽预算，后一层永远 0 收集。
	// effortDeadline 到达后不再提交新任务，等在飞请求收尾（最长一个请求时长）。
	{
		effortDeadline := time.Now().Add(45 * time.Second)
		var mu sync.Mutex
		sem := make(chan struct{}, 4)
		var wg sync.WaitGroup
		for _, tc := range allTaskCtx {
			if time.Now().After(effortDeadline) {
				logger.Warn("工时分析工时明细收集超时，返回已收集部分",
					"tasks", len(allTaskCtx), "collected", len(allEfforts))
				break
			}
			wg.Add(1)
			tc := tc
			async.GoSafe(func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				efforts, ferr := c.GetTaskEffortsContext(ctx, tc.Task.ID)
				if ferr != nil {
					return
				}
				var items []effortItem
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
					items = append(items, effortItem{
						ID:        e.ID,
						TaskID:    tc.Task.ID,
						TaskName:  tc.Task.Name,
						TaskType:  tc.Task.Type,
						Product:   productName,
						Project:   tc.ExecName,
						Execution: tc.ExecName,
						Account:   e.Account,
						Date:      e.Date,
						Consumed:  e.Consumed,
						Left:      e.Left,
						Work:      e.Work,
					})
				}
				if len(items) > 0 {
					mu.Lock()
					allEfforts = append(allEfforts, items...)
					mu.Unlock()
				}
			})
		}
		wg.Wait()
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

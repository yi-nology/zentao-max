package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yi-nology/zentao-max/backend/internal/dto"
	myzentao "github.com/yi-nology/zentao-max/backend/internal/infra/zentao"
)

// TimelogService 工时统计业务逻辑服务
// 负责处理工时统计相关的业务逻辑
type TimelogService struct {
	client *myzentao.Client
}

// NewTimelogService 创建工时统计服务
func NewTimelogService(client *myzentao.Client) *TimelogService {
	return &TimelogService{client: client}
}

// GetTimelogAnalysis 获取工时统计分析
//
// 双数据源按 realm 自动分支：
//   - 标准实例（Token 模式）：task-efforts 逐任务遍历（原实现）；
//   - 受限实例（Session 模式）：task-efforts 端点实测返回空（功能从未工作过），
//     改用 effort-calendar 按用户聚合——语义变为"我的工时"（当前登录账号，
//     受限实例无用户列表权限），产品/执行过滤不适用，按对象类型（任务/Bug）分组。
func (s *TimelogService) GetTimelogAnalysis(query *dto.TimelogQueryDTO) (map[string]interface{}, error) {
	if s.client.GetRealm() != "" {
		return s.getMyEffortAnalysis(query)
	}
	return s.client.GetTimelogAnalysis(
		query.ProductID,
		query.ProjectID,
		query.ExecutionID,
		query.AssignedTo,
		query.DateFrom,
		query.DateTo,
	)
}

// getMyEffortAnalysis 受限实例的工时分析：effort-calendar 按用户聚合。
// 单用户全年数据一次请求（~80KB），冷查秒级；日期范围跨年时逐年拉取。
func (s *TimelogService) getMyEffortAnalysis(query *dto.TimelogQueryDTO) (map[string]interface{}, error) {
	ctx := context.Background()

	account := query.AssignedTo
	if account == "" {
		account = s.client.GetAccount()
	}
	userID := 0
	if me, err := s.client.GetCurrentUser(); err == nil && me != nil && (account == "" || me.Account == account) {
		userID = me.ID
		account = me.Account
	}
	if userID == 0 {
		users, err := s.client.GetUsersAll()
		if err == nil {
			for _, u := range users {
				if u.Account == account {
					userID = u.ID
					break
				}
			}
		}
	}
	if userID == 0 {
		return nil, fmt.Errorf("无法定位用户 %q 的工时（受限实例仅支持查询当前登录账号）", account)
	}

	// 日期范围 → 年份集合（calendar 端点按用户+年查询）
	now := time.Now()
	yearFrom, yearTo := now.Year(), now.Year()
	if query.DateFrom != "" {
		if t, err := time.Parse("2006-01-02", query.DateFrom); err == nil {
			yearFrom = t.Year()
		}
	}
	if query.DateTo != "" {
		if t, err := time.Parse("2006-01-02", query.DateTo); err == nil {
			yearTo = t.Year()
		}
	}
	if yearTo < yearFrom {
		yearTo = yearFrom
	}
	if yearTo-yearFrom > 5 {
		yearTo = yearFrom + 5 // 安全闸
	}

	type entryRow struct {
		entry      myzentao.CalendarEntry
		objectType string
	}
	var rows []entryRow
	for y := yearFrom; y <= yearTo; y++ {
		entries, err := s.client.GetEffortCalendar(ctx, userID, y)
		if err != nil {
			continue
		}
		for _, e := range entries {
			// 对象类型优先从 URL 推断（/task-view-/bug-view-/story-view-…），
			// 比标题 [T]/[B] 前缀可靠（实测多数标题无前缀）
			ot := objectTypeFromURL(e.URL)
			if ot == "" {
				ot, _ = myzentao.ParseCalendarObjectType(e.Title)
			}
			if query.DateFrom != "" && e.Start < query.DateFrom {
				continue
			}
			if query.DateTo != "" && e.Start > query.DateTo {
				continue
			}
			rows = append(rows, entryRow{entry: e, objectType: ot})
		}
	}

	typeNames := map[string]string{
		"task": "任务", "bug": "Bug", "story": "需求", "todo": "待办",
	}
	var totalHours float64
	taskIDs := make(map[int]bool)
	byProjectMap := make(map[string]*timelogStatItem)
	byTypeMap := make(map[string]*timelogStatItem)
	byDateMap := make(map[string]*timelogDailyStat)

	effortsData := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		e := r.entry
		totalHours += e.Consumed
		taskIDs[e.ID] = true

		// 日历条目的 Title 是自由文本工作内容（无 [T]/[B] 前缀、URL 无对象 ID），
		// 对象类型多数不可解析——组名回落为"我的工时"单组展示
		typeName := "我的工时"
		if r.objectType != "" {
			if tn, ok := typeNames[r.objectType]; ok {
				typeName = tn
			} else {
				typeName = r.objectType
			}
		}
		if _, ok := byProjectMap[typeName]; !ok {
			byProjectMap[typeName] = &timelogStatItem{Name: typeName}
		}
		byProjectMap[typeName].Hours += e.Consumed
		byProjectMap[typeName].Count++

		if _, ok := byTypeMap[typeName]; !ok {
			byTypeMap[typeName] = &timelogStatItem{Name: typeName}
		}
		byTypeMap[typeName].Hours += e.Consumed
		byTypeMap[typeName].Count++

		if _, ok := byDateMap[e.Start]; !ok {
			byDateMap[e.Start] = &timelogDailyStat{Date: e.Start}
		}
		byDateMap[e.Start].Hours += e.Consumed
		byDateMap[e.Start].Count++

		effortsData = append(effortsData, map[string]interface{}{
			"id":        e.ID,
			"taskId":    0,
			"taskName":  e.Title,
			"taskType":  r.objectType,
			"product":   "",
			"project":   typeName,
			"execution": "",
			"account":   account,
			"date":      e.Start,
			"consumed":  e.Consumed,
			"left":      0,
			"work":      e.Title,
		})
	}

	byProject := mapToTimelogSlice(byProjectMap)
	byType := mapToTimelogSlice(byTypeMap)
	byDate := make([]timelogDailyStat, 0, len(byDateMap))
	for _, v := range byDateMap {
		byDate = append(byDate, *v)
	}
	sort.Slice(byDate, func(i, j int) bool { return byDate[i].Date < byDate[j].Date })

	return map[string]interface{}{
		"totalHours":  totalHours,
		"effortCount": len(rows),
		"taskCount":   len(taskIDs),
		"byProject":   byProject,
		"byType":      byType,
		"byDate":      byDate,
		"efforts":     effortsData,
		"source":      "my-effort-calendar",
		"account":     account,
	}, nil
}

// ---- 本地聚合类型（与 zentao 包内 client_timelog 的聚合 shape 保持一致） ----

type timelogStatItem struct {
	Name   string  `json:"name"`
	Hours  float64 `json:"hours"`
	Count  int     `json:"count"`
}

type timelogDailyStat struct {
	Date  string  `json:"date"`
	Hours float64 `json:"hours"`
	Count int     `json:"count"`
}

func mapToTimelogSlice(m map[string]*timelogStatItem) []timelogStatItem {
	out := make([]timelogStatItem, 0, len(m))
	for _, v := range m {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Hours > out[j].Hours })
	return out
}

// objectTypeFromURL 从禅道详情页 URL 推断对象类型（task/bug/story/todo…）。
func objectTypeFromURL(rawURL string) string {
	for _, t := range []string{"task", "bug", "story", "todo", "testtask", "case"} {
		if strings.Contains(rawURL, "/"+t+"-view-") || strings.Contains(rawURL, "/"+t+"-") {
			return t
		}
	}
	return ""
}

// GetTimelogDashboard 获取工时统计看板数据
// 业务逻辑：提取看板相关的统计数据
func (s *TimelogService) GetTimelogDashboard(query *dto.TimelogQueryDTO) (map[string]interface{}, error) {
	analysis, err := s.GetTimelogAnalysis(query)
	if err != nil {
		return nil, err
	}

	// 提取看板数据（source/account 透传：前端据此切换"我的工时"模式，
	// 隐藏产品/执行等不参与过滤的筛选器）
	dashboardData := map[string]interface{}{
		"totalHours":  analysis["totalHours"],
		"effortCount": analysis["effortCount"],
		"taskCount":   analysis["taskCount"],
		"byProject":   analysis["byProject"],
		"byType":      analysis["byType"],
		"byDate":      analysis["byDate"],
	}
	for _, k := range []string{"source", "account"} {
		if v, ok := analysis[k]; ok {
			dashboardData[k] = v
		}
	}

	return dashboardData, nil
}

// GetTimelogEfforts 获取工时流水明细
// 业务逻辑：提取工时明细数据
func (s *TimelogService) GetTimelogEfforts(query *dto.TimelogQueryDTO) (interface{}, error) {
	analysis, err := s.GetTimelogAnalysis(query)
	if err != nil {
		return nil, err
	}

	// 提取明细数据
	effortsData := analysis["efforts"]

	return effortsData, nil
}

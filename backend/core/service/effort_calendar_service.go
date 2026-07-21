package service

import (
	"context"
	"fmt"

	myzentao "github.com/yi-nology/zentao-max/backend/core/zentao"
)

// === 工时日历 DTO（前端日历视图直接消费） ===

// EffortCalendarResult 业务层返回结构（含日聚合 + 月度汇总）。
type EffortCalendarResult struct {
	Entries  []EffortCalendarEntry      `json:"entries"`
	Summary  *EffortCalendarSummary     `json:"summary"`
	Year     int                        `json:"year"`
	UserID   int                        `json:"userId"`
	UserName string                     `json:"userName,omitempty"`
}

// EffortCalendarEntry 单条记录（直接透传 client.CalendarEntry 字段）。
type EffortCalendarEntry struct {
	ID         int     `json:"id"`
	Title      string  `json:"title"`
	ObjectType string  `json:"objectType"`
	Start      string  `json:"start"`
	End        string  `json:"end"`
	Consumed   float64 `json:"consumed"`
	URL        string  `json:"url"`
}

// EffortCalendarSummary 月度/全年汇总。
type EffortCalendarSummary struct {
	TotalHours    float64                            `json:"totalHours"`
	TotalRecords  int                                `json:"totalRecords"`
	WorkDays      int                                `json:"workDays"`      // 有工时的天数
	AvgDailyHours float64                            `json:"avgDailyHours"` // 平均每日（按 workDays）
	MaxDailyHours float64                            `json:"maxDailyHours"`
	MaxDailyDate  string                             `json:"maxDailyDate"`
	ByMonth       []EffortCalendarMonthStat          `json:"byMonth"`
	ByType        map[string]EffortCalendarTypeStat  `json:"byType"`
}

// EffortCalendarMonthStat 月度汇总。
type EffortCalendarMonthStat struct {
	Month       string  `json:"month"` // YYYY-MM
	TotalHours  float64 `json:"totalHours"`
	RecordCount int     `json:"recordCount"`
	WorkDays    int     `json:"workDays"`
}

// EffortCalendarTypeStat 按对象类型汇总。
type EffortCalendarTypeStat struct {
	Count int     `json:"count"`
	Hours float64 `json:"hours"`
}

// EffortCalendarServicer 是 service 层的对外接口，handler 依赖这个接口便于 mock。
type EffortCalendarServicer interface {
	GetEffortCalendar(userAccount string, year int) (*EffortCalendarResult, error)
}

// === service 实现 ===

// EffortCalendarService 工时日历业务层。
//
// 职责：
//   - 把 user account 解析成 user.id（effort-calendar 端点要的是 id 不是 account）
//   - 调 client.GetEffortCalendar 拉全年数据
//   - 在内存里按月/按类型聚合，返前端日历视图直接消费的 summary
//
// Session 模式专属（受限实例 SSO 等禁用 REST API 的实例）。
// 缓存策略：5min fresh / 1h stale（client.sessionCache 内置）。
type EffortCalendarService struct {
	client *myzentao.Client
}

// NewEffortCalendarService 构造 service。
func NewEffortCalendarService(client *myzentao.Client) *EffortCalendarService {
	return &EffortCalendarService{client: client}
}

// compile-time interface check
var _ EffortCalendarServicer = (*EffortCalendarService)(nil)

// GetEffortCalendar 拉指定用户整年 effort + 聚合 summary。
//
// userAccount 为空时取当前登录用户。
func (s *EffortCalendarService) GetEffortCalendar(userAccount string, year int) (*EffortCalendarResult, error) {
	// 1) 解析 user account → user.id
	account := userAccount
	if account == "" {
		account = s.client.GetAccount()
	}
	if account == "" {
		return nil, fmt.Errorf("未指定 user 且当前未登录")
	}

	userID, userName, err := s.resolveUserID(account)
	if err != nil {
		return nil, err
	}

	// 2) 拉全年 effort
	raw, err := s.client.GetEffortCalendar(context.Background(), userID, year)
	if err != nil {
		return nil, fmt.Errorf("拉取 effort 数据失败: %w", err)
	}

	// 3) 透传 entries（json 形状跟 client 一致）
	entries := make([]EffortCalendarEntry, 0, len(raw))
	for _, e := range raw {
		entries = append(entries, EffortCalendarEntry{
			ID:         e.ID,
			Title:      e.Title,
			ObjectType: e.ObjectType,
			Start:      e.Start,
			End:        e.End,
			Consumed:   e.Consumed,
			URL:        e.URL,
		})
	}

	// 4) 内存聚合 summary
	summary := summarizeEffortCalendar(entries, year)

	return &EffortCalendarResult{
		Entries:  entries,
		Summary:  summary,
		Year:     year,
		UserID:   userID,
		UserName: userName,
	}, nil
}

// resolveUserID 用 account 找到对应的 user.id。
//
// 受限 SSO 兼容性：受限实例上 /user-browse.json 经常返 deny（受权限限制），
// 这时 GetUsersAll 会报错。fallback 到 /user-profile.json 拿当前登录用户。
func (s *EffortCalendarService) resolveUserID(account string) (int, string, error) {
	users, usersErr := s.client.GetUsersAll()
	if usersErr == nil && len(users) > 0 {
		for _, u := range users {
			if u.Account == account {
				return u.ID, u.Realname, nil
			}
		}
		// users 列表拿到但没匹配上,继续 fallback 而不是直接返错,
		// 因为列表可能不完整(权限过滤)
	}
	// fallback: 直接 /user-profile.json 拿当前用户(几乎所有受限实例都允许)
	me, perr := s.client.GetCurrentUser()
	if perr == nil && me != nil && me.Account == account {
		return me.ID, me.Realname, nil
	}
	if usersErr != nil {
		return 0, "", fmt.Errorf("未找到用户 %s (GetUsersAll: %v; GetCurrentUser: %v)",
			account, usersErr, perr)
	}
	return 0, "", fmt.Errorf("未找到用户: %s", account)
}

// summarizeEffortCalendar 内存聚合：按月 / 按类型 / 全年统计。
func summarizeEffortCalendar(entries []EffortCalendarEntry, year int) *EffortCalendarSummary {
	summary := &EffortCalendarSummary{
		ByType: map[string]EffortCalendarTypeStat{},
	}

	// 按日聚合（key = YYYY-MM-DD），用来算 maxDaily / avgDaily / workDays
	byDate := map[string]float64{}
	for _, e := range entries {
		summary.TotalHours += e.Consumed
		summary.TotalRecords++

		byDate[e.Start] += e.Consumed

		// 按类型
		t := effortCalendarTypeKey(e.ObjectType)
		st := summary.ByType[t]
		st.Count++
		st.Hours += e.Consumed
		summary.ByType[t] = st
	}

	// 按日聚合 → 全年统计 + 月度统计
	monthMap := map[string]*EffortCalendarMonthStat{}
	for date, hours := range byDate {
		summary.WorkDays++
		if hours > summary.MaxDailyHours {
			summary.MaxDailyHours = hours
			summary.MaxDailyDate = date
		}
		month := date[:7]
		m := monthMap[month]
		if m == nil {
			m = &EffortCalendarMonthStat{Month: month}
			monthMap[month] = m
		}
		m.TotalHours += hours
		m.WorkDays++
	}
	// recordCount 按月单独算
	for _, e := range entries {
		month := e.Start[:7]
		m := monthMap[month]
		if m == nil {
			m = &EffortCalendarMonthStat{Month: month}
			monthMap[month] = m
		}
		m.RecordCount++
	}

	// 月份按字典序排列（YYYY-MM 字典序=时间序）
	for i := 1; i <= 12; i++ {
		monthKey := fmt.Sprintf("%d-%02d", year, i)
		if m, ok := monthMap[monthKey]; ok {
			summary.ByMonth = append(summary.ByMonth, *m)
		}
	}

	// 平均
	if summary.WorkDays > 0 {
		summary.AvgDailyHours = summary.TotalHours / float64(summary.WorkDays)
	}

	return summary
}

// effortCalendarTypeKey 把 ObjectType 标准化成稳定 key（空 = "other"）。
func effortCalendarTypeKey(t string) string {
	if t == "" {
		return "other"
	}
	return t
}

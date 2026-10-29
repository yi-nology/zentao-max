package zentao

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ---- CalendarEntry (工时日历) ----
//
// 工时日历（your-zentao-server.com/effort-calendar.html）背后实际拉的是
// `/effort-ajaxGetEfforts-{userID}-{year}.html` 这个 XHR 端点，返
// application/json，结构是顶层数组，字段就 6 个：
//   id, title, start, end, url, consumed
//
// title 形如 "[T]完善全景图文档" / "[B]xxx"，前缀表示工时关联的对象
// 类型（T=task / B=bug），本类型把这条隐含信息单独抽成 ObjectType
// 字段方便前端上色 / 分类聚合。
//
// Session 模式专用：Token 模式（标准禅道）走 /api.php/v1/efforts
// 等其他端点，逻辑不同，本期不做。

// CalendarEntry 工时日历一条记录
type CalendarEntry struct {
	ID         int     `json:"id"`
	Title      string  `json:"title"`
	ObjectType string  `json:"objectType"` // "task" / "bug"，解析 [T]/[B] 前缀
	Start      string  `json:"start"`     // YYYY-MM-DD
	End        string  `json:"end"`       // YYYY-MM-DD
	Consumed   float64 `json:"consumed"`  // 消耗工时（小时）
	URL        string  `json:"url"`       // 禅道详情页
}

// calendarRawEntry 是禅道端点返回的原始 shape。本地解析后转 CalendarEntry。
type calendarRawEntry struct {
	ID       int     `json:"id"`
	Title    string  `json:"title"`
	Start    string  `json:"start"`
	End      string  `json:"end"`
	URL      string  `json:"url"`
	Consumed float64 `json:"consumed"`
}

// parseCalendarObjectType 从 "[T]xxx" / "[B]xxx" 解析对象类型。
// 未知前缀返空字符串 + 原始 title，前端可按空当 fallback。
func ParseCalendarObjectType(title string) (string, string) {
	if !strings.HasPrefix(title, "[") {
		return "", title
	}
	idx := strings.Index(title, "]")
	if idx < 0 || idx > 4 { // [T] / [B] / [S] / [P] 这种单字符前缀
		return "", title
	}
	switch title[1:idx] {
	case "T":
		return "task", title[idx+1:]
	case "B":
		return "bug", title[idx+1:]
	case "S":
		return "story", title[idx+1:]
	case "P":
		return "project", title[idx+1:]
	default:
		return "", title
	}
}

// parseCalendarEntries 把禅道端点原始响应（数组 JSON）解析成 CalendarEntry 列表。
// 抽出来便于单测：直接喂 testdata/sample_effort_calendar.json 验证。
func parseCalendarEntries(data []byte) ([]CalendarEntry, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var raw []calendarRawEntry
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("解析 calendar 端点响应失败: %w", err)
	}
	out := make([]CalendarEntry, 0, len(raw))
	for _, r := range raw {
		objType, cleanTitle := ParseCalendarObjectType(r.Title)
		out = append(out, CalendarEntry{
			ID:         r.ID,
			Title:      cleanTitle,
			ObjectType: objType,
			Start:      r.Start,
			End:        r.End,
			Consumed:   r.Consumed,
			URL:        r.URL,
		})
	}
	return out, nil
}

// GetEffortCalendar 获取指定用户指定年份的全量 effort 记录（工时日历数据源）。
//
// 端点：GET /effort-ajaxGetEfforts-{userID}-{year}.html
// 认证：Session 模式（受限实例等禁用 REST API 的实例）
// 缓存：5min fresh / 1h stale（复用 sessionEntityEfforts 的 TTL）
//
// userID 是禅道 user.id（不是 account），调用方需要先 GetUsersAll 拿到 id 映射。
// 受限实例实测：单 user 一年数据 80KB / 300+ 条，前端按月切片渲染即可。
func (c *Client) GetEffortCalendar(ctx context.Context, userID, year int) ([]CalendarEntry, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("userID 必须大于 0")
	}
	if year < 2000 || year > 2100 {
		return nil, fmt.Errorf("year 超出合理范围: %d", year)
	}

	// 走 SessionCache，scope 带上 userID + year，避免不同用户/年份串数据
	scope := fmt.Sprintf("calendar:%d:%d", userID, year)
	loader := func(ctx context.Context) ([]byte, error) {
		path := fmt.Sprintf("/effort-ajaxGetEfforts-%d-%d.html", userID, year)
		// 端点实际返 application/json 但 content-type 标 text/html，
		// 而且响应是顶层 JSON 数组（不是双层 {status,data,md5} 包裹），
		// 必须用 GetRawJSON 跳过外层解包，直接读 body。
		body, err := c.GetRawJSON(ctx, path)
		if err != nil {
			// 致命会话错误（登录失效 / 权限拒绝）必须 wrap errDoNotServeStale，
			// 让 SessionCache 跳过 L2 stale fallback 直接把 err 透传，
			// 否则前端会拿到旧 stale 数据误以为"今天确实没工时"或"账号真没权限"。
			// 跟 client.go doSessionJSONCached 的 safeLoader 保持一致语义。
			if IsSessionAuthError(err) || IsSessionAccessDenied(err) {
				return nil, fmt.Errorf("%w: %w", errDoNotServeStale, err)
			}
			return nil, err
		}
		var raw []calendarRawEntry
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, fmt.Errorf("解析 effort-calendar 端点响应失败: %w; body=%s", err, truncate(string(body), 200))
		}
		out := make([]CalendarEntry, 0, len(raw))
		for _, r := range raw {
			objType, cleanTitle := ParseCalendarObjectType(r.Title)
			out = append(out, CalendarEntry{
				ID:         r.ID,
				Title:      cleanTitle,
				ObjectType: objType,
				Start:      r.Start,
				End:        r.End,
				Consumed:   r.Consumed,
				URL:        r.URL,
			})
		}
		// 二次序列化让 GetOrLoad 写入 L1/L2 cache 时是紧凑 JSON
		return json.Marshal(out)
	}

	res, err := c.cache.GetOrLoad(ctx, sessionEntityEfforts, scope, loader)
	if err != nil {
		return nil, err
	}
	if res == nil || len(res.Data) == 0 {
		return nil, nil
	}
	var entries []CalendarEntry
	if err := json.Unmarshal(res.Data, &entries); err != nil {
		return nil, fmt.Errorf("解析 calendar entries 失败: %w", err)
	}
	return entries, nil
}

// CurrentYear 返回当前年份（service 层 GetCurrentUserEffortCalendar 默认 year=0 用）。
func CurrentYear() int {
	return time.Now().Year()
}

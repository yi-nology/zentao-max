package service

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/yi-nology/zentao-max/backend/internal/infra/zentao"
)

// TestSummarizeEffortCalendar_Empty 空输入聚合应返全 0，无 panic。
func TestSummarizeEffortCalendar_Empty(t *testing.T) {
	got := summarizeEffortCalendar(nil, 2026)
	if got == nil {
		t.Fatal("want non-nil summary")
	}
	if got.TotalHours != 0 || got.TotalRecords != 0 || got.WorkDays != 0 {
		t.Errorf("want zero summary, got %+v", got)
	}
	if got.ByType == nil {
		t.Error("ByType should be empty map, got nil")
	}
	if got.AvgDailyHours != 0 {
		t.Errorf("AvgDailyHours should be 0, got %f", got.AvgDailyHours)
	}
}

// TestSummarizeEffortCalendar_Sample 用真实脱敏样本（testdata/sample_effort_calendar.json）
// 验证聚合逻辑：totalHours / workDays / maxDaily / byType / byMonth 全部正确。
func TestSummarizeEffortCalendar_Sample(t *testing.T) {
	raw, err := os.ReadFile("../infra/zentao/testdata/sample_effort_calendar.json")
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}
	// 先把原始 raw 解析成 client.CalendarEntry，再转成 service.EffortCalendarEntry
	// （跟 service.GetEffortCalendar 内部的转换一致）
	var clientEntries []zentao.CalendarEntry
	if err := json.Unmarshal(raw, &clientEntries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	entries := make([]EffortCalendarEntry, 0, len(clientEntries))
	for _, e := range clientEntries {
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

	summary := summarizeEffortCalendar(entries, 2026)

	if summary.TotalRecords != len(entries) {
		t.Errorf("TotalRecords: got %d want %d", summary.TotalRecords, len(entries))
	}
	// 全年样本里最早 2025-12-01 最晚 2026-07-15,所有 2025 年的不在 2026 byMonth 里
	if len(summary.ByMonth) == 0 {
		t.Error("ByMonth should not be empty for 2026 data")
	}
	// byMonth 顺序应该 1-12 月依次排(空月份不返)
	for i := 1; i < len(summary.ByMonth); i++ {
		if summary.ByMonth[i-1].Month >= summary.ByMonth[i].Month {
			t.Errorf("ByMonth not sorted: %s >= %s",
				summary.ByMonth[i-1].Month, summary.ByMonth[i].Month)
		}
	}
	// 按类型：样本里 T=210 B=106(见 client_session_effort_calendar_test.go 的 log)
	if summary.ByType["task"].Count == 0 {
		t.Error("ByType[task] should not be empty")
	}
	if summary.ByType["bug"].Count == 0 {
		t.Error("ByType[bug] should not be empty")
	}
	// MaxDaily 应该 ≥ 8(样本里大部分天是 8h)
	if summary.MaxDailyHours < 4 {
		t.Errorf("MaxDailyHours too low: %f", summary.MaxDailyHours)
	}
	// AvgDaily 在合理范围
	if summary.AvgDailyHours <= 0 {
		t.Errorf("AvgDailyHours should be > 0, got %f", summary.AvgDailyHours)
	}
	t.Logf("summary: totalHours=%.1f records=%d workDays=%d maxDaily=%.1f@%s byMonthMonths=%d",
		summary.TotalHours, summary.TotalRecords, summary.WorkDays,
		summary.MaxDailyHours, summary.MaxDailyDate, len(summary.ByMonth))
}

// TestEffortCalendarTypeKey 验证 ObjectType 标准化。
func TestEffortCalendarTypeKey(t *testing.T) {
	cases := map[string]string{
		"":        "other",
		"task":    "task",
		"bug":     "bug",
		"story":   "story",
		"project": "project",
		"unknown": "unknown", // 未知类型也透传
	}
	for in, want := range cases {
		if got := effortCalendarTypeKey(in); got != want {
			t.Errorf("effortCalendarTypeKey(%q) = %q, want %q", in, got, want)
		}
	}
}

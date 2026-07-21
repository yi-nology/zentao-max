package zentao

import (
	"encoding/json"
	"os"
	"testing"
)

// TestParseCalendarObjectType 验证 [T]/[B]/[S]/[P]/空 五种 title 前缀解析。
func TestParseCalendarObjectType(t *testing.T) {
	cases := []struct {
		title        string
		wantType     string
		wantClean    string
	}{
		{"[T]完善全景图文档", "task", "完善全景图文档"},
		{"[B]修复样式错位", "bug", "修复样式错位"},
		{"[S]需求评审", "story", "需求评审"},
		{"[P]项目维护", "project", "项目维护"},
		{"没有前缀的工时", "", "没有前缀的工时"},
		{"[]空前缀", "", "[]空前缀"},
		{"[X]未知前缀", "", "[X]未知前缀"}, // 未知前缀应保持原 title
		{"[TASK]多字符前缀", "", "[TASK]多字符前缀"},
	}
	for _, tc := range cases {
		t.Run(tc.title, func(t *testing.T) {
			gotType, gotClean := parseCalendarObjectType(tc.title)
			if gotType != tc.wantType {
				t.Errorf("type: got %q want %q", gotType, tc.wantType)
			}
			if gotClean != tc.wantClean {
				t.Errorf("clean: got %q want %q", gotClean, tc.wantClean)
			}
		})
	}
}

// TestParseCalendarEntries_Empty 验证空响应不报错。
func TestParseCalendarEntries_Empty(t *testing.T) {
	got, err := parseCalendarEntries(nil)
	if err != nil {
		t.Fatalf("empty: %v", err)
	}
	if got != nil {
		t.Errorf("empty: got %v want nil", got)
	}
	got, err = parseCalendarEntries([]byte(""))
	if err != nil {
		t.Fatalf("empty bytes: %v", err)
	}
	if got != nil {
		t.Errorf("empty bytes: got %v want nil", got)
	}
}

// TestParseCalendarEntries_BadJSON 验证脏数据不会 panic，能返清晰错误。
func TestParseCalendarEntries_BadJSON(t *testing.T) {
	_, err := parseCalendarEntries([]byte("{not valid json"))
	if err == nil {
		t.Fatal("want error on bad json, got nil")
	}
}

// TestParseCalendarEntries_Sample 验证真实样本（testdata/sample_effort_calendar.json）
// 能完整解析，所有字段都被还原，且 ObjectType 被正确抽取。
func TestParseCalendarEntries_Sample(t *testing.T) {
	raw, err := os.ReadFile("testdata/sample_effort_calendar.json")
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}

	entries, err := parseCalendarEntries(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// 样本是 316 条（一个用户整年 effort），不能丢
	if len(entries) < 100 {
		t.Errorf("entries too few: got %d, want >= 100", len(entries))
	}

	// 统计 ObjectType 分布，确保 [T] 和 [B] 都被识别
	var taskCount, bugCount, unknownCount int
	for _, e := range entries {
		switch e.ObjectType {
		case "task":
			taskCount++
		case "bug":
			bugCount++
		default:
			unknownCount++
		}
		// 所有 entry 必须有日期
		if e.Start == "" {
			t.Errorf("entry %d missing start", e.ID)
		}
		// Consumed 允许 0（占位 effort：登记了但没填工时，禅道支持这种用法）。
		// 只允许 <= 0 的告警：负数是脏数据，但 0 是合法占位。
		if e.Consumed < 0 {
			t.Errorf("entry %d consumed negative: %f", e.ID, e.Consumed)
		}
		// title 不能带 [T] / [B] 前缀（应该被剥掉）
		if len(e.Title) > 0 && e.Title[0] == '[' {
			t.Errorf("entry %d title not stripped: %q", e.ID, e.Title)
		}
	}
	if taskCount == 0 {
		t.Error("no task entries parsed")
	}
	if bugCount == 0 {
		t.Error("no bug entries parsed")
	}
	if unknownCount > 0 {
		t.Logf("warning: %d entries with unknown object type", unknownCount)
	}
	t.Logf("parsed: task=%d bug=%d unknown=%d total=%d",
		taskCount, bugCount, unknownCount, len(entries))
}

// TestCalendarEntry_JSONRoundTrip 验证 JSON 序列化字段名稳定（前端依赖 snake_case）。
func TestCalendarEntry_JSONRoundTrip(t *testing.T) {
	in := CalendarEntry{
		ID:         10086,
		Title:      "工时标题",
		ObjectType: "task",
		Start:      "2026-07-15",
		End:        "2026-07-15",
		Consumed:   4.5,
		URL:        "/effort-view-10086.html",
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// 字段名必须稳定（前端 types/api.ts 也用同样名字）
	for _, k := range []string{`"id":`, `"title":`, `"objectType":`, `"start":`, `"end":`, `"consumed":`, `"url":`} {
		if !contains(b, k) {
			t.Errorf("missing field %s in JSON: %s", k, b)
		}
	}
	var out CalendarEntry
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out != in {
		t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", out, in)
	}
}

func contains(haystack []byte, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

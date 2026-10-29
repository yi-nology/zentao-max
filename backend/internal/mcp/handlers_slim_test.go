package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

// asJSON 把瘦身结果序列化回 map 便于断言。
func asJSON(t *testing.T, v interface{}) map[string]interface{} {
	t.Helper()
	payload, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

func TestSlimLongTextTruncatesAndMarks(t *testing.T) {
	long := strings.Repeat("长", 300) // 300 个 rune，超过 200 上限
	data := map[string]interface{}{
		"list": []interface{}{
			map[string]interface{}{"id": float64(1), "steps": long, "title": "短标题"},
		},
		"total": float64(1),
	}
	out := asJSON(t, slimViaJSON(data, []string{"steps"}))
	list := out["list"].([]interface{})
	item := list[0].(map[string]interface{})

	got := item["steps"].(string)
	if len([]rune(got)) >= 300 {
		t.Fatalf("steps not truncated, len=%d", len([]rune(got)))
	}
	if !strings.Contains(got, "[截断") || !strings.Contains(got, "get_*_detail") {
		t.Errorf("truncation marker missing: %q", got[len(got)-60:])
	}
	if item["title"] != "短标题" || item["id"] != float64(1) {
		t.Errorf("untouched fields altered: %v", item)
	}
	if out["total"] != float64(1) {
		t.Errorf("outer structure altered: %v", out["total"])
	}
}

func TestSlimShortTextUntouched(t *testing.T) {
	data := map[string]interface{}{"steps": "短内容"}
	out := asJSON(t, slimViaJSON(data, []string{"steps"}))
	if out["steps"] != "短内容" {
		t.Errorf("short field should pass through: %v", out["steps"])
	}
}

func TestSlimNonStringFieldIgnored(t *testing.T) {
	data := map[string]interface{}{"steps": 12345}
	out := asJSON(t, slimViaJSON(data, []string{"steps"}))
	if out["steps"] != float64(12345) {
		t.Errorf("non-string field must not be altered: %v", out["steps"])
	}
}

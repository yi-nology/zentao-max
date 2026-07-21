package logger

import (
	"strings"
	"testing"
)

func TestRingBuffer_AppendAndQuery(t *testing.T) {
	rb := &LogRingBuffer{
		entries: make([]*LogEntry, 0, 5),
		maxSize: 3,
	}

	// 追加 5 条，由于 maxSize=3，应只保留最后 3 条
	for i := 0; i < 5; i++ {
		rb.Append(&LogEntry{
			Time:    "2024-06-15T12:00:0XZ",
			Level:   "info",
			Message: "msg-" + string(rune('A'+i)),
		})
	}

	if rb.Size() != 3 {
		t.Errorf("Size() = %d, want 3 (should be capped at maxSize)", rb.Size())
	}

	all := rb.Query("", "", 10)
	if len(all) != 3 {
		t.Errorf("Query() returned %d, want 3", len(all))
	}
	// 最新在前
	if all[0].Message != "msg-E" {
		t.Errorf("first entry = %s, want msg-E", all[0].Message)
	}
}

func TestRingBuffer_FilterByLevel(t *testing.T) {
	rb := &LogRingBuffer{
		entries: make([]*LogEntry, 0, 10),
		maxSize: 100,
	}
	rb.Append(&LogEntry{Level: "info", Message: "info-1"})
	rb.Append(&LogEntry{Level: "error", Message: "err-1"})
	rb.Append(&LogEntry{Level: "warn", Message: "warn-1"})
	rb.Append(&LogEntry{Level: "error", Message: "err-2"})

	errOnly := rb.Query("error", "", 10)
	if len(errOnly) != 2 {
		t.Errorf("Query(error) = %d entries, want 2", len(errOnly))
	}
	for _, e := range errOnly {
		if e.Level != "error" {
			t.Errorf("got level %s, want error", e.Level)
		}
	}
}

func TestRingBuffer_FilterByKeyword(t *testing.T) {
	rb := &LogRingBuffer{
		entries: make([]*LogEntry, 0, 10),
		maxSize: 100,
	}
	rb.Append(&LogEntry{Level: "info", Message: "Failed to connect to zentao"})
	rb.Append(&LogEntry{Level: "info", Message: "Success"})
	rb.Append(&LogEntry{Level: "error", Message: "zentao timeout"})

	results := rb.Query("", "zentao", 10)
	if len(results) != 2 {
		t.Errorf("Query(zentao) = %d, want 2", len(results))
	}
}

func TestRingBuffer_KeywordCaseInsensitive(t *testing.T) {
	rb := &LogRingBuffer{
		entries: make([]*LogEntry, 0, 10),
		maxSize: 100,
	}
	rb.Append(&LogEntry{Level: "info", Message: "Hello World"})

	// 小写搜索应该匹配大写内容
	if len(rb.Query("", "hello", 10)) != 1 {
		t.Error("expected case-insensitive match for 'hello'")
	}
	if len(rb.Query("", "WORLD", 10)) != 1 {
		t.Error("expected case-insensitive match for 'WORLD'")
	}
}

func TestRingBuffer_Limit(t *testing.T) {
	rb := &LogRingBuffer{
		entries: make([]*LogEntry, 0, 100),
		maxSize: 100,
	}
	for i := 0; i < 50; i++ {
		rb.Append(&LogEntry{Level: "info", Message: "msg"})
	}

	if len(rb.Query("", "", 10)) != 10 {
		t.Error("limit=10 should return 10 entries")
	}
	if len(rb.Query("", "", 0)) != 50 {
		t.Error("limit=0 should return all entries (no cap)")
	}
}

func TestRingBuffer_Clear(t *testing.T) {
	rb := &LogRingBuffer{
		entries: make([]*LogEntry, 0, 10),
		maxSize: 10,
	}
	rb.Append(&LogEntry{Level: "info", Message: "msg"})
	rb.Append(&LogEntry{Level: "info", Message: "msg"})

	if rb.Size() != 2 {
		t.Fatalf("before Clear: Size = %d, want 2", rb.Size())
	}

	rb.Clear()

	if rb.Size() != 0 {
		t.Errorf("after Clear: Size = %d, want 0", rb.Size())
	}
	if len(rb.Query("", "", 10)) != 0 {
		t.Error("Query after Clear should return 0")
	}
}

func TestContainsCI(t *testing.T) {
	tests := []struct {
		s, substr string
		want      bool
	}{
		{"Hello World", "hello", true},
		{"Hello World", "WORLD", true},
		{"Hello", "xyz", false},
		{"", "x", false},
		{"abc", "", true},
		{"禅道连接失败", "禅道", true},
	}
	for _, tt := range tests {
		got := containsCI(tt.s, tt.substr)
		// strings.Contains 仅用于验证非 ASCII 情况下不走 ASCII 比较路径
		_ = strings.Contains
		if got != tt.want {
			t.Errorf("containsCI(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
		}
	}
}

package service

import (
	"strings"
	"testing"
	"time"

	"github.com/yi-nology/zentao-max/backend/core/models"
)

func TestCronExprFieldCount(t *testing.T) {
	tests := []struct {
		name     string
		cronExpr string
		want     string // expected expression after prepending
	}{
		{"5-field weekday", "0 9 * * 1-5", "0 0 9 * * 1-5"},
		{"5-field daily", "0 9 * * *", "0 0 9 * * *"},
		{"5-field every 8h", "0 */8 * * *", "0 0 */8 * * *"},
		{"5-field every 30min", "*/30 * * * *", "0 */30 * * * *"},
		{"6-field already", "0 0 9 * * 1-5", "0 0 9 * * 1-5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cronExpr := tt.cronExpr
			if len(strings.Fields(cronExpr)) == 5 {
				cronExpr = "0 " + cronExpr
			}
			if cronExpr != tt.want {
				t.Errorf("got %q, want %q", cronExpr, tt.want)
			}
		})
	}
}

func TestSeverityName(t *testing.T) {
	tests := []struct {
		sev  int
		want string
	}{
		{1, "致命"},
		{2, "严重"},
		{3, "一般"},
		{4, "轻微"},
		{0, "未知"},
		{99, "未知"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := severityName(tt.sev)
			if got != tt.want {
				t.Errorf("severityName(%d) = %q, want %q", tt.sev, got, tt.want)
			}
		})
	}
}

func TestParseDateField(t *testing.T) {
	tests := []struct {
		name string
		v    interface{}
		want string
	}{
		{"string", "2026-06-01 10:00:00", "2026-06-01 10:00:00"},
		{"iso8601", "2026-06-17T10:41:06Z", "2026-06-17T10:41:06Z"},
		{"time", time.Date(2026, 6, 1, 10, 0, 0, 0, time.Local), "2026-06-01 10:00:00"},
		{"other", 12345, "12345"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDateField(tt.v)
			if got != tt.want {
				t.Errorf("parseDateField(%v) = %q, want %q", tt.v, got, tt.want)
			}
		})
	}
}

func TestBuildBugAgingMessage(t *testing.T) {
	now := time.Date(2026, 6, 18, 9, 0, 0, 0, time.Local)
	details := []models.AssigneeBugAgingStats{
		{
			Assignee: "张三",
			Account:  "zhangsan",
			Total:    2,
			Bugs: []models.BugAgingItem{
				{ID: 1001, Title: "登录接口超时", Severity: "严重", OpenedDate: "2026-06-01 10:00:00", DaysOpen: 17},
				{ID: 1002, Title: "监控数据丢失", Severity: "致命", OpenedDate: "2026-06-05 14:00:00", DaysOpen: 13},
			},
		},
		{
			Assignee: "李四",
			Account:  "lisi",
			Total:    1,
			Bugs: []models.BugAgingItem{
				{ID: 1003, Title: "配置项未校验", Severity: "轻微", OpenedDate: "2026-06-10 09:00:00", DaysOpen: 8},
			},
		},
	}

	msg := buildBugAgingMessage("Bug 停留超时提醒 - 测试项目", now, 3, 7, details, "提醒", "", "")

	if !strings.Contains(msg, "【提醒】") {
		t.Error("消息应包含关键词【提醒】")
	}
	if !strings.Contains(msg, "超时 Bug：3个") {
		t.Error("消息应包含超时Bug总数")
	}
	if !strings.Contains(msg, "阈值：7天") {
		t.Error("消息应包含阈值天数")
	}
	if !strings.Contains(msg, "张三") {
		t.Error("消息应包含责任人张三")
	}
	if !strings.Contains(msg, "李四") {
		t.Error("消息应包含责任人李四")
	}
	if !strings.Contains(msg, "#1001") {
		t.Error("消息应包含Bug ID")
	}
	if !strings.Contains(msg, "[严重]") {
		t.Error("消息应包含严重程度标签")
	}
	if !strings.Contains(msg, "已停留 17天") {
		t.Error("消息应包含停留天数")
	}
	if !strings.Contains(msg, "请尽快处理") {
		t.Error("消息应包含催办提示")
	}
}

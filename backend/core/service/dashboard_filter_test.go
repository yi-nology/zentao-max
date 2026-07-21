package service

import (
	"testing"

	"github.com/yi-nology/common/biz/zentao"
)

func TestIsDateInRange(t *testing.T) {
	tests := []struct {
		name      string
		rawDate   string
		startDate string
		endDate   string
		want      bool
	}{
		{
			name:    "空日期字符串返回false",
			rawDate: "",
			want:    false,
		},
		{
			name:    "无范围限制返回true",
			rawDate: "2024-06-01 10:00:00",
			want:    true,
		},
		{
			name:      "日期在范围内",
			rawDate:   "2024-06-15 12:00:00",
			startDate: "2024-06-01",
			endDate:   "2024-06-30",
			want:      true,
		},
		{
			name:      "日期早于起始",
			rawDate:   "2024-05-15 12:00:00",
			startDate: "2024-06-01",
			endDate:   "2024-06-30",
			want:      false,
		},
		{
			name:      "日期晚于截止",
			rawDate:   "2024-07-15 12:00:00",
			startDate: "2024-06-01",
			endDate:   "2024-06-30",
			want:      false,
		},
		{
			name:      "仅起始日期，恰好等于起始",
			rawDate:   "2024-06-01 00:00:00",
			startDate: "2024-06-01",
			want:      true,
		},
		{
			name:    "仅截止日期，无起始",
			rawDate: "2024-06-15",
			endDate: "2024-06-30",
			want:    true,
		},
		{
			name:      "仅日期格式(无时间)在范围内",
			rawDate:   "2024-06-15",
			startDate: "2024-06-01",
			endDate:   "2024-06-30",
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDateInRange(tt.rawDate, tt.startDate, tt.endDate)
			if got != tt.want {
				t.Errorf("isDateInRange(%q, %q, %q) = %v, want %v",
					tt.rawDate, tt.startDate, tt.endDate, got, tt.want)
			}
		})
	}
}

func TestFilterBugsByDateRange(t *testing.T) {
	bugs := []zentao.Bug{
		{ID: 1, OpenedDate: "2024-06-01 10:00:00"},
		{ID: 2, OpenedDate: "2024-06-15 10:00:00"},
		{ID: 3, OpenedDate: "2024-07-01 10:00:00"},
		{ID: 4, OpenedDate: nil},
	}

	t.Run("无范围参数返回全部", func(t *testing.T) {
		got := filterBugsByDateRange(bugs, "", "")
		if len(got) != len(bugs) {
			t.Errorf("want %d, got %d", len(bugs), len(got))
		}
	})

	t.Run("6月份范围应该返回2条", func(t *testing.T) {
		got := filterBugsByDateRange(bugs, "2024-06-01", "2024-06-30")
		if len(got) != 2 {
			t.Errorf("want 2 bugs in June, got %d", len(got))
		}
	})

	t.Run("空列表应该返回空", func(t *testing.T) {
		got := filterBugsByDateRange([]zentao.Bug{}, "2024-06-01", "2024-06-30")
		if len(got) != 0 {
			t.Errorf("want 0, got %d", len(got))
		}
	})
}

func TestFilterStoriesByDateRange(t *testing.T) {
	stories := []zentao.Story{
		{ID: 1, OpenedDate: "2024-06-01"},
		{ID: 2, OpenedDate: "2024-07-01"},
	}

	got := filterStoriesByDateRange(stories, "2024-06-01", "2024-06-30")
	if len(got) != 1 || got[0].ID != 1 {
		t.Errorf("want story id=1, got %v", got)
	}
}

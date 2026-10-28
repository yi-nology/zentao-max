package service

import (
	"testing"

	zentaosdk "github.com/yi-nology/zentao-max/backend/internal/infra/zentao/sdk"

	"github.com/yi-nology/zentao-max/backend/pkg/utils"
	"github.com/yi-nology/zentao-max/backend/internal/vo"
)

func createMockBugs() []zentaosdk.Bug {
	return []zentaosdk.Bug{
		{
			ID:         1,
			Project:    10,
			Product:    100,
			Title:      "Bug 1",
			Status:     "active",
			OpenedDate: "2024-01-15 10:00:00",
			AssignedTo: zentaosdk.UserRef{Account: "user1", Realname: "User 1"},
			OpenedBy:   zentaosdk.UserRef{Account: "user2", Realname: "User 2"},
		},
		{
			ID:         2,
			Project:    10,
			Product:    100,
			Title:      "Bug 2",
			Status:     "resolved",
			OpenedDate: "2024-01-16 11:00:00",
			AssignedTo: zentaosdk.UserRef{Account: "user1", Realname: "User 1"},
			OpenedBy:   zentaosdk.UserRef{Account: "user3", Realname: "User 3"},
		},
		{
			ID:         3,
			Project:    11,
			Product:    100,
			Title:      "Bug 3",
			Status:     "active",
			OpenedDate: "2024-01-17 12:00:00",
			AssignedTo: zentaosdk.UserRef{Account: "user2", Realname: "User 2"},
			OpenedBy:   zentaosdk.UserRef{Account: "user1", Realname: "User 1"},
		},
	}
}

// TestBugService_ConvertToVO tests the VO conversion logic
func TestBugService_ConvertToVO(t *testing.T) {
	service := &BugService{client: nil}

	bugs := createMockBugs()
	vos := service.convertToVO(bugs)

	if len(vos) != 3 {
		t.Errorf("expected 3 VOs, got %d", len(vos))
	}

	if vos[0].ID != 1 {
		t.Errorf("expected first VO ID=1, got %d", vos[0].ID)
	}
	if vos[0].Title != "Bug 1" {
		t.Errorf("expected first VO Title='Bug 1', got '%s'", vos[0].Title)
	}
	if vos[0].Status != "active" {
		t.Errorf("expected first VO Status='active', got '%s'", vos[0].Status)
	}
}

// TestBugService_ConvertToVO_Empty tests empty slice conversion
func TestBugService_ConvertToVO_Empty(t *testing.T) {
	service := &BugService{client: nil}

	vos := service.convertToVO([]zentaosdk.Bug{})

	if len(vos) != 0 {
		t.Errorf("expected 0 VOs, got %d", len(vos))
	}
}

// TestBugService_DateFilterLogic tests the date filter logic used in GetBugs
func TestBugService_DateFilterLogic(t *testing.T) {
	bugs := createMockBugs()

	tests := []struct {
		name          string
		startDate     string
		endDate       string
		specificDate  string
		expectedCount int
	}{
		{
			name:          "no date filter",
			expectedCount: 3,
		},
		{
			name:          "filter by date range",
			startDate:     "2024-01-15",
			endDate:       "2024-01-16",
			expectedCount: 2,
		},
		{
			name:          "filter by specific date",
			specificDate:  "2024-01-17",
			expectedCount: 1,
		},
		{
			name:          "filter with no matches",
			startDate:     "2025-01-01",
			endDate:       "2025-01-31",
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chainFilter := utils.NewChainFilter(bugs)

			if tt.startDate != "" || tt.endDate != "" || tt.specificDate != "" {
				chainFilter = chainFilter.Filter(func(item zentaosdk.Bug) bool {
					filtered := utils.FilterByDateRangeOrSpecific(
						[]zentaosdk.Bug{item},
						tt.startDate,
						tt.endDate,
						tt.specificDate,
						func(b zentaosdk.Bug) string { s, _ := b.OpenedDate.(string); return s },
					)
					return len(filtered) > 0
				})
			}

			if chainFilter.Count() != tt.expectedCount {
				t.Errorf("expected %d bugs, got %d", tt.expectedCount, chainFilter.Count())
			}
		})
	}
}

// TestBugService_Pagination tests the pagination logic
func TestBugService_Pagination(t *testing.T) {
	bugs := createMockBugs()

	tests := []struct {
		name          string
		page          int
		pageSize      int
		expectedLen   int
		expectedTotal int
	}{
		{
			name:          "page 1 size 2",
			page:          1,
			pageSize:      2,
			expectedLen:   2,
			expectedTotal: 3,
		},
		{
			name:          "page 2 size 2",
			page:          2,
			pageSize:      2,
			expectedLen:   1,
			expectedTotal: 3,
		},
		{
			name:          "page 1 size 10",
			page:          1,
			pageSize:      10,
			expectedLen:   3,
			expectedTotal: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chainFilter := utils.NewChainFilter(bugs)
			total := chainFilter.Count()
			paged := chainFilter.Paginate(tt.page, tt.pageSize).Result()

			if total != tt.expectedTotal {
				t.Errorf("expected total=%d, got %d", tt.expectedTotal, total)
			}
			if len(paged) != tt.expectedLen {
				t.Errorf("expected len=%d, got %d", tt.expectedLen, len(paged))
			}
		})
	}
}

// TestBugService_VOTypes tests that VO types are correctly mapped
func TestBugService_VOTypes(t *testing.T) {
	service := &BugService{client: nil}

	bug := zentaosdk.Bug{
		ID:         42,
		Project:    10,
		Product:    100,
		Title:      "Test Bug",
		Status:     "active",
		OpenedDate: "2024-01-15 10:00:00",
		AssignedTo: zentaosdk.UserRef{Account: "user1", Realname: "User 1"},
		OpenedBy:   zentaosdk.UserRef{Account: "user2", Realname: "User 2"},
	}

	vos := service.convertToVO([]zentaosdk.Bug{bug})
	if len(vos) != 1 {
		t.Fatalf("expected 1 VO, got %d", len(vos))
	}

	voItem := vos[0]
	if voItem.ID != 42 {
		t.Errorf("expected VO ID=42, got %d", voItem.ID)
	}
	if voItem.Title != "Test Bug" {
		t.Errorf("expected VO Title='Test Bug', got '%s'", voItem.Title)
	}
	if voItem.Status != "active" {
		t.Errorf("expected VO Status='active', got '%s'", voItem.Status)
	}

	// Verify type assertion works
	var _ vo.BugVO = voItem
}

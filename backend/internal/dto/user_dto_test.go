package dto

import (
	"math"
	"testing"
)

// TestUserQueryDTO_Validate issue #11：/users 手写解析绕过 DTO 校验导致 page 无上界；
// 改走 DTO 后 page/pageSize 必须被钳到安全区间
func TestUserQueryDTO_Validate(t *testing.T) {
	tests := []struct {
		name             string
		page             int
		pageSize         int
		expectedPage     int
		expectedPageSize int
	}{
		{name: "默认值", page: 0, pageSize: 0, expectedPage: 1, expectedPageSize: 20},
		{name: "负数钳到默认", page: -5, pageSize: -1, expectedPage: 1, expectedPageSize: 20},
		{name: "正常值透传", page: 3, pageSize: 50, expectedPage: 3, expectedPageSize: 50},
		{name: "pageSize超上限钳到100", page: 1, pageSize: 10000, expectedPage: 1, expectedPageSize: MaxPageSize},
		{name: "page超上限钳到MaxPage", page: math.MaxInt64, pageSize: 20, expectedPage: MaxPage, expectedPageSize: 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &UserQueryDTO{Page: tt.page, PageSize: tt.pageSize}
			if err := d.Validate(); err != nil {
				t.Fatalf("Validate() err = %v", err)
			}
			if d.Page != tt.expectedPage {
				t.Errorf("Page = %d, want %d", d.Page, tt.expectedPage)
			}
			if d.PageSize != tt.expectedPageSize {
				t.Errorf("PageSize = %d, want %d", d.PageSize, tt.expectedPageSize)
			}
		})
	}
}

package utils

import (
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

func TestParsePagination(t *testing.T) {
	tests := []struct {
		name             string
		queryParams      map[string]string
		expectedPage     int
		expectedPageSize int
	}{
		{
			name:             "默认值",
			queryParams:      map[string]string{},
			expectedPage:     DefaultPage,
			expectedPageSize: DefaultPageSize,
		},
		{
			name: "自定义page和pageSize",
			queryParams: map[string]string{
				"page":     "2",
				"pageSize": "50",
			},
			expectedPage:     2,
			expectedPageSize: 50,
		},
		{
			name: "page参数无效",
			queryParams: map[string]string{
				"page":     "invalid",
				"pageSize": "20",
			},
			expectedPage:     DefaultPage,
			expectedPageSize: 20,
		},
		{
			name: "pageSize参数无效",
			queryParams: map[string]string{
				"page":     "1",
				"pageSize": "invalid",
			},
			expectedPage:     1,
			expectedPageSize: DefaultPageSize,
		},
		{
			name: "pageSize超过最大值",
			queryParams: map[string]string{
				"page":     "1",
				"pageSize": "200",
			},
			expectedPage:     1,
			expectedPageSize: DefaultPageSize,
		},
		{
			name: "pageSize小于1",
			queryParams: map[string]string{
				"page":     "1",
				"pageSize": "0",
			},
			expectedPage:     1,
			expectedPageSize: DefaultPageSize,
		},
		{
			name: "page小于1",
			queryParams: map[string]string{
				"page":     "0",
				"pageSize": "20",
			},
			expectedPage:     DefaultPage,
			expectedPageSize: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := app.NewContext(0)
			query := ""
			for k, v := range tt.queryParams {
				if query != "" {
					query += "&"
				}
				query += k + "=" + v
			}
			c.Request.SetRequestURI("/test?" + query)

			params := ParsePagination(c)

			assert.DeepEqual(t, tt.expectedPage, params.Page)
			assert.DeepEqual(t, tt.expectedPageSize, params.PageSize)
		})
	}
}

func TestPaginate(t *testing.T) {
	tests := []struct {
		name          string
		total         int
		page          int
		pageSize      int
		expectedStart int
		expectedEnd   int
	}{
		{
			name:          "第一页",
			total:         100,
			page:          1,
			pageSize:      20,
			expectedStart: 0,
			expectedEnd:   20,
		},
		{
			name:          "中间页",
			total:         100,
			page:          3,
			pageSize:      20,
			expectedStart: 40,
			expectedEnd:   60,
		},
		{
			name:          "最后一页",
			total:         100,
			page:          5,
			pageSize:      20,
			expectedStart: 80,
			expectedEnd:   100,
		},
		{
			name:          "超出范围",
			total:         100,
			page:          10,
			pageSize:      20,
			expectedStart: 0,
			expectedEnd:   0,
		},
		{
			name:          "空列表",
			total:         0,
			page:          1,
			pageSize:      20,
			expectedStart: 0,
			expectedEnd:   0,
		},
		{
			name:          "部分页",
			total:         95,
			page:          5,
			pageSize:      20,
			expectedStart: 80,
			expectedEnd:   95,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := Paginate(tt.total, tt.page, tt.pageSize)

			assert.DeepEqual(t, tt.expectedStart, start)
			assert.DeepEqual(t, tt.expectedEnd, end)
		})
	}
}

func TestPaginateSlice(t *testing.T) {
	tests := []struct {
		name     string
		slice    []int
		page     int
		pageSize int
		expected []int
	}{
		{
			name:     "第一页",
			slice:    []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			page:     1,
			pageSize: 3,
			expected: []int{1, 2, 3},
		},
		{
			name:     "中间页",
			slice:    []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			page:     2,
			pageSize: 3,
			expected: []int{4, 5, 6},
		},
		{
			name:     "最后一页",
			slice:    []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			page:     4,
			pageSize: 3,
			expected: []int{10},
		},
		{
			name:     "超出范围",
			slice:    []int{1, 2, 3},
			page:     10,
			pageSize: 3,
			expected: []int{},
		},
		{
			name:     "空切片",
			slice:    []int{},
			page:     1,
			pageSize: 3,
			expected: []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PaginateSlice(tt.slice, tt.page, tt.pageSize)
			assert.DeepEqual(t, tt.expected, result)
		})
	}
}

func TestGetPagination(t *testing.T) {
	t.Run("从context获取", func(t *testing.T) {
		c := app.NewContext(0)
		params := PaginationParams{
			Page:     3,
			PageSize: 30,
		}
		c.Set("pagination", params)

		result := GetPagination(c)

		assert.DeepEqual(t, params.Page, result.Page)
		assert.DeepEqual(t, params.PageSize, result.PageSize)
	})

	t.Run("context中没有值", func(t *testing.T) {
		c := app.NewContext(0)
		result := GetPagination(c)

		assert.DeepEqual(t, DefaultPage, result.Page)
		assert.DeepEqual(t, DefaultPageSize, result.PageSize)
	})
}

package mcp

import "testing"

// pageParams 回归：MCP 列表工具必须尊重调用方 page/pageSize，而不是
// 硬编码第一页 100 条（执行 9151 有 296 条任务，客户端此前拿不到第 2 页）。
func TestPageParams(t *testing.T) {
	cases := []struct {
		name     string
		params   map[string]interface{}
		def      int
		wantPage int
		wantSize int
	}{
		{"defaults", map[string]interface{}{}, 100, 1, 100},
		{"explicit numbers", map[string]interface{}{"page": float64(3), "pageSize": float64(500)}, 100, 3, 500},
		{"string values", map[string]interface{}{"page": "2", "pageSize": "50"}, 100, 2, 50},
		{"pageSize capped", map[string]interface{}{"pageSize": float64(99999)}, 100, 1, 2000},
		{"invalid values", map[string]interface{}{"page": float64(0), "pageSize": float64(-5)}, 100, 1, 100},
		{"big default kept", map[string]interface{}{}, 2000, 1, 2000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, size := pageParams(tc.params, tc.def)
			if page != tc.wantPage || size != tc.wantSize {
				t.Errorf("pageParams(%v, %d) = (%d,%d), want (%d,%d)",
					tc.params, tc.def, page, size, tc.wantPage, tc.wantSize)
			}
		})
	}
}

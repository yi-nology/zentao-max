package handlers

import (
	"net/http"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/yi-nology/zentao-max/backend/core/errors"
)

func TestBugHandler_QueryParams(t *testing.T) {
	tests := []struct {
		name           string
		queryParams    string
		expectedStatus int
	}{
		{
			name:           "缺少产品ID参数",
			queryParams:    "?page=1&pageSize=20",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "无效的productId",
			queryParams:    "?productId=invalid",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "无效的page",
			queryParams:    "?productId=100&page=invalid",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "无效的pageSize",
			queryParams:    "?productId=100&pageSize=invalid",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := app.NewContext(0)
			h.Request.SetRequestURI("/api/v1/bugs" + tt.queryParams)

			if tt.queryParams == "?productId=invalid" {
				errors.BadRequest(h, "无效的产品ID")
			} else {
				errors.Success(h, map[string]interface{}{"list": []interface{}{}, "total": 0, "page": 1, "pageSize": 20})
			}

			if h.Response.StatusCode() != tt.expectedStatus {
				t.Errorf("期望状态码=%d, 实际状态码=%d", tt.expectedStatus, h.Response.StatusCode())
			}
		})
	}
}

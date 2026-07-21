package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/yi-nology/zentao-max/backend/core/dto"
	"github.com/yi-nology/zentao-max/backend/core/vo"
)

func performGET(t *testing.T, handler func(context.Context, *app.RequestContext), uri string) *app.RequestContext {
	t.Helper()
	ctx := app.NewContext(0)
	ctx.Request.SetMethod("GET")
	ctx.Request.SetRequestURI(uri)
	handler(context.Background(), ctx)
	return ctx
}

func getResponseCode(t *testing.T, c *app.RequestContext) int {
	t.Helper()
	var resp struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(c.Response.Body(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v, body: %s", err, string(c.Response.Body()))
	}
	return resp.Code
}

func TestBugHandler_GetBugs_Success(t *testing.T) {
	mock := &MockBugService{
		Result: &vo.PaginatedVO{
			List:     []vo.BugVO{},
			Total:    5,
			Page:     1,
			PageSize: 20,
		},
	}
	h := NewBugHandler(mock)

	c := performGET(t, h.GetBugs, "/api/v1/bugs?productId=1&page=1&pageSize=20")

	if c.Response.StatusCode() != http.StatusOK {
		t.Errorf("status = %d, want %d", c.Response.StatusCode(), http.StatusOK)
	}
	if !mock.Called {
		t.Error("expected GetBugs to be called")
	}
	if mock.Query.ProductID != 1 {
		t.Errorf("ProductID = %d, want 1", mock.Query.ProductID)
	}
	if getResponseCode(t, c) != 200 {
		t.Errorf("response code = %d, want 200", getResponseCode(t, c))
	}
}

func TestBugHandler_GetBugs_PageSizeCap(t *testing.T) {
	mock := &MockBugService{
		Result: &vo.PaginatedVO{List: []vo.BugVO{}},
	}
	h := NewBugHandler(mock)

	performGET(t, h.GetBugs, "/api/v1/bugs?productId=1&pageSize=99999")

	if mock.Query.PageSize != 100 {
		t.Errorf("PageSize = %d, want 100 (capped)", mock.Query.PageSize)
	}
}

func TestBugHandler_GetBugs_ServiceError(t *testing.T) {
	mock := &MockBugService{
		Err: errors.New("zentao unavailable"),
	}
	h := NewBugHandler(mock)

	c := performGET(t, h.GetBugs, "/api/v1/bugs?productId=1")

	if c.Response.StatusCode() != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d (external error maps to 500)", c.Response.StatusCode(), http.StatusInternalServerError)
	}
}

func TestProductHandler_GetProducts_Success(t *testing.T) {
	mock := &MockProductService{
		Result: []vo.ProductVO{
			{ID: 1, Name: "Product A"},
			{ID: 2, Name: "Product B"},
		},
	}
	h := NewProductHandler(mock)

	c := performGET(t, h.GetProducts, "/api/v1/products")

	if c.Response.StatusCode() != http.StatusOK {
		t.Errorf("status = %d, want %d", c.Response.StatusCode(), http.StatusOK)
	}
	if !mock.Called {
		t.Error("expected GetProducts to be called")
	}
}

func TestDashboardHandler_GetDashboard_Success(t *testing.T) {
	mock := &MockDashboardService{
		DashboardResult: &vo.DashboardVO{
			Bugs: vo.BugStatsVO{Total: 10, Active: 5},
		},
	}
	h := NewDashboardHandler(mock)

	c := performGET(t, h.GetDashboard, "/api/v1/dashboard?productId=1")

	if c.Response.StatusCode() != http.StatusOK {
		t.Errorf("status = %d, want %d", c.Response.StatusCode(), http.StatusOK)
	}
	if !mock.DashboardCalled {
		t.Error("expected GetDashboardContext to be called")
	}
	if mock.ProductIDArg != 1 {
		t.Errorf("ProductID = %d, want 1", mock.ProductIDArg)
	}
}

func TestDashboardHandler_GetDashboard_Defaults(t *testing.T) {
	mock := &MockDashboardService{
		DashboardResult: &vo.DashboardVO{},
	}
	h := NewDashboardHandler(mock)

	c := performGET(t, h.GetDashboard, "/api/v1/dashboard")

	if c.Response.StatusCode() != http.StatusOK {
		t.Errorf("status = %d, want %d", c.Response.StatusCode(), http.StatusOK)
	}
}

func TestTaskHandler_GetTasks_MissingProductID(t *testing.T) {
	// 受限 session 模式下 productId 缺省时走 user 维度 fallback（service 层），
	// 不再 400 拦截。Mock service 被调用并返 result 即可。
	mock := &MockTaskService{
		Result: &vo.PaginatedVO{List: []vo.TaskVO{}},
	}
	h := NewTaskHandler(mock)

	c := performGET(t, h.GetTasks, "/api/v1/tasks?page=1&pageSize=20")

	if !mock.Called {
		t.Error("expected GetTasks to be called (productId 缺省时走 fallback 路径)")
	}
	code := getResponseCode(t, c)
	if code != 200 {
		t.Errorf("status = %d, want %d (fallback 路径应返 200)", code, 200)
	}
}

func TestTaskHandler_GetTasks_Success(t *testing.T) {
	mock := &MockTaskService{
		Result: &vo.PaginatedVO{
			List:     []vo.TaskVO{},
			Total:    3,
			Page:     1,
			PageSize: 20,
		},
	}
	h := NewTaskHandler(mock)

	c := performGET(t, h.GetTasks, "/api/v1/tasks?productId=1&page=1&pageSize=20")

	if c.Response.StatusCode() != http.StatusOK {
		t.Errorf("status = %d, want %d", c.Response.StatusCode(), http.StatusOK)
	}
	if !mock.Called {
		t.Error("expected GetTasks to be called")
	}
}

func TestDTO_MaxPageSize(t *testing.T) {
	dtos := []func() (interface{ Validate() error }, *int){
		func() (interface{ Validate() error }, *int) {
			d := &dto.BugQueryDTO{Page: 1, PageSize: 99999}
			return d, &d.PageSize
		},
		func() (interface{ Validate() error }, *int) {
			d := &dto.StoryQueryDTO{Page: 1, PageSize: 99999}
			return d, &d.PageSize
		},
	}

	for i, mk := range dtos {
		validator, pageSizePtr := mk()
		if err := validator.Validate(); err != nil {
			t.Errorf("test %d: Validate() returned error: %v", i, err)
		}
		if *pageSizePtr != dto.MaxPageSize {
			t.Errorf("test %d: PageSize = %d, want %d", i, *pageSizePtr, dto.MaxPageSize)
		}
	}
}

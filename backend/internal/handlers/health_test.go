package handlers

import (
	"context"
	"fmt"
	"testing"

	myzentao "github.com/yi-nology/zentao-max/backend/internal/infra/zentao"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"

	"github.com/yi-nology/zentao-max/backend/internal/dto"
	"github.com/yi-nology/zentao-max/backend/internal/vo"
)

// nilOnErrService 模拟上游凭据失效：GetBugs/GetStories/GetTasks 返回 (nil, err)。
type nilOnErrService struct {
	BugServicer
	StoryServicer
	TaskServicer
	ProductServicer
	ProjectServicer
	UserServicer
}

func (f nilOnErrService) GetProducts() ([]vo.ProductVO, error) {
	return nil, fmt.Errorf("HTTP 401: Unauthorized")
}
func (f nilOnErrService) GetProjects(*dto.ProjectQueryDTO) ([]vo.ProjectVO, error) {
	return nil, fmt.Errorf("HTTP 401: Unauthorized")
}
func (f nilOnErrService) GetUsersAll() ([]vo.UserVO, error) {
	return nil, fmt.Errorf("HTTP 401: Unauthorized")
}

func (f nilOnErrService) GetBugs(*dto.BugQueryDTO) (*vo.PaginatedVO, error) {
	return nil, fmt.Errorf("HTTP 401: Unauthorized")
}
func (f nilOnErrService) GetStories(*dto.StoryQueryDTO) (*vo.PaginatedVO, error) {
	return nil, fmt.Errorf("HTTP 401: Unauthorized")
}
func (f nilOnErrService) GetTasks(*dto.TaskQueryDTO) (*vo.PaginatedVO, error) {
	return nil, fmt.Errorf("HTTP 401: Unauthorized")
}

// TestHealthCheckUpstreamErrorNoPanic 回归测试：上游失败（service 返回 nil 结果）
// 时 healthz 各检查项必须降级为 fail 项而非 nil deref panic。
// 2bfdec6 重构曾把 runCheck(start, result.Total, err) 写成一行——实参先于调用
// 求值，result 为 nil 时直接 panic（上游凭据失效即触发，CI 全绿漏网）。
func TestHealthCheckUpstreamErrorNoPanic(t *testing.T) {
	h := NewHealthHandler(nil, nilOnErrService{}, nilOnErrService{}, nilOnErrService{}, nilOnErrService{}, nilOnErrService{}, nilOnErrService{}, nil)

	for _, check := range []struct {
		name string
		fn   func() CheckItem
	}{{"bugs", h.checkBugs}, {"stories", h.checkStories}, {"tasks", h.checkTasks}} {
		item := check.fn()
		if item.Status != "fail" {
			t.Errorf("%s 上游失败时应为 fail，got %s", check.name, item.Status)
		}
	}
}

// TestHealthCheckRouteNoPanic 经完整 HTTP 链路验证 /api/healthz 在上游全部失败
// （凭据失效形态）时不 panic：client 用最小真实实例（checkZentao 需非 nil）。
func TestHealthCheckRouteNoPanic(t *testing.T) {
	client := myzentao.NewSessionClient("http://127.0.0.1:1", "u", "p", "", "", nil)
	h := NewHealthHandler(client, nilOnErrService{}, nilOnErrService{}, nilOnErrService{}, nilOnErrService{}, nilOnErrService{}, nilOnErrService{}, nil)
	srv := server.New(server.WithHostPorts("localhost:0"))
	srv.GET("/api/healthz", func(ctx context.Context, c *app.RequestContext) {
		h.Check(ctx, c)
	})

	w := ut.PerformRequest(srv.Engine, "GET", "/api/healthz", nil)
	// 200 envelope 内各项 fail（上游不可用），绝不 500 panic
	if w.Result().StatusCode() == 500 {
		t.Fatalf("healthz 上游失败时应返回 200 envelope（各项 fail），got 500: %s", w.Result().Body())
	}
}

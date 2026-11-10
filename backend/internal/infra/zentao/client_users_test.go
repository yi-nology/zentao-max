package zentao

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/yi-nology/zentao-max/backend/pkg/metrics"
)

// newUsersPaginationFixture 起 8 个用户的假禅道上游，供 GetUsers 分页测试复用
func newUsersPaginationFixture(t *testing.T) (*usersV1TestServer, *Client) {
	t.Helper()
	// GetUsers 会记 metrics（RecordCacheHit），先初始化全局 registry
	if err := metrics.Init(); err != nil {
		t.Fatalf("metrics.Init() err = %v", err)
	}
	srv := newUsersV1TestServer(t)

	users := map[string]interface{}{}
	for i := range 8 {
		account := string(rune('a' + i))
		users[account] = map[string]interface{}{
			"id": i + 1, "account": account, "realname": "用户" + account,
		}
	}
	raw, _ := json.Marshal(map[string]interface{}{"title": "用户列表", "users": users})
	env, _ := json.Marshal(map[string]string{"status": "success", "data": string(raw), "md5": "x"})
	srv.browseBody = string(env)

	c := newUsersV1Client(t, srv)
	return srv, c
}

// TestGetUsers_PageOverflow issue #11：page 无上界时 (page-1)*pageSize 溢出为负，
// 负下标切片 panic（对外 500）。修复后越界页应稳定返回空列表而非 panic。
func TestGetUsers_PageOverflow(t *testing.T) {
	srv, c := newUsersPaginationFixture(t)
	defer srv.Close()

	tests := []struct {
		name        string
		page        int
		pageSize    int
		expectedLen int
	}{
		{name: "第一页正常", page: 1, pageSize: 5, expectedLen: 5},
		{name: "尾页截断", page: 2, pageSize: 5, expectedLen: 3},
		{name: "越界页返回空", page: 3, pageSize: 5, expectedLen: 0},
		// page=2^62：(page-1)*20 溢出为 -20，修复前 allUsers[-20:0] panic
		{name: "page=2^62溢出为负下标", page: 4611686018427387904, pageSize: 20, expectedLen: 0},
		// page=MaxInt64：(page-1)*20 溢出为 -40，修复前 allUsers[-40:-20] panic
		{name: "page=MaxInt64溢出为负下标", page: math.MaxInt64, pageSize: 20, expectedLen: 0},
		// 2^61 量级：溢出回绕后 start 为正大数，走 start >= total 分支返回空
		{name: "page=2^61回绕为正大数", page: 2305843009213693952, pageSize: 20, expectedLen: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := c.GetUsers(tt.page, tt.pageSize)
			if err != nil {
				t.Fatalf("GetUsers(%d, %d) err = %v", tt.page, tt.pageSize, err)
			}
			if resp.Total != 8 {
				t.Errorf("Total = %d, want 8", resp.Total)
			}
			if len(resp.Users) != tt.expectedLen {
				t.Errorf("len(Users) = %d, want %d (page=%d pageSize=%d)",
					len(resp.Users), tt.expectedLen, tt.page, tt.pageSize)
			}
		})
	}
}

package zentao

import (
	"context"

	"github.com/yi-nology/zentao-max/backend/core/metrics"

	zentaosdk "github.com/yi-nology/zentao-max/backend/core/zentao/sdk"
)

// GetUsers 获取用户列表（支持分页）
func (c *Client) GetUsers(page, pageSize int) (*zentaosdk.UserListResponse, error) {
	allUsers, err := c.GetUsersAll()
	if err != nil {
		return nil, err
	}

	metrics.RecordCacheHit("users")

	total := len(allUsers)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= total {
		return &zentaosdk.UserListResponse{
			Users: []zentaosdk.User{},
			Page:  page,
			Total: total,
			Limit: pageSize,
		}, nil
	}
	if end > total {
		end = total
	}

	return &zentaosdk.UserListResponse{
		Users: allUsers[start:end],
		Page:  page,
		Total: total,
		Limit: pageSize,
	}, nil
}

// GetUsersAll 获取所有用户列表
func (c *Client) GetUsersAll() ([]zentaosdk.User, error) {
	return c.getUsersAllSession(context.Background())
}

// GetCurrentUser 获取当前登录用户（/user-profile.json）。
//
// Session 模式专属：是受限实例 SSO 上"获取 user.id"的最稳路径，
// 因为 /user-browse.json 在受限实例上经常被权限拦截，
// 但 /user-profile.json 总能拿到自己。
//
// 返回的 *zentaosdk.User.ID 就是 effort-calendar 等端点需要的 userID。
func (c *Client) GetCurrentUser() (*zentaosdk.User, error) {
	return c.getCurrentUserSession(context.Background())
}

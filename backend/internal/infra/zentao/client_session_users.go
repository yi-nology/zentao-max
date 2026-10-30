package zentao

import (
	"context"
	"fmt"

	"github.com/yi-nology/zentao-max/backend/pkg/logger"
	zentaosdk "github.com/yi-nology/zentao-max/backend/internal/infra/zentao/sdk"
)

// ---- Users (session mode) ----
//
// 用户列表三级回退链（2026-10-30 pm.kylin.com 实地抓包定级）：
//  1. /user-browse.json —— 权限完整实例，字段最全（gender/phone/status 等）；
//     受限实例（账号无用户管理权限）返 HTTP 200 + 0 字节。
//  2. /api.php/v1/users?limit=&page= —— v1 REST，session cookie 即可鉴权，
//     受限账号也能读全量（实测 3410 用户 limit=5000 单请求返回）；
//     未登录返 401，不会泄漏。字段少（无 gender/phone/status）。
//  3. /user-profile.json —— 仅当前用户，最后兜底。
//
// 当前用户走 /user-profile.json（拿完整 user 对象）。

// v1UsersPageSize 是 /api.php/v1/users 单页拉取条数。var 便于测试缩小验证翻页。
var v1UsersPageSize = 5000

// v1UsersMaxPages 是 v1 users 翻页安全上限（5000×5=25000 用户封顶，防异常实例翻页死循环）。
const v1UsersMaxPages = 5

// v1User 是 /api.php/v1/users 返回的单个用户（字段比 user-browse 少）。
type v1User struct {
	ID       int    `json:"id"`
	Account  string `json:"account"`
	Realname string `json:"realname"`
	Role     string `json:"role"`
	Dept     int    `json:"dept"`
	Email    string `json:"email"`
}

func (u v1User) toSDK() zentaosdk.User {
	return zentaosdk.User{
		ID:       u.ID,
		Account:  u.Account,
		Realname: u.Realname,
		Role:     u.Role,
		Dept:     u.Dept,
		Email:    u.Email,
	}
}

type sessionUserProfileResp struct {
	Title string      `json:"title"`
	User  sessionUser `json:"user"`
	// DeptPath 在部分实例上是数组，这里不做强类型解析。
	DeptPath interface{} `json:"deptPath"`
}

type sessionUser struct {
	ID       int    `json:"id"`
	Account  string `json:"account"`
	Realname string `json:"realname"`
	Role     string `json:"role"`
	Dept     int    `json:"dept"`
	Email    string `json:"email"`
	Gender   string `json:"gender"`
	Phone    string `json:"phone"`
	Status   string `json:"status"`
	Join     string `json:"join"`
}

func (u sessionUser) toSDK() zentaosdk.User {
	return zentaosdk.User{
		ID:       u.ID,
		Account:  u.Account,
		Realname: u.Realname,
		Role:     u.Role,
		Dept:     u.Dept,
		Email:    u.Email,
		Gender:   u.Gender,
		Phone:    u.Phone,
		Status:   u.Status,
		Join:     u.Join,
	}
}

// getCurrentUserSession 通过 /user-profile.json 获取当前登录用户。
func (c *Client) getCurrentUserSession(ctx context.Context) (*zentaosdk.User, error) {
	var resp sessionUserProfileResp
	if err := c.doSessionJSON(ctx, "GetCurrentUser", "/user-profile.json", &resp); err != nil {
		return nil, err
	}
	u := resp.User.toSDK()
	return &u, nil
}

// getUsersAllSession 获取用户列表，三级回退：
// /user-browse.json → /api.php/v1/users → 仅当前用户。
func (c *Client) getUsersAllSession(ctx context.Context) ([]zentaosdk.User, error) {
	// 1. user-browse：权限完整实例的首选（字段最全）。
	users, err := c.getUsersByBrowse(ctx)
	if err == nil {
		return users, nil
	}
	logger.Warn("user-browse 不可用，尝试 v1 REST users", "error", err)

	// 2. v1 REST：受限实例实测可用（session cookie 鉴权，401 保护未泄漏）。
	if v1users, ok, verr := c.getUsersV1Session(ctx); ok {
		return v1users, nil
	} else if verr != nil {
		logger.Warn("v1 users 不可用，降级为当前用户", "error", verr)
	}

	// 3. 最后兜底：仅当前用户（dashboard 的 current-user 合成依赖此路径）。
	me, merr := c.getCurrentUserSession(ctx)
	if merr != nil {
		return nil, fmt.Errorf("获取用户列表失败且降级失败: %w", merr)
	}
	return []zentaosdk.User{*me}, nil
}

// getUsersByBrowse 走经典 /user-browse.json（envelope + users map）。
// 成功返回非空列表；权限不足（0 字节/deny/错 shape）或列表为空时返回错误，
// 由调用方继续走 v1 回退（注意：空列表不再直接降级当前用户，v1 可能拿得到全量）。
func (c *Client) getUsersByBrowse(ctx context.Context) ([]zentaosdk.User, error) {
	var resp struct {
		Title   string                 `json:"title"`
		Users   map[string]sessionUser `json:"users"`
		Members map[string]sessionUser `json:"memberPairs"`
		Pager   *sessionPager          `json:"pager"`
	}
	// op 名用 GetUsers（opToEntity → sessionEntityUsers）。
	// 历史上用 "GetUsersAll"，opToEntity 未命中落到 sessionEntityBugs，
	// 用户数据被当 bugs 实体缓存（TTL/失效都错位），顺手纠正。
	if err := c.doSessionJSON(ctx, "GetUsers", "/user-browse.json", &resp); err != nil {
		// 受限实例实测：无用户管理权限时 HTTP 200 + 0 字节，JSON parse fail
		//（不是 access denied 走 locate=user-deny）。
		return nil, err
	}
	out := make([]zentaosdk.User, 0, len(resp.Users))
	for _, u := range resp.Users {
		// 受限实例 SSO 限流时 /user-browse.json 偶尔返错端点数据（3400 个空 user），
		// id=0 + account="" 的占位 user 没意义，过滤掉。
		if u.ID == 0 && u.Account == "" {
			continue
		}
		out = append(out, u.toSDK())
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("user-browse 返回空列表")
	}
	return out, nil
}

// getUsersV1Session 走 v1 REST /api.php/v1/users 分页拉全量。
// 返回 ok=false 表示该端点不可用（老实例无 v1 / 403 / 空数据），调用方继续兜底。
func (c *Client) getUsersV1Session(ctx context.Context) ([]zentaosdk.User, bool, error) {
	var out []zentaosdk.User
	total := 0
	for page := 1; page <= v1UsersMaxPages; page++ {
		var resp struct {
			Total int      `json:"total"`
			Users []v1User `json:"users"`
		}
		path := fmt.Sprintf("/api.php/v1/users?limit=%d&page=%d", v1UsersPageSize, page)
		if err := c.doV1JSON(ctx, "GetUsers", path, &resp); err != nil {
			if page == 1 {
				// 首页失败 = 该端点整体不可用，交给调用方走下一级回退。
				return nil, false, err
			}
			// 后续页失败：已拉到的部分数据仍有效，停止翻页。
			logger.Warn("v1 users 后续页拉取失败，使用部分数据", "page", page, "error", err)
			break
		}
		if len(resp.Users) == 0 {
			if page == 1 {
				return nil, false, fmt.Errorf("v1 users 首页为空")
			}
			break
		}
		for _, u := range resp.Users {
			if u.ID == 0 && u.Account == "" {
				continue
			}
			out = append(out, u.toSDK())
		}
		total = resp.Total
		if len(resp.Users) < v1UsersPageSize || (total > 0 && len(out) >= total) {
			break
		}
	}
	if len(out) == 0 {
		return nil, false, fmt.Errorf("v1 users 无数据")
	}
	return out, true, nil
}

// getUsersSession 分页包装 GetUsersAll（内存分页，与 Token 模式行为一致）。
func (c *Client) getUsersSession(ctx context.Context, page, pageSize int) ([]zentaosdk.User, error) {
	all, err := c.getUsersAllSession(ctx)
	if err != nil {
		return nil, err
	}
	start := (page - 1) * pageSize
	if start < 0 {
		start = 0
	}
	end := start + pageSize
	if end > len(all) {
		end = len(all)
	}
	if start >= len(all) {
		return nil, nil
	}
	return all[start:end], nil
}

// getAccountNameSession 返回当前登录账号名（与 c.GetAccount() 一致，
// 但优先用 user-profile 的 account 字段，更贴近服务端实际态）。
func (c *Client) getAccountNameSession(ctx context.Context) string {
	if u, err := c.getCurrentUserSession(ctx); err == nil && u.Account != "" {
		return u.Account
	}
	return c.GetAccount()
}

var _ = fmt.Sprintf // 保留 fmt 以便后续扩展

package zentao

import (
	"context"
	"fmt"
	"strings"

	"github.com/yi-nology/zentao-max/backend/pkg/logger"
	zentaosdk "github.com/yi-nology/zentao-max/backend/internal/infra/zentao/sdk"
)

// ---- Users (session mode) ----
//
// 受限实例上 /user-browse.json 返回 0 字节（账号无用户管理权限）。
// 会话模式下：当前用户走 /user-profile.json（拿完整 user 对象）；
// 用户列表能力受限，GetUsersAll 在权限允许时走 /user-browse.json，
// 不允许时返回仅含当前用户的单元素列表（保证 dashboard 的 current-user 合成可用）。

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

// isEmptyBodyParseError 判断 err 是否为「HTTP 200 但 body 为空 / 错 shape 导致 JSON 解析失败」类。
//
// 受限实例上无用户管理权限的账号访问 /user-browse.json 至少有两种伪装：
//  1. HTTP 200 + 0 字节 → 解析时 "unexpected end of JSON input"
//  2. 受限实例 SSO 限流时返了 project list 的 shape（"title":"项目列表" + projectStats 字段），
//     unmarshal 到 sessionUser 结构体失败：cannot unmarshal string into ... of type sessionUser
// 两种都本质上是"权限受限"或"限流伪装"，应与 IsSessionAccessDenied 同样走 fallback
// 到 getCurrentUserSession。
//
// 注意：IsSessionAccessDenied 捕获的是 locate=user-deny 触发的 errSessionAccessDenied，
// 这里是另外两种表现，三种都需要 fallback。
func isEmptyBodyParseError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if strings.Contains(msg, "unexpected end of JSON input") {
		return true
	}
	if strings.Contains(msg, "body=") && strings.HasSuffix(strings.TrimSpace(msg), "body=") {
		return true
	}
	// wrong shape 伪装：受限实例限流时 user-browse 端点返 project list 的 schema
	// （"项目列表" / projectStats），unmarshal sessionUser 失败。
	if strings.Contains(msg, "of type zentao.sessionUser") {
		return true
	}
	return false
}

// getUsersAllSession 获取用户列表。权限允许时走 /user-browse.json；
// 否则回退到仅当前用户。前端 dashboard 的 current-user 合成依赖此方法。
func (c *Client) getUsersAllSession(ctx context.Context) ([]zentaosdk.User, error) {
	var resp struct {
		Title   string                 `json:"title"`
		Users   map[string]sessionUser `json:"users"`
		Members map[string]sessionUser `json:"memberPairs"`
		Pager   *sessionPager          `json:"pager"`
	}
	// /user-browse.json：权限允许时返回用户列表；不允许时返回空 body 或 deny。
	// 受限实例实测：账号无用户管理权限时 HTTP 200 + 0 字节，导致 JSON parse fail
	// （不是 access denied 走 locate=user-deny），需要也 fallback 到 getCurrentUserSession。
	if err := c.doSessionJSON(ctx, "GetUsersAll", "/user-browse.json", &resp); err != nil {
		// user-browse 是可选能力：无权限（空 body/deny）、网关偶发截断等都属于
		// "拿不到完整列表"——一律降级为仅当前用户，而不是把错误抛给上层
		// （列表/筛选页因此整页报错，指派人下拉这种非关键数据不值得）。
		// 原条件只识别两种错误形态，分类器文案变化就漏（踩过）。
		logger.Warn("user-browse 不可用，降级为当前用户", "error", err)
		me, merr := c.getCurrentUserSession(ctx)
		if merr != nil {
			return nil, fmt.Errorf("获取用户列表失败且降级失败: %w", merr)
		}
		return []zentaosdk.User{*me}, nil
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
		// data 为空（受限实例行为）：回退到当前用户。
		me, err := c.getCurrentUserSession(ctx)
		if err != nil {
			return nil, err
		}
		return []zentaosdk.User{*me}, nil
	}
	return out, nil
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

package service

import (
	"github.com/yi-nology/common/biz/zentao"

	"github.com/yi-nology/zentao-max/backend/core/vo"
	myzentao "github.com/yi-nology/zentao-max/backend/core/zentao"
)

// UserService 用户业务逻辑服务
// 负责处理用户相关的业务逻辑
type UserService struct {
	client *myzentao.Client
}

// NewUserService 创建用户服务
func NewUserService(client *myzentao.Client) *UserService {
	return &UserService{client: client}
}

// GetUsers 获取用户列表（支持分页）
// 业务逻辑：调用禅道客户端获取分页用户数据
func (s *UserService) GetUsers(page, pageSize int) (*vo.PaginatedVO, error) {
	userList, err := s.client.GetUsers(page, pageSize)
	if err != nil {
		return nil, err
	}

	users := s.convertToVO(userList.Users)

	return &vo.PaginatedVO{
		List:     users,
		Total:    userList.Total,
		Page:     userList.Page,
		PageSize: userList.Limit,
	}, nil
}

// GetUsersAll 获取所有用户列表
// 业务逻辑：调用禅道客户端获取所有用户（带缓存）
func (s *UserService) GetUsersAll() ([]vo.UserVO, error) {
	users, err := s.client.GetUsersAll()
	if err != nil {
		return nil, err
	}

	// 转换为VO
	return s.convertToVO(users), nil
}

// GetCurrentUser 获取当前登录用户信息
// 业务逻辑：从所有用户中查找当前登录用户
func (s *UserService) GetCurrentUser() (*vo.UserVO, error) {
	users, err := s.client.GetUsersAll()
	if err != nil {
		return nil, err
	}

	// 查找当前登录用户
	account := s.client.GetAccount()
	for _, u := range users {
		if u.Account == account {
			return &vo.UserVO{
				ID:       u.ID,
				Account:  u.Account,
				Realname: u.Realname,
			}, nil
		}
	}

	return nil, &ValidationError{Message: "未找到当前登录用户"}
}

// convertToVO 将zentao.User转换为vo.UserVO
func (s *UserService) convertToVO(users []zentao.User) []vo.UserVO {
	if len(users) == 0 {
		return []vo.UserVO{}
	}

	result := make([]vo.UserVO, 0, len(users))
	for _, user := range users {
		result = append(result, vo.UserVO{
			ID:       user.ID,
			Account:  user.Account,
			Realname: user.Realname,
		})
	}
	return result
}

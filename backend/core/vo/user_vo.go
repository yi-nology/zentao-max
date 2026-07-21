package vo

// UserVO 用户响应模型
// 用于API响应的数据结构
type UserVO struct {
	ID       int    `json:"id"`       // 用户ID
	Account  string `json:"account"`  // 账号
	Realname string `json:"realname"` // 真实姓名
}

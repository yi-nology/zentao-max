package vo

// ProjectVO 项目响应模型
// 用于API响应的数据结构
type ProjectVO struct {
	ID       int         `json:"id"`       // 项目ID
	Name     string      `json:"name"`     // 项目名称
	Code     string      `json:"code"`     // 项目代码
	Model    string      `json:"model"`    // 模型
	Type     string      `json:"type"`     // 类型
	Status   string      `json:"status"`   // 状态
	PM       interface{} `json:"pm"`       // 项目经理
	Begin    string      `json:"begin"`    // 开始日期
	End      string      `json:"end"`      // 结束日期
	Progress interface{} `json:"progress"` // 进度
}

// ExecutionVO 执行/迭代响应模型
type ExecutionVO struct {
	ID      int    `json:"id"`      // 执行ID
	Project int    `json:"project"` // 项目ID
	Name    string `json:"name"`    // 执行名称
	Code    string `json:"code"`    // 执行代码
	Status  string `json:"status"`  // 状态
	Type    string `json:"type"`    // 类型
	Begin   string `json:"begin"`   // 开始日期
	End     string `json:"end"`     // 结束日期
}

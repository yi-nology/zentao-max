package vo

// TaskVO 任务响应模型
// 用于API响应的数据结构
type TaskVO struct {
	ID           int         `json:"id"`           // 任务ID
	Project      int         `json:"project"`      // 项目ID
	Execution    int         `json:"execution"`    // 执行ID
	Name         string      `json:"name"`         // 任务名称
	Type         string      `json:"type"`         // 类型
	Pri          interface{} `json:"pri"`          // 优先级
	Status       string      `json:"status"`       // 状态
	AssignedTo   interface{} `json:"assignedTo"`   // 指派给
	EstStarted   interface{} `json:"estStarted"`   // 预计开始
	Deadline     interface{} `json:"deadline"`     // 截止日期
	Estimate     interface{} `json:"estimate"`     // 预计工时
	Consumed     interface{} `json:"consumed"`     // 已消耗工时
	Left         interface{} `json:"left"`         // 剩余工时
	Desc         interface{} `json:"desc"`         // 描述
	OpenedBy     interface{} `json:"openedBy"`     // 创建人
	OpenedDate   interface{} `json:"openedDate"`   // 创建日期
	FinishedBy   interface{} `json:"finishedBy"`   // 完成人
	FinishedDate interface{} `json:"finishedDate"` // 完成日期
	ClosedBy     interface{} `json:"closedBy"`     // 关闭人
	ClosedDate   interface{} `json:"closedDate"`   // 关闭日期
	StatusName   interface{} `json:"statusName"`   // 状态名称
}

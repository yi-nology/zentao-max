package vo

// StoryVO 需求响应模型
// 用于API响应的数据结构
type StoryVO struct {
	ID           int         `json:"id"`           // 需求ID
	Product      int         `json:"product"`      // 产品ID
	Module       interface{} `json:"module"`       // 模块ID
	Plan         interface{} `json:"plan"`         // 计划
	Source       interface{} `json:"source"`       // 来源
	Title        string      `json:"title"`        // 标题
	Spec         interface{} `json:"spec"`         // 规格
	Verify       interface{} `json:"verify"`       // 验收标准
	Type         string      `json:"type"`         // 类型
	Status       string      `json:"status"`       // 状态
	Stage        interface{} `json:"stage"`        // 阶段
	Pri          interface{} `json:"pri"`          // 优先级
	Estimate     interface{} `json:"estimate"`     // 预计工时
	Version      interface{} `json:"version"`      // 版本
	OpenedBy     interface{} `json:"openedBy"`     // 创建人
	OpenedDate   interface{} `json:"openedDate"`   // 创建日期
	AssignedTo   interface{} `json:"assignedTo"`   // 指派给
	AssignedDate interface{} `json:"assignedDate"` // 指派日期
	ClosedBy     interface{} `json:"closedBy"`     // 关闭人
	ClosedDate   interface{} `json:"closedDate"`   // 关闭日期
	ClosedReason interface{} `json:"closedReason"` // 关闭原因
}

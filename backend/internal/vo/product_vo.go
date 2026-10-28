package vo

// ProductVO 产品响应模型
// 用于API响应的数据结构
type ProductVO struct {
	ID     int    `json:"id"`     // 产品ID
	Name   string `json:"name"`   // 产品名称
	Code   string `json:"code"`   // 产品代码
	Type   string `json:"type"`   // 类型
	Status string `json:"status"` // 状态
	Desc   string `json:"desc"`   // 描述
}

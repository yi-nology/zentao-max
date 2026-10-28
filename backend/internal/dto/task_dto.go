package dto

type TaskQueryDTO struct {
	ProductID   int    `form:"productId" json:"productId"`
	ExecutionID int    `form:"executionId" json:"executionId"`
	AssignedTo  string `form:"assignedTo" json:"assignedTo"`
	Status      string `form:"status" json:"status"`
	StartDate   string `form:"startDate" json:"startDate"`
	EndDate     string `form:"endDate" json:"endDate"`
	Page        int    `form:"page" json:"page"`
	PageSize    int    `form:"pageSize" json:"pageSize"`
}

func (dto *TaskQueryDTO) Validate() error {
	if dto.Page <= 0 {
		dto.Page = 1
	}
	if dto.PageSize <= 0 {
		dto.PageSize = 20
	}
	if dto.PageSize > MaxPageSize {
		dto.PageSize = MaxPageSize
	}
	// productId 改为可选：受限 session 模式下 product 维度严格过滤容易丢数据
	// （<user-account> 的 task 不一定都关联到选中的 product）。productId=0/空时
	// 走"全 execution 收集"路径（dashboard 同款），保证页面有数据。
	return nil
}

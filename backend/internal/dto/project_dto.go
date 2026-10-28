package dto

// ProjectQueryDTO 项目查询请求参数
type ProjectQueryDTO struct {
	ProductID int `form:"productId" json:"productId"` // 产品ID（可选）
	Page      int `form:"page" json:"page"`
	PageSize  int `form:"pageSize" json:"pageSize"`
}

func (dto *ProjectQueryDTO) Validate() error {
	if dto.Page <= 0 {
		dto.Page = 1
	}
	if dto.PageSize <= 0 {
		dto.PageSize = 20
	}
	if dto.PageSize > MaxPageSize {
		dto.PageSize = MaxPageSize
	}
	return nil
}

// ExecutionQueryDTO 执行/迭代查询请求参数
type ExecutionQueryDTO struct {
	ProjectID int `form:"projectId" json:"projectId"` // 项目ID（可选）
	ProductID int `form:"productId" json:"productId"` // 产品ID（可选）
	Page      int `form:"page" json:"page"`
	PageSize  int `form:"pageSize" json:"pageSize"`
}

func (dto *ExecutionQueryDTO) Validate() error {
	if dto.Page <= 0 {
		dto.Page = 1
	}
	if dto.PageSize <= 0 {
		dto.PageSize = 20
	}
	if dto.PageSize > MaxPageSize {
		dto.PageSize = MaxPageSize
	}
	return nil
}

package dto

type StoryQueryDTO struct {
	ProductID    int    `form:"productId" json:"productId"`
	ProjectID    int    `form:"projectId" json:"projectId"`
	ExecutionID  int    `form:"executionId" json:"executionId"`
	AssignedTo   string `form:"assignedTo" json:"assignedTo"`
	StartDate    string `form:"startDate" json:"startDate"`
	EndDate      string `form:"endDate" json:"endDate"`
	SpecificDate string `form:"specificDate" json:"specificDate"`
	Page         int    `form:"page" json:"page"`
	PageSize     int    `form:"pageSize" json:"pageSize"`
}

func (dto *StoryQueryDTO) Validate() error {
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

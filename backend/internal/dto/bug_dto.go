package dto

type BugQueryDTO struct {
	ProductID    int    `form:"productId" json:"productId"`
	ProjectID    int    `form:"projectId" json:"projectId"`
	Status       string `form:"status" json:"status"`
	AssignedTo   string `form:"assignedTo" json:"assignedTo"`
	Version      string `form:"version" json:"version"`
	Type         string `form:"type" json:"type"`
	StartDate    string `form:"startDate" json:"startDate"`
	EndDate      string `form:"endDate" json:"endDate"`
	SpecificDate string `form:"specificDate" json:"specificDate"`
	Page         int    `form:"page" json:"page"`
	PageSize     int    `form:"pageSize" json:"pageSize"`
}

func (dto *BugQueryDTO) Validate() error {
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

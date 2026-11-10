package dto

// UserQueryDTO 用户列表查询参数（issue #11：/users 此前手写解析绕过 DTO 校验，
// page 无上界导致 (page-1)*pageSize 溢出 panic；对齐 bugs/stories 的 DTO 模式）
type UserQueryDTO struct {
	Page     int `form:"page" json:"page"`
	PageSize int `form:"pageSize" json:"pageSize"`
}

// Validate 规范化分页参数：page 钳到 [1, MaxPage]，pageSize 钳到 [1, MaxPageSize]
func (dto *UserQueryDTO) Validate() error {
	dto.Page = min(max(dto.Page, 1), MaxPage)
	dto.PageSize = min(max(dto.PageSize, 20), MaxPageSize)
	return nil
}

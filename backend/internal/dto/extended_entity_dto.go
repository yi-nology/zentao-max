package dto

// 通用查询 DTO，供 Phase2b 新增实体复用。
// 大多数实体只需 productID + 分页。

// ExtendedQueryDTO 通用实体查询参数。
type ExtendedQueryDTO struct {
	ProductID int `form:"productId" json:"productId"`
	Page      int `form:"page" json:"page"`
	PageSize  int `form:"pageSize" json:"pageSize"`
}

// Validate 设置默认分页值。
func (d *ExtendedQueryDTO) Validate() {
	if d.Page <= 0 {
		d.Page = 1
	}
	if d.PageSize <= 0 {
		d.PageSize = 20
	}
	if d.PageSize > MaxPageSize {
		d.PageSize = MaxPageSize
	}
}

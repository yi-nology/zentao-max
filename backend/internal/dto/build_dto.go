package dto

import "fmt"

// BuildQueryDTO 版本（Build）查询请求参数
// 查询版本必须指定项目或执行，二者至少需要一个
type BuildQueryDTO struct {
	ProjectID   int `form:"projectId" json:"projectId"`     // 项目 ID（与 ExecutionID 二选一）
	ExecutionID int `form:"executionId" json:"executionID"` // 执行/迭代 ID（与 ProjectID 二选一）
}

// Validate 校验查询参数：ID 不能为负，且 ProjectID / ExecutionID 至少有一个大于 0
func (dto *BuildQueryDTO) Validate() error {
	if dto.ProjectID < 0 || dto.ExecutionID < 0 {
		return fmt.Errorf("projectId 和 executionId 不能为负数")
	}
	if dto.ProjectID == 0 && dto.ExecutionID == 0 {
		return fmt.Errorf("projectId 和 executionId 至少需要一个")
	}
	return nil
}

package utils

import (
	"github.com/oekit/pkg/slicex"

	"github.com/yi-nology/zentao-max/backend/internal/models"
	zentaosdk "github.com/yi-nology/zentao-max/backend/internal/infra/zentao/sdk"
)

// Converter 数据转换器接口
type Converter[S any, T any] interface {
	Convert(source S) T
}

// ConvertSlice 批量转换切片
// 使用泛型支持任意类型的转换
func ConvertSlice[S any, T any](sources []S, converter Converter[S, T]) []T {
	if len(sources) == 0 {
		return []T{}
	}

	result := make([]T, 0, len(sources))
	for _, source := range sources {
		result = append(result, converter.Convert(source))
	}
	return result
}

// ConvertSliceFunc 使用函数进行批量转换
func ConvertSliceFunc[S any, T any](sources []S, converterFunc func(S) T) []T {
	// 实现委托 oekit/pkg/slicex，保持项目 API 稳定
	return slicex.Map(sources, converterFunc)
}

// BugConverter Bug转换器
type BugConverter struct{}

// Convert 转换单个Bug
func (c *BugConverter) Convert(source zentaosdk.Bug) models.Bug {
	return models.Bug{
		ID:            source.ID,
		Project:       source.Project,
		Product:       source.Product,
		Title:         source.Title,
		Keywords:      source.Keywords,
		Severity:      source.Severity,
		Pri:           source.Pri,
		Type:          source.Type,
		OS:            source.OS,
		Browser:       source.Browser,
		Hardware:      source.Hardware,
		Steps:         source.Steps,
		Status:        source.Status,
		SubStatus:     source.SubStatus,
		Color:         source.Color,
		Confirmed:     source.Confirmed,
		PlanTime:      source.PlanTime,
		OpenedBy:      models.UserRef(source.OpenedBy),
		OpenedDate:    source.OpenedDate,
		OpenedBuild:   source.OpenedBuild.Slice(),
		AssignedTo:    models.UserRef(source.AssignedTo),
		AssignedDate:  source.AssignedDate,
		Deadline:      source.Deadline,
		ResolvedBy:    source.ResolvedBy,
		Resolution:    source.Resolution,
		ResolvedBuild: source.ResolvedBuild,
		ResolvedDate:  source.ResolvedDate,
		ClosedBy:      source.ClosedBy,
		ClosedDate:    source.ClosedDate,
		StatusName:    source.StatusName,
		LifeCycle:     source.LifeCycle,
	}
}

// ConvertBugs 批量转换Bug
func ConvertBugs(sources []zentaosdk.Bug) []models.Bug {
	converter := &BugConverter{}
	return ConvertSlice(sources, converter)
}

// TaskConverter Task转换器
type TaskConverter struct{}

// Convert 转换单个Task
func (c *TaskConverter) Convert(source zentaosdk.Task) models.Task {
	return models.Task{
		ID:           source.ID,
		Project:      source.Project,
		Execution:    source.Execution,
		Name:         source.Name,
		Type:         source.Type,
		Pri:          source.Pri,
		Status:       source.Status,
		AssignedTo:   source.AssignedTo,
		EstStarted:   source.EstStarted,
		Deadline:     source.Deadline,
		Estimate:     source.Estimate,
		Consumed:     source.Consumed,
		Left:         source.Left,
		Desc:         source.Desc,
		OpenedBy:     source.OpenedBy,
		OpenedDate:   source.OpenedDate,
		FinishedBy:   source.FinishedBy,
		FinishedDate: source.FinishedDate,
		ClosedBy:     source.ClosedBy,
		ClosedDate:   source.ClosedDate,
		StatusName:   source.StatusName,
	}
}

// ConvertTasks 批量转换Task
func ConvertTasks(sources []zentaosdk.Task) []models.Task {
	converter := &TaskConverter{}
	return ConvertSlice(sources, converter)
}

// StoryConverter Story转换器
type StoryConverter struct{}

// Convert 转换单个Story
func (c *StoryConverter) Convert(source zentaosdk.Story) models.Story {
	return models.Story{
		ID:           source.ID,
		Product:      source.Product,
		Module:       source.Module,
		Plan:         source.Plan,
		Source:       source.Source,
		Title:        source.Title,
		Spec:         source.Spec,
		Verify:       source.Verify,
		Type:         source.Type,
		Status:       source.Status,
		Stage:        source.Stage,
		Pri:          source.Pri,
		Estimate:     source.Estimate,
		Version:      source.Version,
		OpenedBy:     source.OpenedBy,
		OpenedDate:   source.OpenedDate,
		AssignedTo:   source.AssignedTo,
		AssignedDate: source.AssignedDate,
		ClosedBy:     source.ClosedBy,
		ClosedDate:   source.ClosedDate,
		ClosedReason: source.ClosedReason,
	}
}

// ConvertStories 批量转换Story
func ConvertStories(sources []zentaosdk.Story) []models.Story {
	converter := &StoryConverter{}
	return ConvertSlice(sources, converter)
}

// ProductConverter Product转换器
type ProductConverter struct{}

// Convert 转换单个Product
func (c *ProductConverter) Convert(source zentaosdk.Product) models.Product {
	return models.Product{
		ID:     source.ID,
		Name:   source.Name,
		Code:   source.Code,
		Type:   source.Type,
		Status: source.Status,
		Desc:   source.Desc,
	}
}

// ConvertProducts 批量转换Product
func ConvertProducts(sources []zentaosdk.Product) []models.Product {
	converter := &ProductConverter{}
	return ConvertSlice(sources, converter)
}

// ProjectConverter Project转换器
type ProjectConverter struct{}

// Convert 转换单个Project
func (c *ProjectConverter) Convert(source zentaosdk.Project) models.Project {
	return models.Project{
		ID:       source.ID,
		Name:     source.Name,
		Code:     source.Code,
		Model:    source.Model,
		Type:     source.Type,
		Status:   source.Status,
		PM:       source.PM,
		Begin:    source.Begin,
		End:      source.End,
		Progress: source.Progress,
	}
}

// ConvertProjects 批量转换Project
func ConvertProjects(sources []zentaosdk.Project) []models.Project {
	converter := &ProjectConverter{}
	return ConvertSlice(sources, converter)
}

// ExecutionConverter Execution转换器
type ExecutionConverter struct{}

// Convert 转换单个Execution
func (c *ExecutionConverter) Convert(source zentaosdk.Execution) models.Execution {
	return models.Execution{
		ID:      source.ID,
		Project: source.Project,
		Name:    source.Name,
		Code:    source.Code,
		Status:  source.Status,
		Type:    source.Type,
		Begin:   source.Begin,
		End:     source.End,
	}
}

// ConvertExecutions 批量转换Execution
func ConvertExecutions(sources []zentaosdk.Execution) []models.Execution {
	converter := &ExecutionConverter{}
	return ConvertSlice(sources, converter)
}

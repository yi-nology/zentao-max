package service

import (
	"context"
	"strings"

	"github.com/yi-nology/common/biz/zentao"

	"github.com/yi-nology/zentao-max/backend/core/dto"
	"github.com/yi-nology/zentao-max/backend/core/utils"
	"github.com/yi-nology/zentao-max/backend/core/vo"
	myzentao "github.com/yi-nology/zentao-max/backend/core/zentao"
)

// StoryService 需求业务逻辑服务
// 负责处理需求相关的业务逻辑
type StoryService struct {
	client *myzentao.Client
}

// NewStoryService 创建需求服务
func NewStoryService(client *myzentao.Client) *StoryService {
	return &StoryService{client: client}
}

// GetStories 获取需求列表
// 业务逻辑：
// 1. 根据执行ID、项目ID或产品ID查询需求（优先级：executionID > projectID > productID）
// 2. 应用筛选条件（指派人、时间范围）
// 3. 分页处理
func (s *StoryService) GetStories(query *dto.StoryQueryDTO) (*vo.PaginatedVO, error) {
	var stories []zentao.Story
	var err error

	// 优先级: executionID > projectID > productID > 全 execution fallback
	if query.ExecutionID != 0 {
		stories, err = s.client.GetStoriesByExecution(query.ExecutionID, 1, 1000)
	} else if query.ProjectID != 0 {
		stories, err = s.client.GetStoriesByProject(query.ProjectID, 1, 1000)
	} else if query.ProductID != 0 {
		stories, err = s.client.GetStoriesByProduct(query.ProductID, 1, 1000)
		if err == nil && len(stories) == 0 {
			// product 维度没数据时，fallback 到全 execution 收集（dashboard 同款）
			stories, err = s.fetchAllStoriesAcrossExecutions()
		}
	} else {
		// 全部缺省：直接全 execution 收集（dashboard 同款），保证 health check
		// 跟产品选择解耦，页面也能看到 <user-account> 实际关联的 story
		stories, err = s.fetchAllStoriesAcrossExecutions()
		if err != nil {
			return nil, err
		}
	}

	if err != nil {
		return nil, err
	}

	// 使用链式过滤器进行筛选
	chainFilter := utils.NewChainFilter(stories)

	// 按指派人筛选
	if query.AssignedTo != "" {
		chainFilter = chainFilter.Filter(func(item zentao.Story) bool {
			assignedStr := ""
			if item.AssignedTo != nil {
				switch v := item.AssignedTo.(type) {
				case string:
					assignedStr = v
				case map[string]interface{}:
					if account, ok := v["account"].(string); ok {
						assignedStr = account
					}
				}
			}
			return strings.EqualFold(assignedStr, query.AssignedTo)
		})
	}

	// 按时间范围或具体日期筛选
	if query.StartDate != "" || query.EndDate != "" || query.SpecificDate != "" {
		chainFilter = chainFilter.Filter(func(item zentao.Story) bool {
			filtered := utils.FilterByDateRangeOrSpecific(
				[]zentao.Story{item},
				query.StartDate,
				query.EndDate,
				query.SpecificDate,
				func(s zentao.Story) string { return s.OpenedDate },
			)
			return len(filtered) > 0
		})
	}

	// 获取总数
	total := chainFilter.Count()

	// 执行分页
	pagedStories := chainFilter.Paginate(query.Page, query.PageSize).Result()

	list := s.convertToVO(pagedStories)

	return &vo.PaginatedVO{
		List:     list,
		Total:    total,
		Page:     query.Page,
		PageSize: query.PageSize,
	}, nil
}

// fetchAllStoriesAcrossExecutions 直接遍历全 execution 收集 story（dashboard 同款）。
//
// 受限 session 模式下 user 的 story 不一定都关联到选中的 product，按 product
// 过滤容易 0 数据。dashboard 风格遍历全 execution 是更稳的"全量"语义。
func (s *StoryService) fetchAllStoriesAcrossExecutions() ([]zentao.Story, error) {
	allExecCtxs, err := s.client.GetExecutionsByProductContext(context.Background(), 0)
	if err != nil {
		return nil, err
	}
	var allStories []zentao.Story
	for _, ec := range allExecCtxs {
		stories, err := s.client.GetStoriesByExecution(ec.Exec.ID, 1, 10000)
		if err != nil {
			continue
		}
		allStories = append(allStories, stories...)
	}
	return allStories, nil
}

// convertToVO 将zentao.Story转换为vo.StoryVO
func (s *StoryService) convertToVO(stories []zentao.Story) []vo.StoryVO {
	if len(stories) == 0 {
		return []vo.StoryVO{}
	}

	result := make([]vo.StoryVO, 0, len(stories))
	for _, story := range stories {
		result = append(result, vo.StoryVO{
			ID:           story.ID,
			Product:      story.Product,
			Module:       story.Module,
			Plan:         story.Plan,
			Source:       story.Source,
			Title:        story.Title,
			Spec:         story.Spec,
			Verify:       story.Verify,
			Type:         story.Type,
			Status:       story.Status,
			Stage:        story.Stage,
			Pri:          story.Pri,
			Estimate:     story.Estimate,
			Version:      story.Version,
			OpenedBy:     story.OpenedBy,
			OpenedDate:   story.OpenedDate,
			AssignedTo:   story.AssignedTo,
			AssignedDate: story.AssignedDate,
			ClosedBy:     story.ClosedBy,
			ClosedDate:   story.ClosedDate,
			ClosedReason: story.ClosedReason,
		})
	}
	return result
}

// ValidationError 验证错误
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

package service

import (
	"context"
	"strings"
	"time"

	zentaosdk "github.com/yi-nology/zentao-max/backend/internal/infra/zentao/sdk"

	"github.com/yi-nology/zentao-max/backend/internal/dto"
	myzentao "github.com/yi-nology/zentao-max/backend/internal/infra/zentao"
	"github.com/yi-nology/zentao-max/backend/internal/vo"
	"github.com/yi-nology/zentao-max/backend/pkg/utils"
)

// StoryService 需求业务逻辑服务
// 负责处理需求相关的业务逻辑
type StoryService struct {
	client *myzentao.Client
	cache  *CacheService // 可选；nil = 无 SWR 缓存直连
}

// NewStoryService 创建需求服务
func NewStoryService(client *myzentao.Client) *StoryService {
	return &StoryService{client: client}
}

// SetCache 注入 SWR 缓存（registry 在缓存层就绪后回调装配）。
func (s *StoryService) SetCache(c *CacheService) { s.cache = c }

// GetStories 获取需求列表
// 业务逻辑：
// 1. 根据执行ID、项目ID或产品ID查询需求（优先级：executionID > projectID > productID）
// 2. 应用筛选条件（指派人、时间范围）
// 3. 分页处理
func (s *StoryService) GetStories(query *dto.StoryQueryDTO) (*vo.PaginatedVO, error) {
	stories, err := s.fetchStories(query)
	if err != nil {
		return nil, err
	}

	// 使用链式过滤器进行筛选
	chainFilter := applyStoryFilters(stories, query)

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

// GetStoryStatusCounts 返回应用筛选条件后（不分页）的各状态需求数量，供统计卡使用
func (s *StoryService) GetStoryStatusCounts(query *dto.StoryQueryDTO) (map[string]int, error) {
	counts := map[string]int{"draft": 0, "active": 0, "reviewing": 0, "changed": 0, "closed": 0}

	stories, err := s.fetchStories(query)
	if err != nil {
		return nil, err
	}

	for _, story := range applyStoryFilters(stories, query).Result() {
		if _, ok := counts[story.Status]; ok {
			counts[story.Status]++
		}
	}
	return counts, nil
}

// fetchStories 按优先级 executionID > projectID > productID 拉取需求全量
func (s *StoryService) fetchStories(query *dto.StoryQueryDTO) ([]zentaosdk.Story, error) {
	// 优先级: executionID > projectID > productID > 全 execution fallback
	if query.ExecutionID != 0 {
		return s.client.GetStoriesByExecution(query.ExecutionID, 1, 1000)
	}
	if query.ProjectID != 0 {
		return s.client.GetStoriesByProject(query.ProjectID, 1, 1000)
	}
	if query.ProductID != 0 {
		// 产品维度直连产品需求端点 + SWR（stories:product 30min，重复请求
		// 秒回）；空结果 → fallback 全遍历（关联链断裂产品的诚实兜底），
		// fallback 结果不缓存（防关联链断裂产品的空结果固化）
		cached, _, err := swrLoadJSON[[]zentaosdk.Story](s.cache, "stories:product", query.ProductID, 30*time.Minute,
			func(ctx context.Context) ([]zentaosdk.Story, error) {
				return s.client.GetStoriesByProduct(query.ProductID, 1, 1000)
			},
			nil, "")
		if err != nil {
			return nil, err
		}
		if len(cached) > 0 {
			return cached, nil
		}
		return s.fetchStoriesAcrossExecutions(60 * time.Second)
	}
	// 全部缺省：直接全 execution 收集（dashboard 同款），保证 health check
	// 跟产品选择解耦，页面也能看到 <user-account> 实际关联的 story
	return s.fetchStoriesAcrossExecutions(60 * time.Second)
}

// applyStoryFilters 对需求列表应用查询条件（不含分页），列表与统计卡共用同一筛选口径
func applyStoryFilters(stories []zentaosdk.Story, query *dto.StoryQueryDTO) *utils.ChainFilter[zentaosdk.Story] {
	chainFilter := utils.NewChainFilter(stories)

	// 按指派人筛选
	if query.AssignedTo != "" {
		chainFilter = chainFilter.Filter(func(item zentaosdk.Story) bool {
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
		chainFilter = chainFilter.Filter(func(item zentaosdk.Story) bool {
			filtered := utils.FilterByDateRangeOrSpecific(
				[]zentaosdk.Story{item},
				query.StartDate,
				query.EndDate,
				query.SpecificDate,
				func(s zentaosdk.Story) string { return s.OpenedDate },
			)
			return len(filtered) > 0
		})
	}

	return chainFilter
}

// convertToVO 将zentao.Story转换为vo.StoryVO
func (s *StoryService) convertToVO(stories []zentaosdk.Story) []vo.StoryVO {
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

// GetStoryByID 按 ID 拉取需求详情（列表页 ?id= 落地直达详情弹窗，issue #5）。
func (s *StoryService) GetStoryByID(ctx context.Context, id int) (*zentaosdk.Story, error) {
	return s.client.GetStoryByID(ctx, id)
}

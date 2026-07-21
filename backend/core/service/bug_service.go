package service

import (
	"github.com/yi-nology/common/biz/zentao"

	"github.com/yi-nology/zentao-max/backend/core/dto"
	"github.com/yi-nology/zentao-max/backend/core/utils"
	"github.com/yi-nology/zentao-max/backend/core/vo"
	myzentao "github.com/yi-nology/zentao-max/backend/core/zentao"
)

// BugService Bug业务逻辑服务
// 负责处理Bug相关的业务逻辑
type BugService struct {
	client *myzentao.Client
}

// NewBugService 创建Bug服务
func NewBugService(client *myzentao.Client) *BugService {
	return &BugService{client: client}
}

// GetBugs 获取Bug列表
// 业务逻辑：
// 1. 根据产品ID查询Bug
// 2. 应用筛选条件（状态、指派人、版本、时间范围）
// 3. 分页处理
func (s *BugService) GetBugs(query *dto.BugQueryDTO) (*vo.PaginatedVO, error) {
	var bugs []zentao.Bug
	var err error

	// 如果有产品ID，按产品查询
	if query.ProductID != 0 {
		// 版本/类型/状态过滤需要获取所有bug（含closed），在内存中过滤
		// 注意：closed 状态的 bug 不会被禅道默认接口返回，
		// 必须用 status=all 全量获取（含 closed）后在内存过滤
		if query.Version != "" || query.Type != "" || query.Status != "" {
			bugs, err = s.client.GetAllBugsIncludeClosed(query.ProductID)
		} else if query.AssignedTo != "" {
			// 指派人过滤使用SearchBugs减少内存消耗
			params := zentao.BugSearchParams{
				ProductID:  query.ProductID,
				Status:     query.Status,
				AssignedTo: query.AssignedTo,
				Limit:      1000,
				Page:       1,
			}
			bugs, err = s.client.SearchBugs(params)
		} else if query.ProjectID != 0 {
			// 如果只有项目ID，使用GetBugsByProject
			bugs, err = s.client.GetBugsByProject(query.ProductID, query.ProjectID, 1, 1000)
		} else {
			// 获取产品的所有Bug
			bugs, err = s.client.GetBugs(query.ProductID, 1, 1000)
		}

		if err != nil {
			return nil, err
		}
	} else {
		// 如果没有产品ID，返回空列表
		bugs = []zentao.Bug{}
	}

	// 使用链式过滤器进行筛选
	chainFilter := utils.NewChainFilter(bugs)

	// 按状态筛选（version/type 分支下 bugs 包含全部状态，需在内存过滤）
	if query.Status != "" {
		chainFilter = chainFilter.Filter(func(item zentao.Bug) bool {
			return item.Status == query.Status
		})
	}

	// 按版本筛选（openedBuild 包含指定版本名称）
	if query.Version != "" {
		chainFilter = chainFilter.Filter(func(item zentao.Bug) bool {
			for _, build := range item.OpenedBuild {
				if build == query.Version {
					return true
				}
			}
			return false
		})
	}

	// 按类型筛选
	if query.Type != "" {
		chainFilter = chainFilter.Filter(func(item zentao.Bug) bool {
			return item.Type == query.Type
		})
	}

	// 按时间范围或具体日期筛选
	if query.StartDate != "" || query.EndDate != "" || query.SpecificDate != "" {
		chainFilter = chainFilter.Filter(func(item zentao.Bug) bool {
			filtered := utils.FilterByDateRangeOrSpecific(
				[]zentao.Bug{item},
				query.StartDate,
				query.EndDate,
				query.SpecificDate,
				func(b zentao.Bug) string { s, _ := b.OpenedDate.(string); return s },
			)
			return len(filtered) > 0
		})
	}

	// 获取总数
	total := chainFilter.Count()

	// 执行分页
	pagedBugs := chainFilter.Paginate(query.Page, query.PageSize).Result()

	list := s.convertToVO(pagedBugs)

	return &vo.PaginatedVO{
		List:     list,
		Total:    total,
		Page:     query.Page,
		PageSize: query.PageSize,
	}, nil
}

// convertToVO 将zentao.Bug转换为vo.BugVO
func (s *BugService) convertToVO(bugs []zentao.Bug) []vo.BugVO {
	if len(bugs) == 0 {
		return []vo.BugVO{}
	}

	result := make([]vo.BugVO, 0, len(bugs))
	for _, bug := range bugs {
		result = append(result, vo.BugVO{
			ID:            bug.ID,
			Project:       bug.Project,
			Product:       bug.Product,
			Title:         bug.Title,
			Keywords:      bug.Keywords,
			Severity:      bug.Severity,
			Pri:           bug.Pri,
			Type:          bug.Type,
			OS:            bug.OS,
			Browser:       bug.Browser,
			Hardware:      bug.Hardware,
			Steps:         bug.Steps,
			Status:        bug.Status,
			SubStatus:     bug.SubStatus,
			Color:         bug.Color,
			Confirmed:     bug.Confirmed,
			PlanTime:      bug.PlanTime,
			OpenedBy:      vo.UserRefVO(bug.OpenedBy),
			OpenedDate:    bug.OpenedDate,
			OpenedBuild:   bug.OpenedBuild,
			AssignedTo:    vo.UserRefVO(bug.AssignedTo),
			AssignedDate:  bug.AssignedDate,
			Deadline:      bug.Deadline,
			ResolvedBy:    bug.ResolvedBy,
			Resolution:    bug.Resolution,
			ResolvedBuild: bug.ResolvedBuild,
			ResolvedDate:  bug.ResolvedDate,
			ClosedBy:      bug.ClosedBy,
			ClosedDate:    bug.ClosedDate,
			StatusName:    bug.StatusName,
			LifeCycle:     bug.LifeCycle,
		})
	}
	return result
}

package service

import (
	zentaosdk "github.com/yi-nology/zentao-max/backend/internal/infra/zentao/sdk"

	"github.com/yi-nology/zentao-max/backend/internal/dto"
	"github.com/yi-nology/zentao-max/backend/pkg/utils"
	"github.com/yi-nology/zentao-max/backend/internal/vo"
	myzentao "github.com/yi-nology/zentao-max/backend/internal/infra/zentao"
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
	var bugs []zentaosdk.Bug
	var err error

	// 如果有产品ID，按产品查询。
	// 列表与统计卡同口径：统一走全量（含 closed）后内存过滤+分页。
	// 此前默认分支走 GetBugs（禅道默认 browseType 部分集合），导致
	// 列表"共 100 条"与统计分段卡（全量 1000）对不上。
	if query.ProductID != 0 {
		bugs, err = s.client.GetAllBugs(query.ProductID)
		if err != nil {
			return nil, err
		}
	} else {
		// 如果没有产品ID，返回空列表
		bugs = []zentaosdk.Bug{}
	}

	// 使用链式过滤器进行筛选
	chainFilter := applyBugFilters(bugs, query)

	// 获取总数
	total := chainFilter.Count()

	// 与禅道网页列表一致：默认按 ID 降序（最新在前），再分页
	pagedBugs := chainFilter.Sort(func(a, b zentaosdk.Bug) bool { return a.ID > b.ID }).Paginate(query.Page, query.PageSize).Result()

	list := s.convertToVO(pagedBugs)

	return &vo.PaginatedVO{
		List:     list,
		Total:    total,
		Page:     query.Page,
		PageSize: query.PageSize,
	}, nil
}

// applyBugFilters 对全量 bug 列表应用查询条件（不含分页），列表与统计卡共用同一筛选口径
func applyBugFilters(bugs []zentaosdk.Bug, query *dto.BugQueryDTO) *utils.ChainFilter[zentaosdk.Bug] {
	chainFilter := utils.NewChainFilter(bugs)

	// 按状态筛选（version/type 分支下 bugs 包含全部状态，需在内存过滤）
	if query.Status != "" {
		chainFilter = chainFilter.Filter(func(item zentaosdk.Bug) bool {
			return item.Status == query.Status
		})
	}

	// 按指派人筛选（AssignedTo 是 UserRef，按账号精确匹配）
	if query.AssignedTo != "" {
		chainFilter = chainFilter.Filter(func(item zentaosdk.Bug) bool {
			return item.AssignedTo.Account == query.AssignedTo
		})
	}

	// 按版本筛选（openedBuild 包含指定版本名称）
	if query.Version != "" {
		chainFilter = chainFilter.Filter(func(item zentaosdk.Bug) bool {
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
		chainFilter = chainFilter.Filter(func(item zentaosdk.Bug) bool {
			return item.Type == query.Type
		})
	}

	// 按时间范围或具体日期筛选
	if query.StartDate != "" || query.EndDate != "" || query.SpecificDate != "" {
		chainFilter = chainFilter.Filter(func(item zentaosdk.Bug) bool {
			filtered := utils.FilterByDateRangeOrSpecific(
				[]zentaosdk.Bug{item},
				query.StartDate,
				query.EndDate,
				query.SpecificDate,
				func(b zentaosdk.Bug) string { s, _ := b.OpenedDate.(string); return s },
			)
			return len(filtered) > 0
		})
	}

	return chainFilter
}

// GetBugStatusCounts 返回应用筛选条件后（不分页）的各状态 Bug 数量，供统计卡使用。
// 统计基于含 closed 的全量数据，否则"已关闭"永远是 0。
func (s *BugService) GetBugStatusCounts(query *dto.BugQueryDTO) (map[string]int, error) {
	counts := map[string]int{"active": 0, "resolved": 0, "closed": 0}
	if query.ProductID == 0 {
		return counts, nil
	}

	bugs, err := s.client.GetAllBugs(query.ProductID)
	if err != nil {
		return nil, err
	}

	for _, bug := range applyBugFilters(bugs, query).Result() {
		if _, ok := counts[bug.Status]; ok {
			counts[bug.Status]++
		}
	}
	return counts, nil
}

// GetBugsWithStatusCounts 列表 + 状态统计一次全量拉取合并返回。
// 此前 REST handler 分调 GetBugs 与 GetBugStatusCounts，冷缓存时同一产品
// 全量真拉两遍（实测 2×9.7s=18.7s）；合并后减半。
func (s *BugService) GetBugsWithStatusCounts(query *dto.BugQueryDTO) (*vo.PaginatedVO, map[string]int, error) {
	counts := map[string]int{"active": 0, "resolved": 0, "closed": 0}
	if query.ProductID == 0 {
		empty := &vo.PaginatedVO{List: []vo.BugVO{}, Page: query.Page, PageSize: query.PageSize}
		return empty, counts, nil
	}

	bugs, err := s.client.GetAllBugs(query.ProductID)
	if err != nil {
		return nil, nil, err
	}

	chainFilter := applyBugFilters(bugs, query)

	// ⚠️ 统计必须在 Sort/Paginate 之前取：ChainFilter 的 Sort/Paginate 原地修改
	// cf.data（PaginateSlice 截断成本页），分页后 Result 只剩本页条目——
	// 曾把 statusCounts 算成当前页分布（total 936 而全部分段之和=pageSize）。
	filtered := chainFilter.Result()
	total := chainFilter.Count()
	for _, bug := range filtered {
		if _, ok := counts[bug.Status]; ok {
			counts[bug.Status]++
		}
	}

	// 与禅道网页列表一致：默认按 ID 降序（最新在前），再分页（原地截断）
	pagedBugs := chainFilter.Sort(func(a, b zentaosdk.Bug) bool { return a.ID > b.ID }).Paginate(query.Page, query.PageSize).Result()

	return &vo.PaginatedVO{
		List:     s.convertToVO(pagedBugs),
		Total:    total,
		Page:     query.Page,
		PageSize: query.PageSize,
	}, counts, nil
}

// convertToVO 将zentao.Bug转换为vo.BugVO
func (s *BugService) convertToVO(bugs []zentaosdk.Bug) []vo.BugVO {
	if len(bugs) == 0 {
		return []vo.BugVO{}
	}

	result := make([]vo.BugVO, 0, len(bugs))
	for _, bug := range bugs {
		result = append(result, vo.BugVO{
			ID:            bug.ID,
			Project:       bug.Project,
			Product:       bug.Product,
			Execution:     bug.Execution,
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

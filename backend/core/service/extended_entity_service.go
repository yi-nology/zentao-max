package service

import (
	zentaosdk "github.com/yi-nology/zentao-max/backend/core/zentao/sdk"

	"github.com/yi-nology/zentao-max/backend/core/vo"
	myzentao "github.com/yi-nology/zentao-max/backend/core/zentao"
)

// 本文件汇总 Phase2b 新增实体的 Service（cases/plans/programs/releases/testtasks）。
// 每个 Service 都很轻量：调 wrapper 方法 + 转 VO + 内存分页。

// ---- CaseService ----

type CaseService struct{ client *myzentao.Client }

func NewCaseService(client *myzentao.Client) *CaseService { return &CaseService{client: client} }

func (s *CaseService) GetCasesByProduct(productID, page, pageSize int) (*vo.PaginatedVO, error) {
	cases, recTotal, err := s.client.GetCasesByProduct(productID, page, pageSize)
	if err != nil {
		return nil, err
	}
	all := s.convertCases(cases)
	// 用禅道端点返回的 recTotal 作为真实总数；首探 recTotal=0 时回退到当前页长度。
	total := recTotal
	if total <= 0 {
		total = len(all)
	}
	return &vo.PaginatedVO{List: all, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *CaseService) convertCases(in []zentaosdk.Case) []vo.CaseVO {
	out := make([]vo.CaseVO, 0, len(in))
	for _, c := range in {
		out = append(out, vo.CaseVO{
			ID: c.ID, Product: c.Product, Project: c.Project, Module: c.Module,
			Story: c.Story, Title: c.Title, Precondition: c.Precondition,
			Keywords: c.Keywords, Pri: c.Pri, Type: c.Type, Status: c.Status,
			OpenedBy: vo.UserRefVO(c.OpenedBy), OpenedDate: c.OpenedDate, Version: c.Version,
		})
	}
	return out
}

// ---- PlanService ----

type PlanService struct{ client *myzentao.Client }

func NewPlanService(client *myzentao.Client) *PlanService { return &PlanService{client: client} }

func (s *PlanService) GetPlans(productID, page, pageSize int) (*vo.PaginatedVO, error) {
	plans, err := s.client.GetPlans(productID, page, pageSize)
	if err != nil {
		return nil, err
	}
	all := make([]vo.PlanVO, 0, len(plans))
	for _, p := range plans {
		all = append(all, vo.PlanVO{
			ID: p.ID, Product: p.Product, Parent: p.Parent, Title: p.Title,
			Desc: p.Desc, Begin: p.Begin, End: p.End, Status: p.Status, ClosedBy: p.ClosedBy,
		})
	}
	return &vo.PaginatedVO{List: all, Total: len(all), Page: page, PageSize: pageSize}, nil
}

// ---- ProgramService ----

type ProgramService struct{ client *myzentao.Client }

func NewProgramService(client *myzentao.Client) *ProgramService {
	return &ProgramService{client: client}
}

func (s *ProgramService) GetPrograms(page, pageSize int) (*vo.PaginatedVO, error) {
	programs, err := s.client.GetPrograms(page, pageSize)
	if err != nil {
		return nil, err
	}
	all := make([]vo.ProgramVO, 0, len(programs))
	for _, p := range programs {
		all = append(all, vo.ProgramVO{
			ID: p.ID, Name: p.Name, Code: p.Code, Parent: p.Parent,
			Type: p.Type, Status: p.Status, Begin: p.Begin, End: p.End, Desc: p.Desc,
		})
	}
	return &vo.PaginatedVO{List: all, Total: len(all), Page: page, PageSize: pageSize}, nil
}

// ---- ReleaseService ----

type ReleaseService struct{ client *myzentao.Client }

func NewReleaseService(client *myzentao.Client) *ReleaseService {
	return &ReleaseService{client: client}
}

func (s *ReleaseService) GetReleasesByProduct(productID, page, pageSize int) (*vo.PaginatedVO, error) {
	releases, err := s.client.GetReleasesByProduct(productID, page, pageSize)
	if err != nil {
		return nil, err
	}
	all := make([]vo.ReleaseVO, 0, len(releases))
	for _, r := range releases {
		all = append(all, vo.ReleaseVO{
			ID: r.ID, Product: r.Product, Build: r.Build, Name: r.Name, Marker: r.Marker,
			Date: r.Date, Stories: r.Stories, Bugs: r.Bugs, Desc: r.Desc,
			Status: r.Status, SubStatus: r.SubStatus,
		})
	}
	return &vo.PaginatedVO{List: all, Total: len(all), Page: page, PageSize: pageSize}, nil
}

// ---- TestTaskService ----

type TestTaskService struct{ client *myzentao.Client }

func NewTestTaskService(client *myzentao.Client) *TestTaskService {
	return &TestTaskService{client: client}
}

func (s *TestTaskService) GetTestTasksByProduct(productID, page, pageSize int) (*vo.PaginatedVO, error) {
	tasks, err := s.client.GetTestTasksByProduct(productID, page, pageSize)
	if err != nil {
		return nil, err
	}
	all := make([]vo.TestTaskVO, 0, len(tasks))
	for _, t := range tasks {
		all = append(all, vo.TestTaskVO{
			ID: t.ID, Project: t.Project, Product: t.Product, Name: t.Name,
			Execution: t.Execution, Build: t.Build, Type: t.Type,
			Owner: vo.UserRefVO(t.Owner), Pri: t.Pri, Begin: t.Begin, End: t.End, Status: t.Status,
		})
	}
	return &vo.PaginatedVO{List: all, Total: len(all), Page: page, PageSize: pageSize}, nil
}

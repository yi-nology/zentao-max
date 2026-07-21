package zentao

import (
	"context"
	"fmt"

	"github.com/yi-nology/common/biz/zentao"
)

// ---- Plans (session mode) ----
// 端点：/productplan-browse-{productID}.json，内层 plans 是 list。

type sessionPlanBrowseResp struct {
	Title string        `json:"title"`
	Plans []sessionPlan `json:"plans"`
	Pager *sessionPager `json:"pager"`
}

type sessionPlan struct {
	ID       int    `json:"id"`
	Product  int    `json:"product"`
	Parent   int    `json:"parent"`
	Title    string `json:"title"`
	Desc     string `json:"desc"`
	Begin    string `json:"begin"`
	End      string `json:"end"`
	Status   string `json:"status"`
	ClosedBy string `json:"closedBy"`
}

func (p sessionPlan) toSDK() zentao.Plan {
	return zentao.Plan{
		ID:       p.ID,
		Product:  p.Product,
		Parent:   p.Parent,
		Title:    p.Title,
		Desc:     p.Desc,
		Begin:    p.Begin,
		End:      p.End,
		Status:   p.Status,
		ClosedBy: p.ClosedBy,
	}
}

func (c *Client) getPlansSession(ctx context.Context, productID, page, pageSize int) ([]zentao.Plan, error) {
	path := fmt.Sprintf("/productplan-browse-%d.json", productID)
	var resp sessionPlanBrowseResp
	if err := c.doSessionJSON(ctx, "GetPlans", path, &resp); err != nil {
		if IsSessionAccessDenied(err) {
			return nil, nil
		}
		return nil, err
	}
	all := make([]zentao.Plan, 0, len(resp.Plans))
	for _, p := range resp.Plans {
		all = append(all, p.toSDK())
	}
	if pageSize <= 0 {
		return all, nil
	}
	return paginateSlice(all, page, pageSize), nil
}

// ---- Programs (session mode) ----
// 端点：/program-browse.json，内层 programs 是 map[id]program。

type sessionProgramBrowseResp struct {
	Title    string                    `json:"title"`
	Programs map[string]sessionProgram `json:"programs"`
	Pager    *sessionPager             `json:"pager"`
}

type sessionProgram struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Code   string `json:"code"`
	Parent int    `json:"parent"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Begin  string `json:"begin"`
	End    string `json:"end"`
	Desc   string `json:"desc"`
	PM     string `json:"PM"`
}

func (p sessionProgram) toSDK() zentao.Program {
	return zentao.Program{
		ID:     p.ID,
		Name:   p.Name,
		Code:   p.Code,
		Parent: p.Parent,
		Type:   p.Type,
		Status: p.Status,
		Begin:  p.Begin,
		End:    p.End,
		Desc:   p.Desc,
		PM:     p.PM,
	}
}

func (c *Client) getProgramsSession(ctx context.Context, page, pageSize int) ([]zentao.Program, error) {
	var resp sessionProgramBrowseResp
	if err := c.doSessionJSON(ctx, "GetPrograms", "/program-browse.json", &resp); err != nil {
		if IsSessionAccessDenied(err) {
			return nil, nil
		}
		return nil, err
	}
	all := make([]zentao.Program, 0, len(resp.Programs))
	for _, p := range resp.Programs {
		all = append(all, p.toSDK())
	}
	if pageSize <= 0 {
		return all, nil
	}
	return paginateSliceProgram(all, page, pageSize), nil
}

// ---- Releases (session mode) ----
// 端点：/release-browse-{productID}.json，内层 releases 是 list。

type sessionReleaseBrowseResp struct {
	Title    string           `json:"title"`
	Releases []sessionRelease `json:"releases"`
	Pager    *sessionPager    `json:"pager"`
}

type sessionRelease struct {
	ID        int    `json:"id"`
	Product   int    `json:"product"`
	Build     int    `json:"build"`
	Name      string `json:"name"`
	Marker    string `json:"marker"`
	Date      string `json:"date"`
	Stories   string `json:"stories"`
	Bugs      string `json:"bugs"`
	Desc      string `json:"desc"`
	Status    string `json:"status"`
	SubStatus string `json:"subStatus"`
}

func (r sessionRelease) toSDK() zentao.Release {
	return zentao.Release{
		ID:        r.ID,
		Product:   r.Product,
		Build:     r.Build,
		Name:      r.Name,
		Marker:    r.Marker,
		Date:      r.Date,
		Stories:   r.Stories,
		Bugs:      r.Bugs,
		Desc:      r.Desc,
		Status:    r.Status,
		SubStatus: r.SubStatus,
	}
}

func (c *Client) getReleasesByProductSession(ctx context.Context, productID, page, pageSize int) ([]zentao.Release, error) {
	path := fmt.Sprintf("/release-browse-%d.json", productID)
	var resp sessionReleaseBrowseResp
	if err := c.doSessionJSON(ctx, "GetReleasesByProduct", path, &resp); err != nil {
		if IsSessionAccessDenied(err) {
			return nil, nil
		}
		return nil, err
	}
	all := make([]zentao.Release, 0, len(resp.Releases))
	for _, r := range resp.Releases {
		all = append(all, r.toSDK())
	}
	if pageSize <= 0 {
		return all, nil
	}
	return paginateSliceRelease(all, page, pageSize), nil
}

// 分页辅助（泛型在 Go 1.18+ 可用，但为避免每个 SDK 类型都写一份，这里用具体类型）。
func paginateSlice[T any](in []T, page, pageSize int) []T { return doPaginate(in, page, pageSize) }
func paginateSliceProgram(in []zentao.Program, page, pageSize int) []zentao.Program {
	return doPaginate(in, page, pageSize)
}
func paginateSliceRelease(in []zentao.Release, page, pageSize int) []zentao.Release {
	return doPaginate(in, page, pageSize)
}

func doPaginate[T any](in []T, page, pageSize int) []T {
	if pageSize <= 0 {
		pageSize = 100
	}
	if page <= 0 {
		page = 1
	}
	start := (page - 1) * pageSize
	if start >= len(in) {
		return nil
	}
	end := start + pageSize
	if end > len(in) {
		end = len(in)
	}
	return in[start:end]
}

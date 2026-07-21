package dto

import "fmt"

// DashboardQuery 仪表盘查询参数
type DashboardQuery struct {
	ProductID int    `form:"productId" json:"productId"`
	StartDate string `form:"startDate" json:"startDate"` // 起始日期（含），格式 YYYY-MM-DD，为空表示不限
	EndDate   string `form:"endDate" json:"endDate"`     // 截止日期（含），格式 YYYY-MM-DD，为空表示不限
}

// Validate 校验查询参数：日期格式必须是 YYYY-MM-DD（如果提供）
func (q *DashboardQuery) Validate() error {
	if !isValidDateFormat(q.StartDate) {
		return fmt.Errorf("startDate 格式必须是 YYYY-MM-DD")
	}
	if !isValidDateFormat(q.EndDate) {
		return fmt.Errorf("endDate 格式必须是 YYYY-MM-DD")
	}
	if q.StartDate != "" && q.EndDate != "" && q.StartDate > q.EndDate {
		return fmt.Errorf("startDate 不能晚于 endDate")
	}
	return nil
}

// isValidDateFormat 校验 YYYY-MM-DD 格式（允许空字符串）
func isValidDateFormat(s string) bool {
	if s == "" {
		return true
	}
	if len(s) != 10 || s[4] != '-' || s[7] != '-' {
		return false
	}
	if _, err := fmt.Sscanf(s, "%4d-%2d-%2d", new(int), new(int), new(int)); err != nil {
		return false
	}
	return true
}

// ProjectOverviewQuery 项目概览查询参数
type ProjectOverviewQuery struct {
	ProjectID int `form:"projectId" json:"projectId" binding:"required"`
}

// PersonalTimelogQuery 个人工时报表查询参数
type PersonalTimelogQuery struct {
	Account   string `form:"account" json:"account"`
	ProductID int    `form:"productId" json:"productId"`
	DateFrom  string `form:"dateFrom" json:"dateFrom"`
	DateTo    string `form:"dateTo" json:"dateTo"`
	GroupBy   string `form:"groupBy" json:"groupBy"` // day, week, month
}

// SearchQuery 全局搜索查询参数
type SearchQuery struct {
	Keyword   string `form:"keyword" json:"keyword" binding:"required"`
	ProductID int    `form:"productId" json:"productId"`
	Page      int    `form:"page" json:"page"`
	PageSize  int    `form:"pageSize" json:"pageSize"`
}

func (q *SearchQuery) Validate() error {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 20
	}
	if q.PageSize > MaxPageSize {
		q.PageSize = MaxPageSize
	}
	return nil
}

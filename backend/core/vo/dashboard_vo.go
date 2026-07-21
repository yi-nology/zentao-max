package vo

// DashboardVO 仪表盘响应
type DashboardVO struct {
	Bugs        BugStatsVO       `json:"bugs"`
	Stories     StoryStatsVO     `json:"stories"`
	Tasks       TaskStatsVO      `json:"tasks"`
	Timelog     TimelogSummaryVO `json:"timelog"`
	RecentBugs  []BugVO          `json:"recentBugs"`
	RecentTasks []TaskVO         `json:"recentTasks"`

	// 缓存元信息（前端用于显示"离线/缓存"徽标）
	FromCache bool `json:"fromCache,omitempty"` // 数据是否来自本地缓存
	Stale     bool `json:"stale,omitempty"`     // 是否来自过期缓存（离线兜底）
}

// BugStatsVO Bug 统计
type BugStatsVO struct {
	Total      int            `json:"total"`
	Active     int            `json:"active"`
	Resolved   int            `json:"resolved"`
	Closed     int            `json:"closed"`
	BySeverity map[string]int `json:"bySeverity"`
	ByType     map[string]int `json:"byType"`
}

// StoryStatsVO 需求统计
type StoryStatsVO struct {
	Total   int            `json:"total"`
	Draft   int            `json:"draft"`
	Active  int            `json:"active"`
	Closed  int            `json:"closed"`
	ByStage map[string]int `json:"byStage"`
}

// TaskStatsVO 任务统计
type TaskStatsVO struct {
	Total  int `json:"total"`
	Wait   int `json:"wait"`
	Doing  int `json:"doing"`
	Done   int `json:"done"`
	Closed int `json:"closed"`
}

// TimelogSummaryVO 工时汇总
type TimelogSummaryVO struct {
	TotalHours    float64          `json:"totalHours"`
	ThisWeekHours float64          `json:"thisWeekHours"`
	ByProject     []ProjectHoursVO `json:"byProject"`
}

// ProjectHoursVO 项目工时
type ProjectHoursVO struct {
	ProjectID   int     `json:"projectId"`
	ProjectName string  `json:"projectName"`
	Hours       float64 `json:"hours"`
}

// ProjectOverviewVO 项目概览
type ProjectOverviewVO struct {
	Project    ProjectInfoVO `json:"project"`
	Executions []ExecutionVO `json:"executions"`
	Bugs       BugStatsVO    `json:"bugs"`
	Stories    StoryStatsVO  `json:"stories"`
	Tasks      TaskStatsVO   `json:"tasks"`
}

// ProjectInfoVO 项目基本信息
type ProjectInfoVO struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Code   string `json:"code"`
	Status string `json:"status"`
	Begin  string `json:"begin"`
	End    string `json:"end"`
}

// PersonalTimelogVO 个人工时报表
type PersonalTimelogVO struct {
	TotalHours float64          `json:"totalHours"`
	ByDate     []DateHoursVO    `json:"byDate"`
	ByProject  []ProjectHoursVO `json:"byProject"`
	Details    []TimelogEntryVO `json:"details"`
}

// DateHoursVO 按日期工时
type DateHoursVO struct {
	Date  string  `json:"date"`
	Hours float64 `json:"hours"`
}

// TimelogEntryVO 工时明细
type TimelogEntryVO struct {
	ID          int     `json:"id"`
	Work        string  `json:"work"`
	Date        string  `json:"date"`
	Consumed    float64 `json:"consumed"`
	ProjectID   int     `json:"projectId"`
	ProjectName string  `json:"projectName"`
}

// SearchVO 搜索结果
type SearchVO struct {
	Total int          `json:"total"`
	Items []SearchItem `json:"items"`
}

// SearchItem 搜索结果项
type SearchItem struct {
	Type    string      `json:"type"` // bug, story, task
	ID      int         `json:"id"`
	Title   string      `json:"title"`
	Status  string      `json:"status"`
	Project string      `json:"project"`
	Extra   interface{} `json:"extra"`
}

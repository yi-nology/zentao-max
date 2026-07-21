package vo

// 本文件汇总 Phase2b 新增实体的 VO（cases/plans/programs/releases/testtasks）。
// VO 是面向前端的表现层对象，只暴露业务需要的字段。

// ---- Case ----
type CaseVO struct {
	ID           int       `json:"id"`
	Product      int       `json:"product"`
	Project      int       `json:"project"`
	Module       int       `json:"module"`
	Story        int       `json:"story"`
	Title        string    `json:"title"`
	Precondition string    `json:"precondition"`
	Keywords     string    `json:"keywords"`
	Pri          int       `json:"pri"`
	Type         string    `json:"type"`
	Status       string    `json:"status"`
	OpenedBy     UserRefVO `json:"openedBy"`
	OpenedDate   string    `json:"openedDate"`
	Version      int       `json:"version"`
}

// ---- Plan ----
type PlanVO struct {
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

// ---- Program ----
type ProgramVO struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Code   string `json:"code"`
	Parent int    `json:"parent"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Begin  string `json:"begin"`
	End    string `json:"end"`
	Desc   string `json:"desc"`
	PM     string `json:"pm"`
}

// ---- Release ----
type ReleaseVO struct {
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

// ---- TestTask ----
type TestTaskVO struct {
	ID        int       `json:"id"`
	Project   int       `json:"project"`
	Product   int       `json:"product"`
	Name      string    `json:"name"`
	Execution int       `json:"execution"`
	Build     int       `json:"build"`
	Type      string    `json:"type"`
	Owner     UserRefVO `json:"owner"`
	Pri       int       `json:"pri"`
	Begin     string    `json:"begin"`
	End       string    `json:"end"`
	Status    string    `json:"status"`
}

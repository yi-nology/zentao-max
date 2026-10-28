package zentao

// ========== 测试单(TestTask)相关 ==========

// TestTask 测试单结构
type TestTask struct {
	ID               int       `json:"id"`
	Project          int       `json:"project"`
	Product          int       `json:"product"`
	Name             string    `json:"name"`
	Execution        int       `json:"execution"`
	Build            int       `json:"build"`
	Type             string    `json:"type"`
	Owner            UserRef   `json:"owner"`
	Pri              int       `json:"pri"`
	Begin            string    `json:"begin"`
	End              string    `json:"end"`
	RealFinishedDate *string   `json:"realFinishedDate"`
	Mailto           string    `json:"mailto"`
	Desc             string    `json:"desc"`
	Report           string    `json:"report"`
	Status           string    `json:"status"`
	TestReport       int       `json:"testreport"`
	Auto             string    `json:"auto"`
	SubStatus        string    `json:"subStatus"`
	Deleted          string    `json:"deleted"`
	ProductName       string   `json:"productName"`
	ExecutionName     string   `json:"executionName"`
	BuildName         string   `json:"buildName"`
	Branch           int       `json:"branch"`
}

// TestTaskListResponse 获取测试单列表响应
type TestTaskListResponse struct {
	Page      int         `json:"page"`
	Total     int         `json:"total"`
	Limit     int         `json:"limit"`
	Testtasks []TestTask `json:"testtasks"`
}

// ProjectTestTaskListResponse 获取项目测试单列表响应
type ProjectTestTaskListResponse struct {
	Page      int         `json:"page"`
	Total     int         `json:"total"`
	Limit     int         `json:"limit"`
	Testtasks []TestTask `json:"testtasks"`
}

// ========== 测试单(TestTask) API 方法 ==========

// GetTestTasks 获取测试单列表（支持分页）

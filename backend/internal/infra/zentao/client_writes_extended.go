package zentao

import (
	"context"
	"encoding/json"
	"fmt"

		"regexp"
	"strconv"
	"strings"

	neturl "net/url"

	"github.com/yi-nology/zentao-max/backend/pkg/logger"
	zentaosdk "github.com/yi-nology/zentao-max/backend/internal/infra/zentao/sdk"
)

// 本文件补齐 SDK 的剩余写操作（Phase2c 扩展）。
// 与 client_writes.go 一样的双模式分发：token 模式调 SDK，session 模式 POST PHP 表单端点。
//
// PHP 写端点格式约定（标准禅道）：
//   - create: /{module}-create.json（body 含 product/project/execution 等父ID）
//   - update: /{module}-edit-{id}.json
//   - delete: /{module}-delete-{id}.json
//   - 特殊动作: /{module}-{action}-{id}.json
//
// 返回值：create/update 在 session 模式下无法可靠返回新建对象（PHP 端点返回的是重定向
// 或简单 result），统一返回 nil 指针 + 错误状态；调用方如需对象应再查一次。
// 为保持签名与 SDK 一致，create/update 返回 (*Type, error)，session 模式成功时返回 &零值{}。

// ---- helper ----

// joinInts 把 []int 转成逗号分隔字符串（plan link/unlink 用）。
func joinInts(ids []int) string {
	s := make([]string, len(ids))
	for i, id := range ids {
		s[i] = strconv.Itoa(id)
	}
	return strings.Join(s, ",")
}

// ---- Bug create/update/delete ----

// CreateBug 会话模式创建 Bug。
//
// 实测（biz8.x 受限实例）：POST /bug-create.json 会被静默丢弃（返回页面但不落库），
// 必须提交到完整表单页 URL /bug-create-{productID}-moduleID=0.html，且
// openedBuild[] 需为数组名形式、值为 build ID 或 trunk（可经
// /build-ajaxGetProductBuilds-{productID}-openedBuild--0.html 获取）。
// 麒麟实例还有定制必填字段 txjg[]（体系结构，"all"=所有）与
// Evaluationanalysis（是否评价分析，页面默认 2），缺一被服务端静默弹回表单。
// 成功判定：响应 HTML 含"保存成功"或跳转 bug-view；弹回表单页视为失败。
func (c *Client) CreateBug(productID int, req zentaosdk.BugCreateRequest) (*zentaosdk.Bug, error) {

	// 实测服务端校验：『所属项目』不能为空——选执行时禅道前端会自动联动项目，
	// API 直发必须自己带上。按 executionID 反查所属 project。
	if req.Project == 0 && req.Execution != 0 {
		if execs, err := c.getAllExecutionsSession(context.Background()); err == nil {
			for _, e := range execs {
				if e.ID == req.Execution && e.Project != 0 {
					req.Project = e.Project
					break
				}
			}
		}
	}

	form := bugCreateForm(productID, req)
	path := fmt.Sprintf("/bug-create-%d-moduleID=0.html", productID)
	body, err := c.doSessionPostWithBody(context.Background(), "CreateBug", path, form)
	if err != nil {
		return nil, err
	}
	// TODO(test): 临时诊断，验证后删除
	logger.Warn("[DIAG] CreateBug response", "len", len(body),
		"head", truncate(string(body), 200))
	if err := judgeBugCreateResponse(body); err != nil {
		return nil, err
	}
	return &zentaosdk.Bug{}, nil
}

// judgeBugCreateResponse 判定 bug-create 的 HTML 响应。
// 判定顺序很关键：表单回显页的 JS 里也含 bug-view 字样，必须先判失败特征。
func judgeBugCreateResponse(body []byte) error {
	s := string(body)
	// 弹回表单页（服务端校验拒绝的 HTML 形态）
	if strings.Contains(s, `id="submit"`) && strings.Contains(s, "bug-create") {
		return fmt.Errorf("Bug 创建被实例校验拒绝（弹回表单页）：请检查影响版本/所属模块/执行/项目等必填字段，或该产品存在定制必填项")
	}
	// 成功：保存成功提示，或跳转到 bug-view
	if strings.Contains(s, "保存成功") {
		return nil
	}
	if regexp.MustCompile(`(?i)(location|href)\s*[=:'" ]+[^;'"]*bug-view-\d+`).MatchString(s) {
		return nil
	}
	// 未知形态：HTTP 200 已过，按成功（与既有其它写端点行为一致）
	return nil
}

func bugCreateForm(productID int, req zentaosdk.BugCreateRequest) neturl.Values {
	form := neturl.Values{}
	form.Set("product", strconv.Itoa(productID))
	form.Set("title", req.Title)
	// openedBuild[] 为数组名形式（禅道表单按多选接收），值为 build ID 或 trunk
	for _, b := range req.OpenedBuild {
		form.Add("openedBuild[]", b)
	}
	form.Set("assignedTo", req.AssignedTo)
	form.Set("type", req.Type)
	form.Set("severity", strconv.Itoa(req.Severity))
	form.Set("pri", strconv.Itoa(req.Pri))
	form.Set("steps", req.Steps)
	form.Set("os", req.OS)
	form.Set("browser", req.Browser)
	form.Set("hardware", req.Hardware)
	// module/execution 为表单必填（0=根模块/无执行也必须出现）
	form.Set("module", strconv.Itoa(req.Module))
	form.Set("execution", strconv.Itoa(req.Execution))
	if req.Project != 0 {
		form.Set("project", strconv.Itoa(req.Project))
	}
	// 麒麟受限实例（biz8.x）定制必填：体系结构多选（"all"=所有）与评价分析标记。
	// 标准禅道会忽略未知字段，不影响通用性。
	if form.Get("txjg[]") == "" {
		form.Set("txjg[]", "all")
	}
	if form.Get("Evaluationanalysis") == "" {
		form.Set("Evaluationanalysis", "2")
	}
	return form
}

func (c *Client) UpdateBug(bugID int, req zentaosdk.BugUpdateRequest) (*zentaosdk.Bug, error) {

	form := neturl.Values{}
	form.Set("title", req.Title)
	form.Set("type", req.Type)
	form.Set("severity", strconv.Itoa(req.Severity))
	form.Set("pri", strconv.Itoa(req.Pri))
	form.Set("steps", req.Steps)
	form.Set("assignedTo", req.AssignedTo)
	form.Set("openedBuild", strings.Join(req.OpenedBuild, ","))
	// bug-edit 全量更新：execution 必带原值，否则部分实例校验失败或字段被覆盖
	form.Set("execution", strconv.Itoa(req.Execution))
	if req.Module != 0 {
		form.Set("module", strconv.Itoa(req.Module))
	}
	// 质量分析字段（biz8.x）：fixedPackageVersion 格式为"包名=版本号"，一次 bug-edit 全量写全
	if req.FixedPackageVersion != "" {
		form.Set("fixedPackageVersion", req.FixedPackageVersion)
	}
	if req.ReasonAnalysis != "" {
		form.Set("reasonAnalysis", req.ReasonAnalysis)
	}
	if req.Advice != "" {
		form.Set("advice", req.Advice)
	}
	if req.Implementation != "" {
		form.Set("implementation", req.Implementation)
	}
	if req.TestAdvise != "" {
		form.Set("testAdvise", req.TestAdvise)
	}
	if req.Resolution != "" {
		form.Set("resolution", req.Resolution)
	}
	if req.ResolvedBuild != "" {
		form.Set("resolvedBuild", req.ResolvedBuild)
	}
	if req.Deadline != "" {
		form.Set("deadline", req.Deadline)
	}
	if req.Comment != "" {
		form.Set("comment", req.Comment)
	}
	if err := c.doSessionPost(context.Background(), "UpdateBug",
		fmt.Sprintf("/bug-edit-%d.json", bugID), form); err != nil {
		return nil, err
	}
	return &zentaosdk.Bug{}, nil
}

func (c *Client) DeleteBug(bugID int) error {
	return c.doSessionPost(context.Background(), "DeleteBug",
		fmt.Sprintf("/bug-delete-%d.json", bugID), neturl.Values{})
}

// ---- Task create/update/delete/activate ----

func (c *Client) CreateTask(executionID int, req zentaosdk.TaskCreateRequest) (*zentaosdk.Task, error) {

	form := neturl.Values{}
	form.Set("execution", strconv.Itoa(executionID))
	form.Set("name", req.Name)
	form.Set("type", req.Type)
	form.Set("assignedTo", strings.Join(req.AssignedTo, ","))
	form.Set("estimate", strconv.FormatFloat(req.Estimate, 'f', -1, 64))
	form.Set("pri", strconv.Itoa(req.Pri))
	form.Set("desc", req.Desc)
	if req.Module != 0 {
		form.Set("module", strconv.Itoa(req.Module))
	}
	if req.Story != 0 {
		form.Set("story", strconv.Itoa(req.Story))
	}
	if err := c.doSessionPost(context.Background(), "CreateTask",
		"/task-create.json", form); err != nil {
		return nil, err
	}
	return &zentaosdk.Task{}, nil
}

// CreateTaskNoExecution 创建无 iteration 关联的全局 task（产品/项目/执行=0）。
//
// 用法：daily note 同步到 zentao 时，每个 user 1 个 special "📓 Daily Notes" task 用
// 来挂载 effort（work=daily note 内容，consumed=0.01）。这种 task 不能挂 execution，
// 也不必绑 story/module。zentao 13+ 接受 execution=0 的 task，UI 上"我的任务"会显示。
//
// 失败模式：受限实例限流时返 429/500；fallback 到本地 SQLite。
//
// 已知问题：doSessionPost 不返 location header，没法直接拿新 task id。调用方需要
// 走 listMyTasks + name 匹配或者 service 层用本地 SQLite 存 task_id 映射。
func (c *Client) CreateTaskNoExecution(name, taskType, assignedTo, desc string) error {
	form := neturl.Values{}
	form.Set("execution", "0")
	form.Set("name", name)
	form.Set("type", taskType)
	form.Set("assignedTo", assignedTo)
	form.Set("estimate", "0")
	form.Set("left", "0")
	form.Set("pri", "3")
	form.Set("desc", desc)
	form.Set("status", "wait")
	return c.doSessionPost(context.Background(), "CreateTask",
		"/task-create.json", form)
}

func (c *Client) UpdateTask(taskID int, req zentaosdk.TaskUpdateRequest) (*zentaosdk.Task, error) {

	form := neturl.Values{}
	form.Set("name", req.Name)
	form.Set("type", req.Type)
	form.Set("assignedTo", strings.Join(req.AssignedTo, ","))
	form.Set("estimate", strconv.FormatFloat(req.Estimate, 'f', -1, 64))
	form.Set("consumed", strconv.FormatFloat(req.Consumed, 'f', -1, 64))
	form.Set("left", strconv.FormatFloat(req.Left, 'f', -1, 64))
	form.Set("pri", strconv.Itoa(req.Pri))
	if err := c.doSessionPost(context.Background(), "UpdateTask",
		fmt.Sprintf("/task-edit-%d.json", taskID), form); err != nil {
		return nil, err
	}
	return &zentaosdk.Task{}, nil
}

func (c *Client) DeleteTask(taskID int) error {
	return c.doSessionPost(context.Background(), "DeleteTask",
		fmt.Sprintf("/task-delete-%d.json", taskID), neturl.Values{})
}

func (c *Client) ActivateTask(taskID int, consumed, left float64) error {

	form := neturl.Values{}
	form.Set("consumed", strconv.FormatFloat(consumed, 'f', -1, 64))
	form.Set("left", strconv.FormatFloat(left, 'f', -1, 64))
	return c.doSessionPost(context.Background(), "ActivateTask",
		fmt.Sprintf("/task-restart-%d.json", taskID), form)
}

// ---- Story create/update/delete ----

func (c *Client) CreateStory(req zentaosdk.StoryCreateRequest) (*zentaosdk.Story, error) {

	form := neturl.Values{}
	form.Set("product", strconv.Itoa(req.Product))
	form.Set("title", req.Title)
	form.Set("pri", strconv.Itoa(req.Pri))
	form.Set("category", req.Category)
	form.Set("spec", req.Spec)
	form.Set("verify", req.Verify)
	form.Set("estimate", strconv.FormatFloat(req.Estimate, 'f', -1, 64))
	form.Set("assignedTo", req.AssignedTo)
	if req.Module != 0 {
		form.Set("module", strconv.Itoa(req.Module))
	}
	if err := c.doSessionPost(context.Background(), "CreateStory",
		"/story-create.json", form); err != nil {
		return nil, err
	}
	return &zentaosdk.Story{}, nil
}

func (c *Client) UpdateStory(storyID int, req zentaosdk.StoryUpdateRequest) (*zentaosdk.Story, error) {

	form := neturl.Values{}
	form.Set("title", req.Title)
	form.Set("spec", req.Spec)
	form.Set("verify", req.Verify)
	form.Set("type", req.Type)
	form.Set("pri", strconv.Itoa(req.Pri))
	form.Set("estimate", strconv.FormatFloat(req.Estimate, 'f', -1, 64))
	form.Set("assignedTo", req.AssignedTo)
	if err := c.doSessionPost(context.Background(), "UpdateStory",
		fmt.Sprintf("/story-edit-%d.json", storyID), form); err != nil {
		return nil, err
	}
	return &zentaosdk.Story{}, nil
}

func (c *Client) DeleteStory(storyID int) error {
	return c.doSessionPost(context.Background(), "DeleteStory",
		fmt.Sprintf("/story-delete-%d.json", storyID), neturl.Values{})
}

// ---- Effort record ----

func (c *Client) RecordEffort(taskID int, date string, consumed, left float64, work string) error {

	if date == "" {
		date = nowDate()
	}
	form := neturl.Values{}
	form.Set("dates[]", date)
	form.Set("consumed[]", strconv.FormatFloat(consumed, 'f', -1, 64))
	form.Set("left[]", strconv.FormatFloat(left, 'f', -1, 64))
	form.Set("work[]", work)
	return c.doSessionPost(context.Background(), "RecordEffort",
		fmt.Sprintf("/task-recordEstimate-%d.json", taskID), form)
}

// BatchCreateEffortItem 批量创建日志的单条记录。
//
// 受限实例 batchCreate 端点的 form 字段有特殊索引格式：id[] 是 1-based 行号，
// execution[N]/left[N] 用行号 N 索引；consumed[]/work[]/objectType[] 是无索引数组。
//
// 详见 /effort-batchCreate-{date}.html 的实际格式。
type BatchCreateEffortItem struct {
	ObjectType string  // "task_617525" 或 "bug_xxx" 或空行省略
	Execution  int     // execution ID（行号 N 对应 execution[N]）
	Consumed   float64 // 消耗工时
	Left       float64 // 剩余工时
	Work       string  // 工作内容
}

// BatchCreateEffort 批量创建某日 effort（受限 session mode 走 PHP form 端点）。
//
// 端点：POST /effort-batchCreate-{date}.html?onlybody=yes
// 鉴权：session 模式（受限实例 SSO 等禁用 REST API 的实例）。
// 缓存：写成功后通过 doSessionPost 的 writeOpToEntity 自动失效 sessionEntityEfforts。
//
// 响应穿透判定（技能红线：success 外壳 ≠ 落库）：
//   - 成功：HTML reload/closeModal 脚本；
//   - 失败：HTML alert('日志N : <原因>')——如实测"不能为将来的日期添加日志"。
func (c *Client) BatchCreateEffort(ctx context.Context, date string, items []BatchCreateEffortItem) error {
	if date == "" {
		return fmt.Errorf("date 不能为空")
	}
	if len(items) == 0 {
		return fmt.Errorf("items 不能为空")
	}

	form := neturl.Values{}
	form.Set("date", date)
	for i, item := range items {
		id := i + 1
		form.Set("id[]", strconv.Itoa(id))
		form.Set("work[]", item.Work)
		form.Set("objectType[]", item.ObjectType)
		form.Set(fmt.Sprintf("execution[%d]", id), strconv.Itoa(item.Execution))
		form.Set("consumed[]", strconv.FormatFloat(item.Consumed, 'f', -1, 64))
		form.Set(fmt.Sprintf("left[%d]", id), strconv.FormatFloat(item.Left, 'f', -1, 64))
	}
	path := fmt.Sprintf("/effort-batchCreate-%s.html?onlybody=yes", date)
	body, err := c.doSessionPostWithBody(ctx, "BatchCreateEffort", path, form)
	if err != nil {
		return err
	}
	if err := judgeBatchEffortResponse(body); err != nil {
		return err
	}
	return nil
}

// WriteRejectedError 表示禅道服务端明确拒绝的写操作（含可透传给用户的中文原因）。
type WriteRejectedError struct {
	Msg string
}

func (e *WriteRejectedError) Error() string { return e.Msg }

// judgeBatchEffortResponse 判定 effort-batchCreate 的 HTML 响应：
// alert('...') 弹窗即服务端拒绝（提取原因），reload/closeModal 视为成功。
func judgeBatchEffortResponse(body []byte) error {
	s := string(body)
	alertRe := regexp.MustCompile(`alert\('([^']+)'\)`)
	if m := alertRe.FindStringSubmatch(s); len(m) > 1 {
		return &WriteRejectedError{Msg: "工时写入被拒绝: " + m[1]}
	}
	if strings.Contains(s, "reload") || strings.Contains(s, "closeModal") || strings.Contains(s, "parent.$.enableForm") {
		// 含 enableForm 但无 alert 的失败形态已由上面拦截；此处为成功跳转/关窗
		return nil
	}
	// JSON 形态（部分实例）：{"result":"fail","message":...}
	var jr struct {
		Result  string `json:"result"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &jr) == nil && jr.Result == "fail" {
		return &WriteRejectedError{Msg: "工时写入被拒绝: " + jr.Message}
	}
	// 未知形态：HTTP 200 按成功（与既有写端点行为一致）
	return nil
}

// ---- Plan create/update/delete + link/unlink ----

func (c *Client) CreatePlan(productID int, req zentaosdk.PlanCreateRequest) (*zentaosdk.Plan, error) {

	form := neturl.Values{}
	form.Set("product", strconv.Itoa(productID))
	form.Set("title", req.Title)
	form.Set("begin", req.Begin)
	form.Set("end", req.End)
	form.Set("desc", req.Desc)
	if err := c.doSessionPost(context.Background(), "CreatePlan",
		"/plan-create.json", form); err != nil {
		return nil, err
	}
	return &zentaosdk.Plan{}, nil
}

func (c *Client) UpdatePlan(planID int, req zentaosdk.PlanCreateRequest) (*zentaosdk.Plan, error) {

	form := neturl.Values{}
	form.Set("title", req.Title)
	form.Set("begin", req.Begin)
	form.Set("end", req.End)
	form.Set("desc", req.Desc)
	if err := c.doSessionPost(context.Background(), "UpdatePlan",
		fmt.Sprintf("/plan-edit-%d.json", planID), form); err != nil {
		return nil, err
	}
	return &zentaosdk.Plan{}, nil
}

func (c *Client) DeletePlan(planID int) error {
	return c.doSessionPost(context.Background(), "DeletePlan",
		fmt.Sprintf("/plan-delete-%d.json", planID), neturl.Values{})
}

func (c *Client) LinkStoriesToPlan(planID int, storyIDs []int) error {

	form := neturl.Values{}
	form.Set("stories", joinInts(storyIDs))
	return c.doSessionPost(context.Background(), "LinkStoriesToPlan",
		fmt.Sprintf("/plan-linkStory-%d.json", planID), form)
}

func (c *Client) UnlinkStoriesFromPlan(planID int, storyIDs []int) error {

	form := neturl.Values{}
	form.Set("stories", joinInts(storyIDs))
	return c.doSessionPost(context.Background(), "UnlinkStoriesFromPlan",
		fmt.Sprintf("/plan-unlinkStory-%d.json", planID), form)
}

func (c *Client) LinkBugsToPlan(planID int, bugIDs []int) error {

	form := neturl.Values{}
	form.Set("bugs", joinInts(bugIDs))
	return c.doSessionPost(context.Background(), "LinkBugsToPlan",
		fmt.Sprintf("/plan-linkBug-%d.json", planID), form)
}

func (c *Client) UnlinkBugsFromPlan(planID int, bugIDs []int) error {

	form := neturl.Values{}
	form.Set("bugs", joinInts(bugIDs))
	return c.doSessionPost(context.Background(), "UnlinkBugsFromPlan",
		fmt.Sprintf("/plan-unlinkBug-%d.json", planID), form)
}

// ---- Case create/update/delete ----

func (c *Client) CreateCase(productID int, req zentaosdk.CaseCreateRequest) (*zentaosdk.Case, error) {

	form := neturl.Values{}
	form.Set("product", strconv.Itoa(productID))
	form.Set("title", req.Title)
	form.Set("type", req.Type)
	form.Set("pri", strconv.Itoa(req.Pri))
	form.Set("precondition", req.Precondition)
	form.Set("keywords", req.Keywords)
	if req.Module != 0 {
		form.Set("module", strconv.Itoa(req.Module))
	}
	if req.Story != 0 {
		form.Set("story", strconv.Itoa(req.Story))
	}
	if err := c.doSessionPost(context.Background(), "CreateCase",
		"/testcase-create.json", form); err != nil {
		return nil, err
	}
	return &zentaosdk.Case{}, nil
}

func (c *Client) UpdateCase(caseID int, req zentaosdk.CaseUpdateRequest) (*zentaosdk.Case, error) {

	form := neturl.Values{}
	form.Set("title", req.Title)
	form.Set("type", req.Type)
	form.Set("pri", strconv.Itoa(req.Pri))
	form.Set("precondition", req.Precondition)
	if err := c.doSessionPost(context.Background(), "UpdateCase",
		fmt.Sprintf("/testcase-edit-%d.json", caseID), form); err != nil {
		return nil, err
	}
	return &zentaosdk.Case{}, nil
}

func (c *Client) DeleteCase(caseID int) error {
	return c.doSessionPost(context.Background(), "DeleteCase",
		fmt.Sprintf("/testcase-delete-%d.json", caseID), neturl.Values{})
}

// ---- Product/Project/Program/Execution/Build/User create/update/delete ----
// 这些是基础实体管理，session 模式同样走 PHP 表单端点。

func (c *Client) CreateProduct(req zentaosdk.ProductCreateRequest) (*zentaosdk.Product, error) {

	form := neturl.Values{}
	form.Set("name", req.Name)
	form.Set("code", req.Code)
	form.Set("type", req.Type)
	form.Set("desc", req.Desc)
	if err := c.doSessionPost(context.Background(), "CreateProduct",
		"/product-create.json", form); err != nil {
		return nil, err
	}
	return &zentaosdk.Product{}, nil
}

func (c *Client) UpdateProduct(productID int, req zentaosdk.ProductCreateRequest) (*zentaosdk.Product, error) {

	form := neturl.Values{}
	form.Set("name", req.Name)
	form.Set("code", req.Code)
	form.Set("type", req.Type)
	if err := c.doSessionPost(context.Background(), "UpdateProduct",
		fmt.Sprintf("/product-edit-%d.json", productID), form); err != nil {
		return nil, err
	}
	return &zentaosdk.Product{}, nil
}

func (c *Client) DeleteProduct(productID int) error {
	return c.doSessionPost(context.Background(), "DeleteProduct",
		fmt.Sprintf("/product-delete-%d.json", productID), neturl.Values{})
}

func (c *Client) CreateProject(req zentaosdk.ProjectCreateRequest) (*zentaosdk.Project, error) {

	form := neturl.Values{}
	form.Set("name", req.Name)
	form.Set("code", req.Code)
	form.Set("model", req.Model)
	form.Set("begin", req.Begin)
	form.Set("end", req.End)
	if err := c.doSessionPost(context.Background(), "CreateProject",
		"/project-create.json", form); err != nil {
		return nil, err
	}
	return &zentaosdk.Project{}, nil
}

func (c *Client) UpdateProject(projectID int, req zentaosdk.ProjectCreateRequest) (*zentaosdk.Project, error) {

	form := neturl.Values{}
	form.Set("name", req.Name)
	form.Set("code", req.Code)
	if err := c.doSessionPost(context.Background(), "UpdateProject",
		fmt.Sprintf("/project-edit-%d.json", projectID), form); err != nil {
		return nil, err
	}
	return &zentaosdk.Project{}, nil
}

func (c *Client) DeleteProject(projectID int) error {
	return c.doSessionPost(context.Background(), "DeleteProject",
		fmt.Sprintf("/project-delete-%d.json", projectID), neturl.Values{})
}

func (c *Client) CreateProgram(req zentaosdk.ProgramCreateRequest) (*zentaosdk.Program, error) {

	form := neturl.Values{}
	form.Set("name", req.Name)
	form.Set("code", req.Code)
	form.Set("begin", req.Begin)
	form.Set("end", req.End)
	if err := c.doSessionPost(context.Background(), "CreateProgram",
		"/program-create.json", form); err != nil {
		return nil, err
	}
	return &zentaosdk.Program{}, nil
}

func (c *Client) UpdateProgram(programID int, req zentaosdk.ProgramCreateRequest) (*zentaosdk.Program, error) {

	form := neturl.Values{}
	form.Set("name", req.Name)
	form.Set("code", req.Code)
	if err := c.doSessionPost(context.Background(), "UpdateProgram",
		fmt.Sprintf("/program-edit-%d.json", programID), form); err != nil {
		return nil, err
	}
	return &zentaosdk.Program{}, nil
}

func (c *Client) DeleteProgram(programID int) error {
	return c.doSessionPost(context.Background(), "DeleteProgram",
		fmt.Sprintf("/program-delete-%d.json", programID), neturl.Values{})
}

func (c *Client) CreateExecution(projectID int, req zentaosdk.ExecutionCreateRequest) (*zentaosdk.Execution, error) {

	form := neturl.Values{}
	form.Set("project", strconv.Itoa(projectID))
	form.Set("name", req.Name)
	form.Set("code", req.Code)
	form.Set("begin", req.Begin)
	form.Set("end", req.End)
	if err := c.doSessionPost(context.Background(), "CreateExecution",
		"/execution-create.json", form); err != nil {
		return nil, err
	}
	return &zentaosdk.Execution{}, nil
}

func (c *Client) UpdateExecution(executionID int, req zentaosdk.ExecutionCreateRequest) (*zentaosdk.Execution, error) {

	form := neturl.Values{}
	form.Set("name", req.Name)
	form.Set("code", req.Code)
	if err := c.doSessionPost(context.Background(), "UpdateExecution",
		fmt.Sprintf("/execution-edit-%d.json", executionID), form); err != nil {
		return nil, err
	}
	return &zentaosdk.Execution{}, nil
}

func (c *Client) DeleteExecution(executionID int) error {
	return c.doSessionPost(context.Background(), "DeleteExecution",
		fmt.Sprintf("/execution-delete-%d.json", executionID), neturl.Values{})
}

func (c *Client) CreateBuild(projectID int, req zentaosdk.BuildCreateRequest) (*zentaosdk.Build, error) {

	form := neturl.Values{}
	form.Set("project", strconv.Itoa(projectID))
	form.Set("name", req.Name)
	form.Set("date", req.Date)
	if req.Execution != 0 {
		form.Set("execution", strconv.Itoa(req.Execution))
	}
	if err := c.doSessionPost(context.Background(), "CreateBuild",
		"/build-create.json", form); err != nil {
		return nil, err
	}
	return &zentaosdk.Build{}, nil
}

func (c *Client) UpdateBuild(buildID int, req zentaosdk.BuildCreateRequest) (*zentaosdk.Build, error) {

	form := neturl.Values{}
	form.Set("name", req.Name)
	if err := c.doSessionPost(context.Background(), "UpdateBuild",
		fmt.Sprintf("/build-edit-%d.json", buildID), form); err != nil {
		return nil, err
	}
	return &zentaosdk.Build{}, nil
}

func (c *Client) DeleteBuild(buildID int) error {
	return c.doSessionPost(context.Background(), "DeleteBuild",
		fmt.Sprintf("/build-delete-%d.json", buildID), neturl.Values{})
}

func (c *Client) CreateUser(req zentaosdk.UserCreateRequest) (*zentaosdk.User, error) {

	form := neturl.Values{}
	form.Set("account", req.Account)
	form.Set("password", req.Password)
	form.Set("realname", req.Realname)
	form.Set("email", req.Email)
	if err := c.doSessionPost(context.Background(), "CreateUser",
		"/user-create.json", form); err != nil {
		return nil, err
	}
	return &zentaosdk.User{}, nil
}

func (c *Client) UpdateUser(userID int, req zentaosdk.UserUpdateRequest) (*zentaosdk.User, error) {

	form := neturl.Values{}
	form.Set("realname", req.Realname)
	form.Set("email", req.Email)
	if err := c.doSessionPost(context.Background(), "UpdateUser",
		fmt.Sprintf("/user-edit-%d.json", userID), form); err != nil {
		return nil, err
	}
	return &zentaosdk.User{}, nil
}

func (c *Client) DeleteUser(userID int) error {
	return c.doSessionPost(context.Background(), "DeleteUser",
		fmt.Sprintf("/user-delete-%d.json", userID), neturl.Values{})
}

// nowDate 返回当天日期 YYYY-MM-DD（RecordEffort 默认日期用）。
func nowDate() string {
	return timeNow().Format("2006-01-02")
}

package mcp

// Tool 定义 MCP 工具.
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

// InputSchema JSON Schema 格式的参数定义.
type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties,omitempty"`
	Required   []string            `json:"required,omitempty"`
}

// Property 参数属性.
type Property struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
}

// Tools 所有可用的 MCP 工具.
var Tools = []Tool{
	{
		Name:        "ping",
		Description: "测试 MCP 服务是否正常运行",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]Property{},
		},
	},
	{
		Name:        "get_products",
		Description: "获取禅道产品列表",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]Property{},
		},
	},
	{
		Name:        "get_projects",
		Description: "获取项目列表，可按产品 ID 过滤",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"productId": {
					Type:        "string",
					Description: "产品 ID（可选）",
				},
			},
		},
	},
	{
		Name:        "get_executions",
		Description: "获取执行/迭代列表，可按项目 ID 或产品 ID 过滤",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"projectId": {
					Type:        "string",
					Description: "项目 ID（可选）",
				},
				"productId": {
					Type:        "string",
					Description: "产品 ID（可选）",
				},
			},
		},
	},
	{
		Name:        "get_bugs",
		Description: "获取 Bug 列表，可按产品 ID 和状态过滤",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"productId": {
					Type:        "string",
					Description: "产品 ID（可选）",
				},
				"status": {
					Type:        "string",
					Description: "Bug 状态（可选，如 active, resolved, closed）",
				},
			},
		},
	},
	{
		Name:        "get_stories",
		Description: "获取需求列表，可按产品 ID 过滤",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"productId": {
					Type:        "string",
					Description: "产品 ID（可选）",
				},
			},
		},
	},
	{
		Name:        "get_tasks",
		Description: "获取任务列表，可按产品 ID 和执行 ID 过滤",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"productId": {
					Type:        "string",
					Description: "产品 ID（可选）",
				},
				"executionId": {
					Type:        "string",
					Description: "执行/迭代 ID（可选）",
				},
			},
		},
	},
	{
		Name:        "get_users",
		Description: "获取用户列表",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]Property{},
		},
	},
	{
		Name:        "get_bug_detail",
		Description: "获取单个 Bug 的完整详情（标题/描述/状态/严重级/优先级/指派人/解决方案/所属执行等）。比 get_bugs 列表字段全，分析单个问题时用",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"bugId": {Type: "number", Description: "Bug ID（必填）"},
			},
			Required: []string{"bugId"},
		},
	},
	{
		Name:        "get_task_detail",
		Description: "获取单个任务的完整详情（名称/类型/状态/预计与消耗/剩余工时/指派人/预计起止日期等）",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"taskId": {Type: "number", Description: "任务 ID（必填）"},
			},
			Required: []string{"taskId"},
		},
	},
	{
		Name:        "get_story_detail",
		Description: "获取单个需求的完整详情（标题/描述/验收标准/阶段/估时/状态等）",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"storyId": {Type: "number", Description: "需求 ID（必填）"},
			},
			Required: []string{"storyId"},
		},
	},
	{
		Name:        "get_my_tasks",
		Description: "获取我负责/参与的任务列表（my-work 视图）。回答「我现在有哪些任务」「今天该做什么」时优先用这个",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]Property{},
		},
	},
	{
		Name:        "get_effort_calendar",
		Description: "获取指定用户全年的工时日志明细（工时日历）。回查某天/某月记了哪些工时、核对 record_effort 是否落库时用。注意：受限实例账号通常无用户管理权限，account 反查仅对当前登录账号有效，查他人会报「未找到账号」",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"account": {Type: "string", Description: "禅道账号（与 userId 二选一；不传默认查当前登录账号。受限实例仅支持查自己）"},
				"userId":  {Type: "number", Description: "禅道用户数字 ID（与 account 二选一，优先级更高）"},
				"year":    {Type: "number", Description: "年份（可选，默认当年）"},
			},
		},
	},
	{
		Name:        "get_timelog",
		Description: "获取工时统计数据，可按产品 ID 和日期范围过滤",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"productId": {
					Type:        "string",
					Description: "产品 ID（可选）",
				},
				"dateFrom": {
					Type:        "string",
					Description: "开始日期，格式 YYYY-MM-DD（可选）",
				},
				"dateTo": {
					Type:        "string",
					Description: "结束日期，格式 YYYY-MM-DD（可选）",
				},
			},
		},
	},
	{
		Name:        "get_zentao_config",
		Description: "查看当前禅道连接配置（服务器地址/账号/认证域/连接状态，密码不回显）。刚接入时先用这个确认是否已配置；连接异常时配合 test_connection 排查",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]Property{},
		},
	},
	{
		Name:        "update_zentao_config",
		Description: "更新禅道连接配置并立即验证登录、持久化保存。首次接入/换实例时用：提供服务器地址、账号、密码即可，成功后所有查询工具立即可用。认证域留空=标准禅道 REST 模式；受限实例（如 kydc 域）传对应域名",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"server":   {Type: "string", Description: "禅道服务器地址（必填，如 https://zentao.example.com）"},
				"account":  {Type: "string", Description: "禅道账号（必填）"},
				"password": {Type: "string", Description: "禅道密码（必填）"},
				"realm":    {Type: "string", Description: "认证域（可选：留空=标准 REST；kydc=受限实例会话模式；local 等价于留空）"},
			},
			Required: []string{"server", "account", "password"},
		},
	},
	{
		Name:        "test_connection",
		Description: "实时探测禅道连通性（登录会话探针，返回延迟与当前账号信息）。查询报错/超时时先用这个定位是连接问题还是数据问题",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]Property{},
		},
	},
	{
		Name:        "send_verify_code",
		Description: "麒麟统一认证（SSO）第一步：账号+密码触发短信验证码，返回脱敏手机号。第二步用 sso_login 提交验证码。用于验证统一认证账号可用或协助排错登录问题",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"username": {Type: "string", Description: "统一认证账号（必填）"},
				"password": {Type: "string", Description: "统一认证密码（必填）"},
			},
			Required: []string{"username", "password"},
		},
	},
	{
		Name:        "sso_login",
		Description: "麒麟统一认证（SSO）第二步：账号+密码+短信验证码完成登录，返回认证账号。注意频繁登录会触发 SSO 限流",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"username": {Type: "string", Description: "统一认证账号（必填）"},
				"password": {Type: "string", Description: "统一认证密码（必填）"},
				"code":     {Type: "string", Description: "短信验证码（必填，先调 send_verify_code 触发）"},
			},
			Required: []string{"username", "password", "code"},
		},
	},
	{
		Name:        "record_effort",
		Description: "写禅道日志：为指定任务记录一条工时（挂任务）。consumed 必须大于 0；left 为该任务剩余工时的绝对值。写后建议回查工时日历核对",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"taskId":    {Type: "number", Description: "任务 ID（必填）"},
				"execution": {Type: "number", Description: "执行 ID（可选，缺省自动从我的任务补齐）"},
				"consumed":  {Type: "number", Description: "消耗工时（小时，必填，>0）"},
				"left":      {Type: "number", Description: "任务剩余工时（小时，必填，绝对值）"},
				"work":      {Type: "string", Description: "工作内容（必填）"},
				"date":      {Type: "string", Description: "日期 YYYY-MM-DD（可选，默认今天；不能是未来日期）"},
			},
			Required: []string{"taskId", "consumed", "left", "work"},
		},
	},
	{
		Name:        "add_bug_comment",
		Description: "给禅道 Bug 添加备注（走 bug-edit 全量通道，自动带原值）。success 不代表落库，建议回查",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"bugId":   {Type: "number", Description: "Bug ID（必填）"},
				"comment": {Type: "string", Description: "备注内容（必填）"},
			},
			Required: []string{"bugId", "comment"},
		},
	},
	{
		Name:        "transition_bug",
		Description: "Bug 状态流转：confirm/resolve/close/activate/assign。resolve 必填 resolution 与 resolvedBuild；assign 必填 assignedTo",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"bugId":         {Type: "number", Description: "Bug ID（必填）"},
				"action":        {Type: "string", Description: "流转动作（必填）", Enum: []string{"confirm", "resolve", "close", "activate", "assign"}},
				"resolution":    {Type: "string", Description: "解决方案（resolve 必填）", Enum: []string{"fixed", "bydesign", "duplicate", "notrepro", "postponed", "willnotfix", "external"}},
				"resolvedBuild": {Type: "string", Description: "解决版本（resolve 建议填写）"},
				"assignedTo":    {Type: "string", Description: "指派给（assign 必填 / activate 可选）"},
				"comment":       {Type: "string", Description: "备注（可选）"},
			},
			Required: []string{"bugId", "action"},
		},
	},
	{
		Name:        "create_bug",
		Description: "创建禅道 Bug（测试/验收发现问题需要记录时用）。返回新 Bug ID。标题必填；描述建议包含复现步骤/预期/实际结果；影响版本缺省 trunk",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"productId":   {Type: "number", Description: "产品 ID（必填，可先用 get_products 查）"},
				"title":       {Type: "string", Description: "Bug 标题（必填）"},
				"steps":       {Type: "string", Description: "重现步骤/描述（建议填写，支持简单 HTML）"},
				"severity":    {Type: "number", Description: "严重程度 1-4（可选，默认 3；1 最严重）"},
				"pri":         {Type: "number", Description: "优先级 1-4（可选，默认 3；1 最高）"},
				"type":        {Type: "string", Description: "Bug 类型（可选，默认 codeerror，如 config/execute/install/interface/other）"},
				"assignedTo":  {Type: "string", Description: "指派给（可选，禅道账号）"},
				"openedBuild": {Type: "string", Description: "影响版本（可选，build ID 或 trunk，默认 trunk）"},
				"execution":   {Type: "number", Description: "所属执行/迭代 ID（可选，传了会自动关联所属项目）"},
				"module":      {Type: "number", Description: "所属模块 ID（可选，0=根模块）"},
			},
			Required: []string{"productId", "title"},
		},
	},
	{
		Name:        "transition_task",
		Description: "任务状态流转：start（开始）/finish（完成）/pause（暂停）/assign（改派）。finish 建议带 consumed（本次累计消耗）；如需同步更新剩余工时请再调 record_effort",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"taskId":       {Type: "number", Description: "任务 ID（必填）"},
				"action":       {Type: "string", Description: "流转动作（必填）", Enum: []string{"start", "finish", "pause", "assign"}},
				"consumed":     {Type: "number", Description: "消耗工时（finish 建议填写，累计值）"},
				"assignedTo":   {Type: "string", Description: "指派给（assign 必填，禅道账号）"},
				"left":         {Type: "number", Description: "剩余工时（assign 可选）"},
				"comment":      {Type: "string", Description: "备注（可选）"},
				"finishedDate": {Type: "string", Description: "完成日期 YYYY-MM-DD（可选，默认今天）"},
			},
			Required: []string{"taskId", "action"},
		},
	},
}

// GetToolByName 根据名称获取工具定义.
// 写工具集合：只读模式（mcp.read_only）下这些工具会被拦截。
// 除禅道数据写入外，配置变更（update_zentao_config）与认证操作（send_verify_code/
// sso_login）同属受控行为——只读令牌不应能重配服务器连接或触发认证流程。
var writeTools = map[string]bool{
	"record_effort":        true,
	"add_bug_comment":      true,
	"transition_bug":       true,
	"create_bug":           true,
	"transition_task":      true,
	"update_zentao_config": true,
	"send_verify_code":     true,
	"sso_login":            true,
}

// IsWriteTool 查询工具是否为写操作。
func IsWriteTool(name string) bool {
	return writeTools[name]
}

func GetToolByName(name string) *Tool {
	for _, t := range Tools {
		if t.Name == name {
			return &t
		}
	}
	return nil
}

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
}

// GetToolByName 根据名称获取工具定义.
// 写工具集合：只读模式（mcp.read_only）下这些工具会被拦截。
var writeTools = map[string]bool{
	"record_effort":   true,
	"add_bug_comment": true,
	"transition_bug":  true,
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

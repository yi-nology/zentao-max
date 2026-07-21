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
}

// GetToolByName 根据名称获取工具定义.
func GetToolByName(name string) *Tool {
	for _, t := range Tools {
		if t.Name == name {
			return &t
		}
	}
	return nil
}

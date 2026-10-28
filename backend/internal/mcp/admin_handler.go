package mcp

import (
	"context"

	"github.com/yi-nology/zentao-max/backend/pkg/errors"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
)

// MCPAdminHandler MCP 运行时管理 API 处理器
//
// 提供热重载能力，无需重启进程即可调整 MCP 子系统的运行状态：
//
//	GET  /api/v1/mcp/status  轻量探测，返回当前模式状态（供前端状态卡）
//	GET  /api/v1/mcp/config  返回当前 MCP 配置快照（Token 不返回明文）
//	PUT  /api/v1/mcp/config  部分更新（enabled / readOnly / token / actions）
//
// 安全提示：本接口当前未加额外鉴权（与项目其他 /api/* 路由一致）。
// 生产环境若暴露在公网，建议配合反向代理鉴权或网络隔离。
type MCPAdminHandler struct {
	mgr *MCPModeManager
}

// NewMCPAdminHandler 创建管理处理器.
func NewMCPAdminHandler() *MCPAdminHandler {
	return &MCPAdminHandler{mgr: GetMCPModeManager()}
}

// HandleGetStatus GET /api/v1/mcp/status — 轻量状态探测.
func (h *MCPAdminHandler) HandleGetStatus(ctx context.Context, c *app.RequestContext) {
	errors.Success(c, h.mgr.GetStatus())
}

// HandleGetConfig GET /api/v1/mcp/config — 返回当前配置快照.
func (h *MCPAdminHandler) HandleGetConfig(ctx context.Context, c *app.RequestContext) {
	errors.Success(c, h.mgr.GetStatus())
}

// HandleUpdateConfig PUT /api/v1/mcp/config — 部分热重载
//
// 请求体（所有字段可选，省略表示不修改）：
//
//	{
//	  "enabled": true,
//	  "readOnly": false,
//	  "token": "new-token",      // 设为空串表示关闭鉴权
//	  "actions": ["get_bugs"]    // 设为 [] 表示允许全部
//	}
func (h *MCPAdminHandler) HandleUpdateConfig(ctx context.Context, c *app.RequestContext) {
	var req MCPUpdateRequest
	if err := c.BindAndValidate(&req); err != nil {
		errors.BadRequest(c, "Invalid request body: "+err.Error())
		return
	}
	h.mgr.UpdateStatus(req)
	errors.SuccessWithMessage(c, "MCP config updated", h.mgr.GetStatus())
}

// RegisterAdminRoutes 在指定路由组上注册管理端点
//
// 挂载到 /api/v1 前缀下，例如：
//
//	g := r.Group("/api/v1")
//	adminHandler.RegisterAdminRoutes(g)
func (h *MCPAdminHandler) RegisterAdminRoutes(g routeGroup) {
	g.GET("/mcp/status", h.HandleGetStatus)
	g.GET("/mcp/config", h.HandleGetConfig)
	g.PUT("/mcp/config", h.HandleUpdateConfig)
}

// routeGroup 路由组抽象，兼容 *route.RouterGroup 与 *server.Hertz 的 Group
// 避免在 mcp 包直接依赖具体类型.
type routeGroup interface {
	GET(path string, handlers ...app.HandlerFunc) route.IRoutes
	POST(path string, handlers ...app.HandlerFunc) route.IRoutes
	PUT(path string, handlers ...app.HandlerFunc) route.IRoutes
	DELETE(path string, handlers ...app.HandlerFunc) route.IRoutes
}

package handlers

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/yi-nology/zentao-max/backend/internal/sysinstall"
	"github.com/yi-nology/zentao-max/backend/internal/update"
	"github.com/yi-nology/zentao-max/backend/pkg/errors"
)

// UpdateHandler 软件版本检查/更新端点。
// GET  /api/update/status   匿名可读（GET 由平台认证中间件放行）
// POST /api/update/check    管理员（手动触发检查）
// POST /api/update/download 管理员（下载更新包）
type UpdateHandler struct {
	svc *update.Service
}

func NewUpdateHandler(svc *update.Service) *UpdateHandler {
	return &UpdateHandler{svc: svc}
}

// GetStatus 返回版本检查状态快照。
func (h *UpdateHandler) GetStatus(ctx context.Context, c *app.RequestContext) {
	errors.Success(c, h.svc.Status())
}

// CheckNow 手动触发一次检查。
func (h *UpdateHandler) CheckNow(ctx context.Context, c *app.RequestContext) {
	info := h.svc.CheckNow()
	errors.Success(c, info)
}

// Download 下载更新包（异步，进度经 Status 轮询）。
func (h *UpdateHandler) Download(ctx context.Context, c *app.RequestContext) {
	if err := h.svc.Download(); err != nil {
		errors.BadRequest(c, err.Error())
		return
	}
	errors.Success(c, map[string]interface{}{"message": "下载已开始，可在状态中查看进度"})
}

// Apply 从已下载的更新包提取新二进制并替换当前可执行文件。
func (h *UpdateHandler) Apply(ctx context.Context, c *app.RequestContext) {
	applied, newPath, err := h.svc.Apply()
	if err != nil {
		errors.BadRequest(c, err.Error())
		return
	}
	errors.Success(c, map[string]interface{}{
		"applied":         applied,
		"newBinaryPath":   newPath,
		"restartRequired": true,
		"message":         "更新已应用，重启应用后生效",
	})
}

// Restart 重启应用：已安装系统服务时经服务管理器重启（拉起新二进制更干净），
// 否则 unix 走进程内 exec、Windows 返回手动指引。
func (h *UpdateHandler) Restart(ctx context.Context, c *app.RequestContext) {
	if st, _, derr := sysinstall.Detect(); derr == nil && st.Installed {
		if serr := sysinstall.RestartService(); serr != nil {
			errors.BadRequest(c, "经服务管理器重启失败: "+serr.Error())
			return
		}
		errors.Success(c, map[string]interface{}{"message": "已通过系统服务重启，页面将在数秒后自动恢复"})
		return
	}
	if err := h.svc.RestartSelf(); err != nil {
		errors.BadRequest(c, err.Error())
		return
	}
	errors.Success(c, map[string]interface{}{"message": "重启中，页面将在数秒后自动恢复"})
}

// StartAutoCheck 拉起启动注册/定期检查（HTTPApp.Start 调用）。
func (h *UpdateHandler) StartAutoCheck(ctx context.Context) {
	h.svc.StartAutoCheck(ctx)
}

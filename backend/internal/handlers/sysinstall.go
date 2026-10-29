package handlers

import (
	"context"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/yi-nology/zentao-max/backend/internal/sysinstall"
	"github.com/yi-nology/zentao-max/backend/pkg/errors"
)

// SystemInstallHandler 系统服务安装（开机自启）端点。
// GET  /api/system/install-status 匿名可读
// POST /api/system/{install,uninstall,service-start,service-stop,service-restart} 管理员
type SystemInstallHandler struct{}

func NewSystemInstallHandler() *SystemInstallHandler { return &SystemInstallHandler{} }

// GetStatus 安装/运行状态。
func (h *SystemInstallHandler) GetStatus(ctx context.Context, c *app.RequestContext) {
	st, def, err := sysinstall.Detect()
	if err != nil {
		errors.Error(c, errors.NewInternalError("获取服务状态失败", err))
		return
	}
	if st.DevMode {
		st.Detail = "开发模式（go run）不可安装服务"
	} else if !st.Supported {
		st.Detail = "当前平台不支持"
	}
	errors.Success(c, map[string]interface{}{
		"platform":    st.Platform,
		"installed":   st.Installed,
		"running":     st.Running,
		"execPath":    st.ExecPath,
		"workDir":     st.WorkDir,
		"devMode":     st.DevMode,
		"unitPath":    st.UnitPath,
		"detail":      st.Detail,
		"serviceArgs": strings.Join(def.Args, " "),
	})
}

// Install 一键安装为系统服务（开机自启 + 立即启动）。
func (h *SystemInstallHandler) Install(ctx context.Context, c *app.RequestContext) {
	msg, err := sysinstall.Install()
	if err != nil {
		errors.BadRequest(c, err.Error())
		return
	}
	errors.Success(c, map[string]interface{}{
		"message": msg + "。注意：如当前有手动运行的实例，请停止它（端口冲突会导致服务实例退出）；服务会在下次开机/登录时自启",
	})
}

// Uninstall 卸载系统服务。
func (h *SystemInstallHandler) Uninstall(ctx context.Context, c *app.RequestContext) {
	msg, err := sysinstall.Uninstall()
	if err != nil {
		errors.BadRequest(c, err.Error())
		return
	}
	errors.Success(c, map[string]interface{}{"message": msg})
}

// ServiceAction start/stop/restart 服务实例。
func (h *SystemInstallHandler) ServiceAction(ctx context.Context, c *app.RequestContext) {
	action := c.Param("action")
	if action != "start" && action != "stop" && action != "restart" {
		errors.BadRequest(c, "不支持的动作: "+action)
		return
	}
	msg, err := sysinstall.ServiceControl(action)
	if err != nil {
		errors.BadRequest(c, err.Error())
		return
	}
	errors.Success(c, map[string]interface{}{"message": msg})
}

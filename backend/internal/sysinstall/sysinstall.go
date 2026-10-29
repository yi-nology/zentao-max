// Package sysinstall 把 zentao-max 注册为系统服务（开机自启）。
//
// 平台策略（全部用户级、免 sudo/管理员）：
//   - macOS：launchd LaunchAgent（~/Library/LaunchAgents/com.murphyyi.zentao-max.plist）
//   - Linux：systemd user 服务（~/.config/systemd/user/zentao-max.service，
//     环境变量经 EnvironmentFile 注入；安装时尝试 enable-linger 支持未登录常驻）
//   - Windows：计划任务登录自启（schtasks /sc onlogon），经生成的 run.bat 包装
//     以保证工作目录正确（schtasks 不支持 WorkingDirectory）
//
// 三个关键不变量：
//   1. 工作目录继承安装时的 cwd——auth.db/cron.db/cache 全是 cwd 相对路径；
//   2. 环境快照固化进服务定义（ZENTAO_ENCRYPTION_KEY 等丢失会导致开机解密失败）；
//   3. 开发模式守卫：go run 的临时二进制（路径含 go-build）拒绝安装。
package sysinstall

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const ServiceLabel = "com.murphyyi.zentao-max"

// Status 安装/运行状态快照。
type Status struct {
	Platform    string `json:"platform"`
	Supported   bool   `json:"supported"`
	Installed   bool   `json:"installed"`
	Running     bool   `json:"running"`
	ExecPath    string `json:"execPath"`
	WorkDir     string `json:"workDir"`
	DevMode     bool   `json:"devMode"` // go run 临时二进制，不可安装
	UnitPath    string `json:"unitPath"`
	Detail      string `json:"detail"`
}

type definition struct {
	ExecPath string
	Args     []string
	WorkDir  string
	Env      [][2]string
}

// Detect 收集安装所需信息与当前状态。
func Detect() (Status, definition, error) {
	st := Status{Platform: runtime.GOOS, Supported: runtime.GOOS != "js"}
	execPath, err := os.Executable()
	if err != nil {
		return st, definition{}, err
	}
	workDir, _ := os.Getwd()
	st.ExecPath = execPath
	st.WorkDir = workDir
	if strings.Contains(execPath, "go-build") {
		st.DevMode = true
	}
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		st.UnitPath = filepath.Join(home, "Library", "LaunchAgents", ServiceLabel+".plist")
	case "linux":
		home, _ := os.UserHomeDir()
		st.UnitPath = filepath.Join(home, ".config", "systemd", "user", ServiceLabel+".service")
	case "windows":
		home, _ := os.UserHomeDir()
		st.UnitPath = filepath.Join(home, "zentao-max-service.bat")
	default:
		st.Supported = false
		return st, definition{}, nil
	}
	if _, err := os.Stat(st.UnitPath); err == nil {
		st.Installed = true
	}
	st.Running = serviceRunning()
	return st, buildDefinition(execPath, os.Args, workDir), nil
}

func buildDefinition(execPath string, args []string, workDir string) definition {
	def := definition{ExecPath: execPath, Args: args, WorkDir: workDir}
	for _, kv := range os.Environ() {
		if i := strings.Index(kv, "="); i > 0 {
			def.Env = append(def.Env, [2]string{kv[:i], kv[i+1:]})
		}
	}
	return def
}

// Install 生成服务定义并注册（enable + start）。
func Install() (string, error) {
	st, def, err := Detect()
	if err != nil {
		return "", err
	}
	if st.DevMode {
		return "", fmt.Errorf("开发模式（go run 的临时二进制）不支持安装服务：请使用构建产物启动后再安装")
	}
	if !st.Supported {
		return "", fmt.Errorf("不支持的平台: %s", st.Platform)
	}
	switch runtime.GOOS {
	case "darwin":
		return installLaunchAgent(st, def)
	case "linux":
		return installSystemdUser(st, def)
	case "windows":
		return installWindowsTask(st, def)
	}
	return "", fmt.Errorf("不支持的平台: %s", st.Platform)
}

// Uninstall 停止并移除服务定义。
func Uninstall() (string, error) {
	st, _, err := Detect()
	if err != nil {
		return "", err
	}
	if !st.Installed {
		return "", fmt.Errorf("尚未安装系统服务")
	}
	switch runtime.GOOS {
	case "darwin":
		_ = run("launchctl", "unload", st.UnitPath)
		if err := os.Remove(st.UnitPath); err != nil {
			return "", err
		}
		return "已卸载 launchd 服务", nil
	case "linux":
		_ = run("systemctl", "--user", "disable", "--now", ServiceLabel+".service")
		if err := os.Remove(st.UnitPath); err != nil {
			return "", err
		}
		_ = run("systemctl", "--user", "daemon-reload")
		return "已卸载 systemd 用户服务", nil
	case "windows":
		_ = run("schtasks", "/end", "/tn", ServiceLabel)
		if err := run("schtasks", "/delete", "/tn", ServiceLabel, "/f"); err != nil {
			return "", err
		}
		_ = os.Remove(st.UnitPath)
		return "已卸载计划任务", nil
	}
	return "", fmt.Errorf("不支持的平台")
}

// ServiceControl start/stop/restart 动作。
func ServiceControl(action string) (string, error) {
	st, _, err := Detect()
	if err != nil {
		return "", err
	}
	if !st.Installed {
		return "", fmt.Errorf("尚未安装系统服务")
	}
	switch runtime.GOOS {
	case "darwin":
		switch action {
		case "start":
			return "已启动", run("launchctl", "start", ServiceLabel)
		case "stop":
			return "已停止", run("launchctl", "stop", ServiceLabel)
		case "restart":
			_ = run("launchctl", "stop", ServiceLabel)
			time.Sleep(500 * time.Millisecond)
			return "已重启", run("launchctl", "start", ServiceLabel)
		}
	case "linux":
		unit := ServiceLabel + ".service"
		switch action {
		case "start":
			return "已启动", run("systemctl", "--user", "start", unit)
		case "stop":
			return "已停止", run("systemctl", "--user", "stop", unit)
		case "restart":
			return "已重启", run("systemctl", "--user", "restart", unit)
		}
	case "windows":
		switch action {
		case "start":
			return "已启动", run("schtasks", "/run", "/tn", ServiceLabel)
		case "stop":
			return "已停止", run("schtasks", "/end", "/tn", ServiceLabel)
		case "restart":
			_ = run("schtasks", "/end", "/tn", ServiceLabel)
			time.Sleep(500 * time.Millisecond)
			return "已重启", run("schtasks", "/run", "/tn", ServiceLabel)
		}
	}
	return "", fmt.Errorf("不支持的动作: %s", action)
}

// RestartService 经服务管理器重启（供更新流程使用：应用更新后由管理器
// 拉起新二进制，比进程内 exec 更干净）。未安装服务时返回错误。
func RestartService() error {
	st, _, err := Detect()
	if err != nil {
		return err
	}
	if !st.Installed {
		return fmt.Errorf("尚未安装系统服务")
	}
	_, err = ServiceControl("restart")
	return err
}

func serviceRunning() bool {
	switch runtime.GOOS {
	case "darwin":
		return run("launchctl", "list", ServiceLabel) == nil
	case "linux":
		return run("systemctl", "--user", "is-active", "--quiet", ServiceLabel+".service") == nil
	case "windows":
		return run("schtasks", "/query", "/tn", ServiceLabel) == nil
	}
	return false
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %v: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

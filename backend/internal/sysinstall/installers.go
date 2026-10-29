package sysinstall

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// installLaunchAgent macOS：生成 LaunchAgent plist 并 load。
func installLaunchAgent(st Status, def definition) (string, error) {
	var env strings.Builder
	for _, kv := range def.Env {
		env.WriteString(fmt.Sprintf("\t\t<key>%s</key>\n\t\t<string>%s</string>\n", xmlEscape(kv[0]), xmlEscape(kv[1])))
	}
	var args strings.Builder
	args.WriteString(fmt.Sprintf("\n\t\t<string>%s</string>", xmlEscape(def.ExecPath)))
	for _, a := range def.Args {
		args.WriteString(fmt.Sprintf("\n\t\t<string>%s</string>", xmlEscape(a)))
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
		<key>Label</key>
		<string>%s</string>
		<key>ProgramArguments</key>
		<array>%s
		</array>
		<key>WorkingDirectory</key>
		<string>%s</string>
		<key>RunAtLoad</key>
		<true/>
		<key>EnvironmentVariables</key>
		<dict>
%s		</dict>
		<key>StandardOutPath</key>
		<string>%s/data/service.out.log</string>
		<key>StandardErrorPath</key>
		<string>%s/data/service.err.log</string>
</dict>
</plist>
`, ServiceLabel, args.String(), xmlEscape(def.WorkDir), env.String(), xmlEscape(def.WorkDir), xmlEscape(def.WorkDir))

	if err := os.MkdirAll(filepath.Dir(st.UnitPath), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(st.UnitPath, []byte(plist), 0644); err != nil {
		return "", err
	}
	_ = os.MkdirAll(filepath.Join(def.WorkDir, "data"), 0755)
	_ = run("launchctl", "unload", st.UnitPath) // 幂等
	if err := run("launchctl", "load", st.UnitPath); err != nil {
		return "", fmt.Errorf("launchctl load 失败: %w", err)
	}
	return "已安装 launchd 服务（登录自启）", nil
}

// installSystemdUser Linux：生成 systemd user unit + 环境文件并 enable --now。
func installSystemdUser(st Status, def definition) (string, error) {
	// 环境文件与 unit 同目录：内聚且便于测试隔离
	envPath := filepath.Join(filepath.Dir(st.UnitPath), "service.env")
	var envLines strings.Builder
	for _, kv := range def.Env {
		envLines.WriteString(fmt.Sprintf("%s=%s\n", kv[0], kv[1]))
	}
	if err := os.MkdirAll(filepath.Dir(envPath), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(envPath, []byte(envLines.String()), 0600); err != nil {
		return "", err
	}

	unit := fmt.Sprintf(`[Unit]
Description=zentao-max (禅道 Max)
After=network-online.target

[Service]
Type=simple
WorkingDirectory=%s
ExecStart=%s
EnvironmentFile=%s
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`, def.WorkDir, def.ExecPath, envPath)

	if err := os.MkdirAll(filepath.Dir(st.UnitPath), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(st.UnitPath, []byte(unit), 0644); err != nil {
		return "", err
	}
	_ = os.MkdirAll(filepath.Join(def.WorkDir, "data"), 0755)
	if err := run("systemctl", "--user", "daemon-reload"); err != nil {
		return "", fmt.Errorf("systemctl daemon-reload 失败（需要 systemd）: %w", err)
	}
	// 尝试开启 lingering：未登录也常驻（失败不影响安装，仅登出后停止）
	_ = run("loginctl", "enable-linger")
	if err := run("systemctl", "--user", "enable", "--now", ServiceLabel+".service"); err != nil {
		return "", fmt.Errorf("systemctl enable --now 失败: %w", err)
	}
	return "已安装 systemd 用户服务（开机自启 + 尝试未登录常驻）", nil
}

// installWindowsTask Windows：生成 run.bat 包装（保证工作目录）+ 计划任务登录自启。
func installWindowsTask(st Status, def definition) (string, error) {
	bat := fmt.Sprintf(`@echo off
cd /d "%s"
"%s" %s >> "%s\data\service.log" 2>&1
`, def.WorkDir, def.ExecPath, strings.Join(def.Args, " "), def.WorkDir)
	batPath := st.UnitPath // bat 路径（Detect 已指向 ~/zentao-max-service.bat）
	if err := os.MkdirAll(filepath.Dir(batPath), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(batPath, []byte(bat), 0644); err != nil {
		return "", err
	}
	_ = os.MkdirAll(filepath.Join(def.WorkDir, "data"), 0755)
	if err := run("schtasks", "/create", "/sc", "onlogon", "/tn", ServiceLabel,
		"/tr", fmt.Sprintf(`"%s"`, batPath), "/f"); err != nil {
		return "", fmt.Errorf("schtasks 创建失败（可能需要权限）: %w", err)
	}
	return "已创建计划任务（登录自启）", nil
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return r.Replace(s)
}

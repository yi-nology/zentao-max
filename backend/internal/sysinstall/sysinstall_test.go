package sysinstall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLaunchAgentPlist(t *testing.T) {
	def := definition{
		ExecPath: "/opt/app/zentao-max",
		Args:     []string{"/opt/app/zentao-max", "-static", "/opt/app/static"},
		WorkDir:  "/opt/app",
		Env:      [][2]string{{"PORT", "23456"}, {"A&B", "<x>"}},
	}
	st := Status{UnitPath: filepath.Join(t.TempDir(), ServiceLabel + ".plist")}
	msg, err := installLaunchAgent(st, def)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if msg == "" {
		t.Fatal("empty message")
	}
	data, err := os.ReadFile(st.UnitPath)
	if err != nil {
		t.Fatalf("plist not written: %v", err)
	}
	s := string(data)
	for _, want := range []string{
		ServiceLabel, "/opt/app/zentao-max", "/opt/app",
		"<key>RunAtLoad</key>", "EnvironmentVariables", "PORT", "A&amp;B", "&lt;x&gt;",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("plist missing %q", want)
		}
	}
	_ = os.Remove(st.UnitPath)
}

func TestSystemdUserUnit(t *testing.T) {
	def := definition{
		ExecPath: "/opt/app/zentao-max",
		Args:     []string{"/opt/app/zentao-max"},
		WorkDir:  "/opt/app",
		Env:      [][2]string{{"ZENTAO_ENCRYPTION_KEY", "abc"}},
	}
	st := Status{UnitPath: filepath.Join(t.TempDir(), ServiceLabel + ".service")}
	_, err := installSystemdUser(st, def)
	if err != nil && !strings.Contains(err.Error(), "systemctl") && !strings.Contains(err.Error(), "no such file") {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(st.UnitPath)
	if err != nil {
		t.Fatalf("unit not written: %v", err)
	}
	s := string(data)
	for _, want := range []string{
		"ExecStart=/opt/app/zentao-max", "WorkingDirectory=/opt/app",
		"EnvironmentFile=", "WantedBy=default.target",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("unit missing %q", want)
		}
	}
	envFile := filepath.Join(filepath.Dir(st.UnitPath), "service.env")
	envData, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatalf("service.env not written: %v", err)
	}
	if !strings.Contains(string(envData), "ZENTAO_ENCRYPTION_KEY=abc") {
		t.Errorf("service.env missing encryption key")
	}
	_ = os.Remove(st.UnitPath)
	}

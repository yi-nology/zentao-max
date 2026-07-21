package config

import (
	"testing"
)

// TestMCPConfig_Defaults 验证 MCP 配置默认值.
func TestMCPConfig_Defaults(t *testing.T) {
	// Get() 在未初始化时返回带默认值的配置
	cfg := Get()

	if !cfg.MCP.Enabled {
		t.Error("expected mcp.enabled default true")
	}
	if cfg.MCP.Transport != "http" {
		t.Errorf("expected mcp.transport default http, got %s", cfg.MCP.Transport)
	}
	if cfg.MCP.ReadOnly {
		t.Error("expected mcp.read_only default false")
	}
}

// TestValidate_MCPTransport 验证 MCP transport 取值校验.
func TestValidate_MCPTransport(t *testing.T) {
	validTransports := []string{"http", "stdio", "both", ""}
	for _, tr := range validTransports {
		cfg := &Config{}
		cfg.MCP.Transport = tr
		// 只测 MCP transport 校验分支，其他字段填合法默认避免干扰
		cfg.Server.Type = "http"
		cfg.Server.Port = "12345"
		cfg.Log.Level = "info"
		cfg.Log.Format = "console"
		cfg.RateLimit.RequestsPerMinute = 60
		cfg.RateLimit.BlockDurationMinutes = 5
		cfg.Server.ReadTimeout = 120
		cfg.Server.WriteTimeout = 120
		cfg.Server.ShutdownTimeout = 30
		if err := validate(cfg); err != nil {
			t.Errorf("transport %q should be valid, got error: %v", tr, err)
		}
	}

	// 非法值应报错
	cfg := &Config{}
	cfg.MCP.Transport = "grpc"
	cfg.Server.Type = "http"
	cfg.Server.Port = "12345"
	cfg.Log.Level = "info"
	cfg.Log.Format = "console"
	cfg.RateLimit.RequestsPerMinute = 60
	cfg.RateLimit.BlockDurationMinutes = 5
	cfg.Server.ReadTimeout = 120
	cfg.Server.WriteTimeout = 120
	cfg.Server.ShutdownTimeout = 30
	if err := validate(cfg); err == nil {
		t.Error("invalid transport 'grpc' should fail validation")
	}
}

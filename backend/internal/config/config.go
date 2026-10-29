package config

import (
	"fmt"
	"os"
	"time"

	oekitconfig "github.com/oekit/core/config"
)

// Config 应用配置结构.
// 加载顺序：DefaultConfig() → config.yaml（LoadFile）→ 环境变量覆盖（ApplyEnvOverlay）。
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Zentao    ZentaoConfig    `yaml:"zentao"`
	Auth      AuthConfig      `yaml:"auth"`
	Security  SecurityConfig  `yaml:"security"`
	Log       LogConfig       `yaml:"log"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	MCP       MCPConfig       `yaml:"mcp"`
	Update    UpdateConfig    `yaml:"update"`
}

// UpdateConfig 软件版本检查/更新配置.
type UpdateConfig struct {
	// 是否启用版本检查（启动注册 + 定期检查；关闭后设置页仍可手动检查）
	Enabled bool `yaml:"enabled"`
	// 自建更新服务地址（POST JSON：{app,version,os,arch}；响应：
	// {latest_version,download_url,notes}）。留空则回退 GitHub Releases 查询。
	CheckURL string `yaml:"check_url"`
	// GitHub 仓库（owner/name），check_url 为空时的默认检查源
	GitHubRepo string `yaml:"github_repo"`
	// 定期检查间隔（小时，默认 24）
	IntervalHours int `yaml:"interval_hours"`
	// 更新包下载目录
	DownloadDir string `yaml:"download_dir"`
}

// MCPConfig MCP 子系统配置
// 控制传输模式、运行时开关、Token 鉴权、只读模式与 action 白名单.
type MCPConfig struct {
	// 是否启用 MCP 服务（运行时可通过 /api/v1/mcp/config 热重载）
	Enabled bool `yaml:"enabled"`
	// 传输模式: "http" | "stdio" | "both"
	// 由启动入口决定，运行时不可改
	Transport string `yaml:"transport"`
	// 鉴权 Token，为空表示不鉴权（运行时热重载，仅存哈希）
	Token string `yaml:"token"`
	// 只读模式，禁止写操作（当前 MCP 全为查询接口，为未来扩展预留）
	ReadOnly bool `yaml:"read_only"`
	// action 白名单，为空表示允许全部；非空仅允许列表内的 action
	Actions []string `yaml:"actions"`
}

// ServerConfig 服务器配置.
type ServerConfig struct {
	// 应用类型: http
	Type string `yaml:"type"`
	// HTTP服务器端口
	Port string `yaml:"port"`
	// 静态资源路径（仅用于HTTP模式）
	StaticPath string `yaml:"static_path"`
	// 读超时时间（秒）
	ReadTimeout int `yaml:"read_timeout"`
	// 写超时时间（秒）
	WriteTimeout int `yaml:"write_timeout"`
	// 优雅关闭超时时间（秒）
	ShutdownTimeout int `yaml:"shutdown_timeout"`
}

// ZentaoConfig 禅道配置.
type ZentaoConfig struct {
	// 禅道服务器地址
	Server string `yaml:"server"`
	// 禅道账号
	Account string `yaml:"account"`
	// 禅道密码
	Password string `yaml:"password"`
	// 认证域（kydc / local）
	Realm string `yaml:"realm"`
	// 请求超时时间（秒）
	RequestTimeout int `yaml:"request_timeout"`
}

// AuthConfig 认证配置.
type AuthConfig struct {
	// 认证配置文件路径
	ConfigPath string `yaml:"config_path"`
	// 认证数据库路径
	DBPath string `yaml:"db_path"`
	// 平台访问控制：匿名=只读，登录管理员=读写
	Admin AuthAdminConfig `yaml:"admin"`
	// SSO 麒麟统一认证登录（用户名+密码+短信验证码）
	SSO SSOAuthConfig `yaml:"sso"`
}

// AuthAdminConfig 平台管理员账号（用于登录后获得读写权限）
type AuthAdminConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// SSOAuthConfig SSO 登录配置.
// 对接「用户名+密码+短信验证码」协议的统一认证平台；平台地址不内置默认值，
// 由部署配置提供（代码保持中性，开源发布不含具体平台地址）。
type SSOAuthConfig struct {
	// 是否启用 SSO 登录入口（启用后 /api/auth/sso/* 可用，登录页默认展示 SSO 表单）。
	// 启用时必须同时配置 base_url，否则视为未启用（启动时告警）。
	Enabled bool `yaml:"enabled"`
	// SSO 服务地址（统一认证平台根地址）
	BaseURL string `yaml:"base_url"`
	// SSO 登录页完整地址（作为 Referer 发送，服务端可能校验）。
	// 留空时回退为 {base_url}/sso/auth。
	AuthPageURL string `yaml:"auth_page_url"`
}

// SecurityConfig 安全配置.
type SecurityConfig struct {
	// 加密密钥
	EncryptionKey string `yaml:"encryption_key"`
	// CORS允许的域名（多个域名用逗号分隔）
	AllowedOrigins []string `yaml:"allowed_origins"`
}

// LogConfig 日志配置.
type LogConfig struct {
	// 日志级别: debug, info, warn, error
	Level string `yaml:"level"`
	// 日志格式: json, console
	Format string `yaml:"format"`
	// 日志输出路径（空表示标准输出）
	OutputPath string `yaml:"output_path"`
	// 是否启用调用者信息
	EnableCaller bool `yaml:"enable_caller"`
	// 是否启用堆栈跟踪
	EnableStacktrace bool `yaml:"enable_stacktrace"`
}

// RateLimitConfig 限流配置.
type RateLimitConfig struct {
	// 每分钟允许的请求数
	RequestsPerMinute int `yaml:"requests_per_minute"`
	// 超限后的封禁时长（分钟）
	BlockDurationMinutes int `yaml:"block_duration_minutes"`
}

// 全局配置实例.
var globalConfig *Config

// DefaultConfig 返回带全部默认值的配置（原 viper setDefaults 的结构体化）.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Type:            "http",
			Port:            "12345",
			ReadTimeout:     120,
			WriteTimeout:    120,
			ShutdownTimeout: 30,
		},
		Zentao: ZentaoConfig{
			RequestTimeout: 120,
		},
		Auth: AuthConfig{
			// ConfigPath 默认 ./auth_config.enc
			ConfigPath: "./auth_config.enc",
			// DBPath 空 = 交由 initialization.ResolveAuthDBPath 统一解析
			// （ZENTAO_AUTH_DB_PATH env → 已存在候选 → ./data/auth.db）
			DBPath: "",
			Admin: AuthAdminConfig{
				// 平台管理员账号默认 admin/admin（匿名只读，登录后读写）
				Username: "admin",
				Password: "admin",
			},
			// SSO 登录默认关闭：平台地址由部署侧 config.yaml 提供（不内置默认值），
			// 需要时设置 auth.sso.enabled=true + base_url（见 config.yaml.example）
			SSO: SSOAuthConfig{
				Enabled: false,
			},
		},
		Log: LogConfig{
			Level:            "info",
			Format:           "console",
			EnableCaller:     true,
			EnableStacktrace: false,
		},
		RateLimit: RateLimitConfig{
			// 默认 600 次/分钟——前端单页加载会并发请求多个接口，
			// 60 次/分钟在正常使用下极易触发，触发后封禁 5 分钟会让应用完全不可用
			RequestsPerMinute:    600,
			BlockDurationMinutes: 1,
		},
		MCP: MCPConfig{
			Enabled:   true,
			Transport: "http",
			ReadOnly:  false,
		},
		Update: UpdateConfig{
			Enabled:       true,
			CheckURL:      "",
			GitHubRepo:    "yi-nology/zentao-max",
			IntervalHours: 24,
			DownloadDir:   "./data/updates",
		},
	}
}

// configSearchPaths 未显式指定路径时按序查找的默认位置.
var configSearchPaths = []string{"./config.yaml", "./backend/config.yaml", "./config/config.yaml"}

// Init 初始化配置
// configPath: 配置文件路径（可选，空则按默认位置查找；文件不存在时仅用默认值+环境变量）
// envPrefix: 环境变量前缀（如 "ZENTAO_MAX"，派生规则 server.port → ZENTAO_MAX_SERVER_PORT）.
func Init(configPath string, envPrefix string) error {
	cfg := DefaultConfig()

	// 配置文件（存在才加载；加载失败直接报错，与旧 viper 语义一致——
	// 只有"文件不存在"是允许的，文件存在但解析失败必须暴露）
	paths := []string{configPath}
	if configPath == "" {
		paths = configSearchPaths
	}
	for _, p := range paths {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if err := oekitconfig.LoadFile(p, cfg); err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}
		break
	}

	// 环境变量覆盖（12-factor：env 优先于配置文件）
	if envPrefix != "" {
		if err := oekitconfig.ApplyEnvOverlay(cfg, oekitconfig.WithEnvNaming(oekitconfig.EnvNaming{
			Prefix: envPrefix + "_",
		})); err != nil {
			return fmt.Errorf("failed to apply env overlay: %w", err)
		}
	}

	// 验证配置
	if err := validate(cfg); err != nil {
		// 验证失败时仍然设置配置，避免 Get() 返回 nil
		globalConfig = cfg
		return fmt.Errorf("config validation failed: %w", err)
	}

	globalConfig = cfg
	return nil
}

// validate 验证配置.
func validate(cfg *Config) error {
	// 验证服务器配置
	if cfg.Server.Type != "http" {
		return fmt.Errorf("invalid server type: %s, must be 'http'", cfg.Server.Type)
	}

	if cfg.Server.Type == "http" {
		if cfg.Server.Port == "" {
			return fmt.Errorf("server port is required for http mode")
		}
	}

	// 验证日志级别
	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[cfg.Log.Level] {
		return fmt.Errorf("invalid log level: %s, must be one of: debug, info, warn, error", cfg.Log.Level)
	}

	// 验证日志格式
	if cfg.Log.Format != "json" && cfg.Log.Format != "console" {
		return fmt.Errorf("invalid log format: %s, must be 'json' or 'console'", cfg.Log.Format)
	}

	// 验证限流配置
	if cfg.RateLimit.RequestsPerMinute <= 0 {
		return fmt.Errorf("rate limit requests per minute must be positive")
	}

	if cfg.RateLimit.BlockDurationMinutes <= 0 {
		return fmt.Errorf("rate limit block duration must be positive")
	}

	// 验证超时配置
	if cfg.Server.ReadTimeout <= 0 {
		return fmt.Errorf("server read timeout must be positive")
	}

	if cfg.Server.WriteTimeout <= 0 {
		return fmt.Errorf("server write timeout must be positive")
	}

	if cfg.Server.ShutdownTimeout <= 0 {
		return fmt.Errorf("server shutdown timeout must be positive")
	}

	// 验证 MCP 传输模式
	validMCPTransports := map[string]bool{"http": true, "stdio": true, "both": true}
	if cfg.MCP.Transport != "" && !validMCPTransports[cfg.MCP.Transport] {
		return fmt.Errorf("invalid mcp transport: %s, must be one of: http, stdio, both", cfg.MCP.Transport)
	}

	return nil
}

// Get 获取全局配置实例，未初始化时返回带默认值的空配置.
func Get() *Config {
	if globalConfig == nil {
		// 未初始化时返回带默认值的配置，避免 panic
		return DefaultConfig()
	}
	return globalConfig
}

// GetServerTimeout 获取服务器超时配置（转换为time.Duration）.
func (c *ServerConfig) GetReadTimeout() time.Duration {
	return time.Duration(c.ReadTimeout) * time.Second
}

func (c *ServerConfig) GetWriteTimeout() time.Duration {
	return time.Duration(c.WriteTimeout) * time.Second
}

func (c *ServerConfig) GetShutdownTimeout() time.Duration {
	return time.Duration(c.ShutdownTimeout) * time.Second
}

// GetZentaoTimeout 获取禅道请求超时配置.
func (c *ZentaoConfig) GetRequestTimeout() time.Duration {
	return time.Duration(c.RequestTimeout) * time.Second
}

// GetTokenRefreshInterval 获取Token刷新间隔.
// 会话模式下没有 token 刷新（保活由 SessionTransport 30min 一次 IsLoggedIn 同步），
// 保留方法以兼容调用方（http_app 等），返回 0 表示不刷新。
func (c *ZentaoConfig) GetTokenRefreshInterval() time.Duration {
	return 0
}

// GetBlockDuration 获取限流封禁时长.
func (c *RateLimitConfig) GetBlockDuration() time.Duration {
	return time.Duration(c.BlockDurationMinutes) * time.Minute
}

// IsProduction 是否为生产环境.
func (c *Config) IsProduction() bool {
	return c.Log.Format == "json"
}

// IsDevelopment 是否为开发环境.
func (c *Config) IsDevelopment() bool {
	return c.Log.Format == "console"
}

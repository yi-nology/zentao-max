package mcp

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
	"sync"

	"github.com/yi-nology/zentao-max/backend/core/config"
)

// 传输模式常量.
const (
	TransportHTTP  = "http"
	TransportStdio = "stdio"
	TransportBoth  = "both"
)

// MCPModeManager MCP 模式管理器（全局单例）
//
// 承载三种"模式"概念，统一管理 MCP 子系统的运行时状态：
//  1. 传输模式 transport：http | stdio | both（由启动入口决定，运行时只读）
//  2. 开关 enabled + action 白名单（运行时可通过 API 热重载）
//  3. 只读模式 readOnly + Token 鉴权（运行时热重载）
//
// 所有可变字段由 sync.RWMutex 保护，支持并发读写。
type MCPModeManager struct {
	mu sync.RWMutex

	transport string   // 当前传输模式（只读，仅 SetTransport 设置一次）
	enabled   bool     // 是否启用 MCP 服务
	tokenHash string   // 鉴权 token 的 SHA-256 哈希（hex）；空表示不鉴权
	readOnly  bool     // 只读模式
	actions   []string // action 白名单；nil/空表示允许全部
}

var (
	modeManagerOnce sync.Once
	modeManager     *MCPModeManager
)

// GetMCPModeManager 获取全局 MCPModeManager 单例（懒初始化）.
func GetMCPModeManager() *MCPModeManager {
	modeManagerOnce.Do(func() {
		modeManager = &MCPModeManager{
			transport: TransportHTTP,
			enabled:   true,
		}
	})
	return modeManager
}

// InitFromConfig 从配置初始化模式管理器（进程启动时调用一次）
// transport 不从此设置（由启动入口用 SetTransport 单独指定）.
func (m *MCPModeManager) InitFromConfig(cfg config.MCPConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = cfg.Enabled
	m.readOnly = cfg.ReadOnly
	m.tokenHash = hashToken(cfg.Token)
	// 复制切片避免外部修改
	if len(cfg.Actions) > 0 {
		m.actions = append([]string(nil), cfg.Actions...)
	} else {
		m.actions = nil
	}
}

// SetTransport 设置当前传输模式（仅启动入口调用一次，运行时不再改）.
func (m *MCPModeManager) SetTransport(mode string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.transport = mode
}

// Transport 返回当前传输模式.
func (m *MCPModeManager) Transport() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.transport
}

// IsEnabled 是否启用 MCP 服务.
func (m *MCPModeManager) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// IsReadOnly 是否为只读模式.
func (m *MCPModeManager) IsReadOnly() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.readOnly
}

// IsActionAllowed 判断 action 是否被白名单允许
// 白名单为空（nil 或长度 0）表示允许全部.
func (m *MCPModeManager) IsActionAllowed(action string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.actions) == 0 {
		return true
	}
	for _, a := range m.actions {
		if a == action {
			return true
		}
	}
	return false
}

// VerifyToken 校验提供的 token 是否与配置一致（常量时间比较）
// 未配置 token 时（tokenHash 为空）直接放行.
func (m *MCPModeManager) VerifyToken(provided string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.tokenHash == "" {
		return true
	}
	h := hashToken(provided)
	// subtle.ConstantTimeCompare 要求等长，hashToken 结果长度固定，可直接比较
	if len(h) != len(m.tokenHash) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(h), []byte(m.tokenHash)) == 1
}

// HasToken 是否配置了鉴权 Token.
func (m *MCPModeManager) HasToken() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tokenHash != ""
}

// MCPStatus MCP 当前运行状态快照（供 GET /api/v1/mcp/config 等接口返回）
// 注意：Token 永不返回明文，仅返回 tokenSet 布尔值.
type MCPStatus struct {
	Enabled   bool     `json:"enabled"`
	Transport string   `json:"transport"`
	ReadOnly  bool     `json:"readOnly"`
	TokenSet  bool     `json:"tokenSet"`
	Actions   []string `json:"actions,omitempty"` // 空（不设置）表示允许全部
	ActionAll bool     `json:"actionAll"`         // true 表示允许全部（actions 为空）
}

// GetStatus 返回当前模式状态快照.
func (m *MCPModeManager) GetStatus() MCPStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	status := MCPStatus{
		Enabled:   m.enabled,
		Transport: m.transport,
		ReadOnly:  m.readOnly,
		TokenSet:  m.tokenHash != "",
		ActionAll: len(m.actions) == 0,
	}
	if len(m.actions) > 0 {
		status.Actions = append([]string(nil), m.actions...)
	}
	return status
}

// MCPUpdateRequest 运行时更新请求（指针字段实现部分更新，nil 表示不修改）
// Actions 为空切片且非 nil 表示清空白名单（允许全部）；
// 若想不修改 Actions，请显式传 nil（在 JSON 中省略该字段即可）.
type MCPUpdateRequest struct {
	Enabled  *bool     `json:"enabled,omitempty"`
	ReadOnly *bool     `json:"readOnly,omitempty"`
	Token    *string   `json:"token,omitempty"` // 设为空串表示关闭鉴权
	Actions  *[]string `json:"actions,omitempty"`
}

// UpdateStatus 应用运行时更新（热重载）.
func (m *MCPModeManager) UpdateStatus(req MCPUpdateRequest) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if req.Enabled != nil {
		m.enabled = *req.Enabled
	}
	if req.ReadOnly != nil {
		m.readOnly = *req.ReadOnly
	}
	if req.Token != nil {
		m.tokenHash = hashToken(*req.Token)
	}
	if req.Actions != nil {
		if len(*req.Actions) == 0 {
			m.actions = nil
		} else {
			m.actions = append([]string(nil), (*req.Actions)...)
		}
	}
}

// hashToken 计算 token 的 SHA-256 哈希（hex 编码）
// 空 token 返回空串（表示不鉴权）.
func hashToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

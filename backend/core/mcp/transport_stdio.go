package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

// StdioTransport stdio 传输层
//
// 通过 stdin/stdout 以 JSON Lines 协议与外部进程（如 Claude/Cursor）通信。
// 每行一个 JSON 请求：{"action":"...","params":{...}}，
// 响应以单行 JSON 写回：{"status":"ok","data":...} 或 {"status":"error","message":"..."}。
type StdioTransport struct {
	server *MCPServer
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	mutex  sync.Mutex
}

// NewStdioTransport 创建 stdio 传输层（使用 os.Stdin / os.Stdout / os.Stderr）.
func NewStdioTransport(server *MCPServer) *StdioTransport {
	return &StdioTransport{
		server: server,
		stdin:  os.Stdin,
		stdout: os.Stdout,
		stderr: os.Stderr,
	}
}

// NewStdioTransportWithIO 创建带自定义 IO 的 stdio 传输层（用于测试）.
func NewStdioTransportWithIO(server *MCPServer, stdin io.Reader, stdout io.Writer) *StdioTransport {
	return &StdioTransport{
		server: server,
		stdin:  stdin,
		stdout: stdout,
		stderr: os.Stderr,
	}
}

// Start 在后台 goroutine 中启动 stdio 监听（向后兼容旧用法）.
func (t *StdioTransport) Start() {
	go t.listen(context.Background())
}

// Run 阻塞当前 goroutine 监听 stdin，直到 ctx 取消或 stdin 关闭（EOF）
// 推荐在独立 stdio 入口（cmd/mcp）的主 goroutine 调用.
func (t *StdioTransport) Run(ctx context.Context) {
	// 标记当前进程为 stdio 传输模式
	GetMCPModeManager().SetTransport(TransportStdio)
	t.listen(ctx)
}

func (t *StdioTransport) listen(ctx context.Context) {
	decoder := json.NewDecoder(t.stdin)
	encoder := json.NewEncoder(t.stdout)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		var request map[string]interface{}
		if err := decoder.Decode(&request); err != nil {
			if err == io.EOF {
				break
			}
			t.sendError(encoder, fmt.Sprintf("Invalid request: %v", err))
			continue
		}

		action, _ := request["action"].(string)
		params, _ := request["params"].(map[string]interface{})

		// 统一访问控制检查（与 HTTP 传输一致）
		if blocked, msg := checkAccessStdio(action, params); blocked {
			t.sendError(encoder, msg)
			continue
		}

		result, err := t.server.HandleAction(action, params)
		if err != nil {
			t.sendError(encoder, err.Error())
			continue
		}

		t.send(encoder, result)
	}
}

// checkAccessStdio stdio 传输的访问控制检查
// 返回 (blocked, message)：blocked 为 true 时 message 为拒绝原因
// 检查顺序与 HTTP checkAccess 一致：总开关 → Token → 白名单 → 只读.
func checkAccessStdio(action string, params map[string]interface{}) (bool, string) {
	mgr := GetMCPModeManager()

	if !mgr.IsEnabled() {
		return true, "MCP service is disabled"
	}

	if mgr.HasToken() {
		token, _ := params["__token"].(string)
		if token == "" {
			// 兼容：token 也可放在顶层 "token" 字段
			token, _ = params["token"].(string)
		}
		if !mgr.VerifyToken(token) {
			return true, "unauthorized: invalid or missing token"
		}
	}

	if action == "" {
		return false, ""
	}

	if !mgr.IsActionAllowed(action) {
		return true, "action not allowed: " + action
	}

	if mgr.IsReadOnly() && IsWriteAction(action) {
		return true, "read-only mode: write action blocked: " + action
	}

	return false, ""
}

func (t *StdioTransport) send(encoder *json.Encoder, data interface{}) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if err := encoder.Encode(data); err != nil {
		// 写错误日志到 stderr，返回值无法处理故显式忽略
		_, _ = fmt.Fprintf(t.stderr, "Failed to send response: %v\n", err)
	}
}

func (t *StdioTransport) sendError(encoder *json.Encoder, msg string) {
	t.send(encoder, map[string]interface{}{
		"status":  "error",
		"message": msg,
	})
}

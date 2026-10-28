package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yi-nology/zentao-max/backend/internal/config"
)

// stdioTestHarness 构造一个 stdio transport 测试夹具
//
// 设计要点：
//   - stdin 用 io.Pipe，写完请求后关闭 write 端触发 EOF，让 listen 自然退出
//   - stdout 用 bytes.Buffer，由 sync.WaitGroup 保证 listen 完全退出后再读取
type stdioTestHarness struct {
	transport *StdioTransport
	pipeW     *io.PipeWriter
	stdoutBuf *bytes.Buffer
	wg        sync.WaitGroup
}

func newStdioTestHarness() *stdioTestHarness {
	resetManagerForTest()
	// ping 不需要真实 zentao client，可传 nil
	server := NewMCPServer(nil)
	pr, pw := io.Pipe()
	stdoutBuf := &bytes.Buffer{}
	transport := NewStdioTransportWithIO(server, pr, stdoutBuf)
	return &stdioTestHarness{
		transport: transport,
		pipeW:     pw,
		stdoutBuf: stdoutBuf,
	}
}

// startListen 在后台启动 listen 循环.
func (h *stdioTestHarness) startListen() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		defer cancel()
		h.transport.listen(ctx)
	}()
}

// finish 关闭 stdin 触发 EOF，并阻塞等待 listen 完全退出
// 必须在读取 responses() 之前调用，以避免并发读写 stdout buffer.
func (h *stdioTestHarness) finish() {
	_ = h.pipeW.Close()
	done := make(chan struct{})
	go func() {
		h.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
	}
}

// sendRequest 同步写入一行 JSON 请求到 stdin 管道.
func (h *stdioTestHarness) sendRequest(req map[string]interface{}) {
	data, _ := json.Marshal(req)
	_, _ = h.pipeW.Write(append(data, '\n'))
}

// responses 从 stdout 缓冲区解析所有 JSON 响应
// 必须在 finish() 之后调用.
func (h *stdioTestHarness) responses() []map[string]interface{} {
	var results []map[string]interface{}
	for _, line := range strings.Split(strings.TrimSpace(h.stdoutBuf.String()), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(line), &m); err == nil {
			results = append(results, m)
		}
	}
	return results
}

func TestStdioTransport_Ping(t *testing.T) {
	h := newStdioTestHarness()
	h.startListen()
	h.sendRequest(map[string]interface{}{"action": "ping"})
	// 给 listen 一点时间处理
	time.Sleep(50 * time.Millisecond)
	h.finish()

	resps := h.responses()
	if len(resps) == 0 {
		t.Fatal("expected at least one response, got none")
	}
	if resps[0]["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", resps[0]["status"])
	}
}

func TestStdioTransport_Disabled(t *testing.T) {
	h := newStdioTestHarness()
	enabled := false
	GetMCPModeManager().UpdateStatus(MCPUpdateRequest{Enabled: &enabled})

	h.startListen()
	h.sendRequest(map[string]interface{}{"action": "ping"})
	time.Sleep(50 * time.Millisecond)
	h.finish()

	resps := h.responses()
	if len(resps) == 0 {
		t.Fatal("expected a response")
	}
	if resps[0]["status"] != "error" {
		t.Errorf("expected error status when disabled, got %v", resps[0]["status"])
	}
	if !strings.Contains(resps[0]["message"].(string), "disabled") {
		t.Errorf("expected disabled message, got %v", resps[0]["message"])
	}
}

func TestStdioTransport_TokenRequired(t *testing.T) {
	h := newStdioTestHarness()
	GetMCPModeManager().InitFromConfig(config.MCPConfig{
		Enabled: true,
		Token:   "stdio-secret",
	})

	h.startListen()
	// 不带 token → 应被拒
	h.sendRequest(map[string]interface{}{"action": "ping"})
	time.Sleep(50 * time.Millisecond)
	// 带正确 token → 应通过
	h.sendRequest(map[string]interface{}{"action": "ping", "params": map[string]interface{}{"token": "stdio-secret"}})
	time.Sleep(50 * time.Millisecond)
	h.finish()

	resps := h.responses()
	if len(resps) < 2 {
		t.Fatalf("expected 2 responses, got %d: %v", len(resps), resps)
	}
	if resps[0]["status"] != "error" {
		t.Errorf("first request without token should fail, got %v", resps[0]["status"])
	}
	if resps[1]["status"] != "ok" {
		t.Errorf("second request with token should succeed, got %v", resps[1]["status"])
	}
}

func TestStdioTransport_ActionWhitelist(t *testing.T) {
	h := newStdioTestHarness()
	GetMCPModeManager().InitFromConfig(config.MCPConfig{
		Enabled: true,
		Actions: []string{"ping"},
	})

	h.startListen()
	// get_products 不在白名单 → 应被拒
	h.sendRequest(map[string]interface{}{"action": "get_products"})
	time.Sleep(50 * time.Millisecond)
	// ping 在白名单 → 应通过
	h.sendRequest(map[string]interface{}{"action": "ping"})
	time.Sleep(50 * time.Millisecond)
	h.finish()

	resps := h.responses()
	if len(resps) < 2 {
		t.Fatalf("expected 2 responses, got %d: %v", len(resps), resps)
	}
	if resps[0]["status"] != "error" {
		t.Errorf("get_products (not in whitelist) should be blocked, got %v", resps[0]["status"])
	}
	if resps[1]["status"] != "ok" {
		t.Errorf("ping (in whitelist) should succeed, got %v", resps[1]["status"])
	}
}

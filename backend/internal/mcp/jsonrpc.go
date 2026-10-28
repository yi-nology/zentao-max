// jsonrpc.go 实现标准 MCP 协议层（仅最新版本 2026-07-28，不提供旧版本兼容）。
//
// 2026-07-28 核心语义：
//   - 无 initialize 握手：每个请求在 params._meta 携带
//     io.modelcontextprotocol/protocolVersion，缺失或不支持返回 -32022
//   - server/discover：通告支持的版本、能力与身份（唯一免版本检查的方法）
//   - tools/list / tools/call：结果带 resultType:"complete"、ttlMs 缓存提示、
//     structuredContent 结构化输出
//   - ping / logging 等已从规范移除
//
// zentao-max 自有的 {"action":...,"params":{...}} 简化协议仍并行兼容（非 MCP 版本）。
package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/yi-nology/zentao-max/backend/internal/version"
)

// ProtocolVersion 本服务支持的唯一 MCP 协议版本（最新版）
const ProtocolVersion = "2026-07-28"

// UnsupportedProtocolVersion 错误码（2026-07-28 规范：-32022，MCP 规范保留段）
const codeUnsupportedProtocolVersion = -32022

// 列表类结果的缓存提示（2026-07-28 CacheableResult）
const toolsListTTLMS = 300000

// versionCheckError 构造版本错误响应（缺失/不支持统一 -32022，data 带正确版本指引）
func versionCheckError(id json.RawMessage, problem string) *JSONRPCResponse {
	return rpcErrorWithData(id, codeUnsupportedProtocolVersion,
		problem+" (only MCP "+ProtocolVersion+" is supported)",
		map[string]interface{}{
			"supported": []string{ProtocolVersion},
			"hint":      "call server/discover without _meta, or send params._meta['io.modelcontextprotocol/protocolVersion']='" + ProtocolVersion + "'",
		})
}

// requestProtocolVersion 从请求 params._meta 提取协议版本
// （meta key: io.modelcontextprotocol/protocolVersion）。
func requestProtocolVersion(params map[string]interface{}) string {
	meta, _ := params["_meta"].(map[string]interface{})
	if meta == nil {
		return ""
	}
	v, _ := meta["io.modelcontextprotocol/protocolVersion"].(string)
	return v
}

// JSONRPCRequest 标准 JSON-RPC 2.0 请求（也兼容通知，id 缺省）。
type JSONRPCRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      json.RawMessage        `json:"id,omitempty"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
	// 旧协议兼容字段：带 action 的请求按原有 JSON Lines 语义处理
	Action string `json:"action,omitempty"`
}

// JSONRPCResponse 标准 JSON-RPC 2.0 响应。
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// JSON-RPC 标准错误码
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternalError  = -32603
)

// IsJSONRPCRequest 判断一行/一个请求体是否为标准 MCP JSON-RPC
// （有 method 字段且非旧协议的 action 请求）。
func IsJSONRPCRequest(data []byte) bool {
	var probe struct {
		Method string `json:"method"`
		Action string `json:"action"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return false
	}
	return probe.Method != "" && probe.Action == ""
}

// HandleJSONRPC 处理单个标准 MCP JSON-RPC 请求。
// checkAccess 用于 tools/call 的白名单/只读检查（参数为工具名），可为 nil（跳过）。
// 返回值：响应（通知类返回 nil，不应写出）；旧协议兼容时返回 nil 且 fallback 置 true。
func (s *MCPServer) HandleJSONRPC(data []byte, checkAccess func(action string) (bool, string)) (*JSONRPCResponse, bool, error) {
	var req JSONRPCRequest
	// params 同时可能出现在 JSONRPCRequest.Params
	if err := json.Unmarshal(data, &req); err != nil {
		return rpcError(nil, codeParseError, "parse error: "+err.Error()), false, nil
	}
	if req.Action != "" {
		// 旧 JSON Lines 协议，交回原处理路径
		return nil, true, nil
	}
	if req.JSONRPC != "" && req.JSONRPC != "2.0" {
		return rpcError(req.ID, codeInvalidRequest, "unsupported jsonrpc version: "+req.JSONRPC), false, nil
	}

	// JSON-RPC 通知（无 id）：按规范不响应
	if len(req.ID) == 0 {
		return nil, false, nil
	}

	// 版本强制：除 server/discover 外，所有请求必须在 _meta 携带 2026-07-28
	if req.Method != "server/discover" {
		v := requestProtocolVersion(req.Params)
		if v == "" {
			return versionCheckError(req.ID, "missing io.modelcontextprotocol/protocolVersion in params._meta"), false, nil
		}
		if v != ProtocolVersion {
			return versionCheckError(req.ID, "unsupported protocol version: "+v), false, nil
		}
	}

	switch req.Method {
	case "server/discover":
		// 2026-07-28：无状态发现端点，通告支持的版本、能力与身份
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      rawID(req.ID),
			Result: map[string]interface{}{
				"protocolVersions": []string{ProtocolVersion},
				"latestVersion":    ProtocolVersion,
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{"listChanged": false},
				},
				"serverInfo": map[string]interface{}{
					"name":    "zentao-max",
					"title":   "禅道 Max",
					"version": serverVersion(),
				},
			},
		}, false, nil

	case "tools/list":
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      rawID(req.ID),
			Result: map[string]interface{}{
				"resultType": "complete",
				"ttlMs":      toolsListTTLMS,
				"cacheScope": "private",
				"tools":      Tools,
			},
		}, false, nil

	case "tools/call":
		name, _ := req.Params["name"].(string)
		if name == "" {
			return rpcError(req.ID, codeInvalidParams, "tools/call requires params.name"), false, nil
		}
		if GetToolByName(name) == nil {
			return rpcError(req.ID, codeInvalidParams, "unknown tool: "+name), false, nil
		}
		if checkAccess != nil {
			if blocked, msg := checkAccess(name); blocked {
				return rpcError(req.ID, codeInvalidParams, msg), false, nil
			}
		}
		toolArgs, _ := req.Params["arguments"].(map[string]interface{})
		result, err := s.HandleAction(name, toolArgs)
		if err != nil {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      rawID(req.ID),
				Result: map[string]interface{}{
					"content":    []map[string]interface{}{{"type": "text", "text": err.Error()}},
					"isError":    true,
					"resultType": "complete",
				},
			}, false, nil
		}
		payload, merr := json.Marshal(result)
		if merr != nil {
			payload = []byte(fmt.Sprintf("%v", result))
		}
		callResult := map[string]interface{}{
			"content":    []map[string]interface{}{{"type": "text", "text": string(payload)}},
			"resultType": "complete",
		}
		// 结构化工具输出：结果为 JSON 对象时附带 structuredContent
		var asObject map[string]interface{}
		if json.Unmarshal(payload, &asObject) == nil {
			callResult["structuredContent"] = asObject
		}
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      rawID(req.ID),
			Result:  callResult,
		}, false, nil

	default:
		return rpcError(req.ID, codeMethodNotFound, "method not found: "+req.Method), false, nil
	}
}

func rpcError(id json.RawMessage, code int, msg string) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      rawID(id),
		Error:   &RPCError{Code: code, Message: msg},
	}
}

func rpcErrorWithData(id json.RawMessage, code int, msg string, data interface{}) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      rawID(id),
		Error:   &RPCError{Code: code, Message: msg, Data: data},
	}
}

// rawID 把 json.RawMessage 转为可序列化 ID（缺失时为 null）。
func rawID(id json.RawMessage) interface{} {
	if len(id) == 0 {
		return nil
	}
	var v interface{}
	if err := json.Unmarshal(id, &v); err != nil {
		return string(id)
	}
	return v
}

func serverVersion() string {
	if version.Version == "" {
		return "dev"
	}
	return version.Version
}

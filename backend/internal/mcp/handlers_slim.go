package mcp

import (
	"encoding/json"
	"fmt"
)

// MCP 列表工具瘦身：列表里的 steps/spec/desc 等长文本对 AI 是纯 token 浪费
// （100 条 Bug 的 steps 全文可达数十 KB），详情全文用 get_*_detail 获取。
// 前端不消费 MCP 工具输出，走 REST 接口，故此处截断无 UI 影响。

// slimFieldMaxLen 字段截断长度（按 rune 计，中文友好）。
const slimFieldMaxLen = 200

// slimLongText 递归遍历已反序列化的 JSON 数据，把 map 中指定字段的超长字符串
// 按 rune 截断并追加截断标记；数组逐元素递归，其余类型原样返回。
func slimLongText(obj interface{}, fields map[string]bool, maxLen int) interface{} {
	switch t := obj.(type) {
	case map[string]interface{}:
		for k, v := range t {
			if fields[k] {
				if s, ok := v.(string); ok {
					runes := []rune(s)
					if len(runes) > maxLen {
						t[k] = string(runes[:maxLen]) +
							fmt.Sprintf("…[截断，全文 %d 字，详情用对应 get_*_detail 工具]", len(runes))
					}
				}
				continue
			}
			t[k] = slimLongText(v, fields, maxLen)
		}
		return t
	case []interface{}:
		for i, v := range t {
			t[i] = slimLongText(v, fields, maxLen)
		}
		return t
	default:
		return obj
	}
}

// slimViaJSON 把 result 经 JSON 往返转为可遍历形态后执行 slimLongText。
// 往返失败时原样返回——瘦身是尽力而为的优化，绝不影响数据正确性。
func slimViaJSON(result interface{}, fields []string) interface{} {
	set := make(map[string]bool, len(fields))
	for _, f := range fields {
		set[f] = true
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return result
	}
	var generic interface{}
	if err := json.Unmarshal(payload, &generic); err != nil {
		return result
	}
	return slimLongText(generic, set, slimFieldMaxLen)
}

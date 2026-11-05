package utils

import (
	"encoding/json"
	"strconv"
)

// ToFloat64 把禅道 SDK 的弱类型数值字段（interface{}：可能是 float64/int/int64/
// json.Number/数字字符串）统一转成 float64。无法解析时返回 0。
//
// 历史上存在五处同名 toFloat64、两种语义：dashboard 版用 time.ParseDuration 解析
// 字符串，"3.5" 这类普通小数字符串会静默返 0（bug）；其余版本用 ParseFloat。
// 此处统一为 ParseFloat 语义（issue #4）。禅道 API 的工时/预估字段是数字或
// 数字字符串，不存在 "1h30m" 时长格式输入。
func ToFloat64(v interface{}) float64 {
	switch n := v.(type) {
	case nil:
		return 0
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return f
	}
	return 0
}

// ToInt 是 ToFloat64 的整型版本：数字字符串经 ParseFloat 后截断取整。
func ToInt(v interface{}) int {
	switch n := v.(type) {
	case nil:
		return 0
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return int(f)
	}
	return 0
}

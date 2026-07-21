package logger

import (
	"sync"
	"time"
)

// LogEntry 一条日志的结构化记录（供前端日志页消费）
type LogEntry struct {
	Time    string `json:"time"`    // ISO8601 时间
	Level   string `json:"level"`   // info/warn/error/debug
	Message string `json:"message"` // 日志消息
	Caller  string `json:"caller"`  // 调用方
}

// LogRingBuffer 进程内的环形缓冲日志
// 通过 zap 的 WriteSyncer hook 把日志原文写入，由内部解析器解析
type LogRingBuffer struct {
	mu      sync.RWMutex
	entries []*LogEntry
	maxSize int
}

var (
	globalRingBuffer *LogRingBuffer
	ringOnce         sync.Once
)

// GetRingBuffer 获取全局日志环形缓冲实例（懒初始化）
func GetRingBuffer() *LogRingBuffer {
	ringOnce.Do(func() {
		globalRingBuffer = &LogRingBuffer{
			entries: make([]*LogEntry, 0, 1000),
			maxSize: 1000,
		}
	})
	return globalRingBuffer
}

// Append 追加一条日志
func (b *LogRingBuffer) Append(entry *LogEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.entries) >= b.maxSize {
		// 丢弃最旧的
		b.entries = b.entries[1:]
	}
	b.entries = append(b.entries, entry)
}

// Query 查询日志，支持按 level 过滤、关键字匹配、限制数量
// level 为空表示所有级别；keyword 为空表示不过滤
func (b *LogRingBuffer) Query(level string, keyword string, limit int) []*LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if limit <= 0 || limit > len(b.entries) {
		limit = len(b.entries)
	}

	// 倒序遍历（最新在前）
	result := make([]*LogEntry, 0, limit)
	for i := len(b.entries) - 1; i >= 0 && len(result) < limit; i-- {
		e := b.entries[i]
		if level != "" && e.Level != level {
			continue
		}
		if keyword != "" {
			if !containsCI(e.Message, keyword) && !containsCI(e.Caller, keyword) {
				continue
			}
		}
		result = append(result, e)
	}
	return result
}

// Size 返回当前缓冲的日志数（用于日志页显示）
func (b *LogRingBuffer) Size() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.entries)
}

// Clear 清空缓冲
func (b *LogRingBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries = b.entries[:0]
}

// containsCI 大小写不敏感的包含检查
func containsCI(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			c1 := s[i+j]
			c2 := substr[j]
			// 简单的 ASCII 大小写转换
			if c1 >= 'A' && c1 <= 'Z' {
				c1 += 32
			}
			if c2 >= 'A' && c2 <= 'Z' {
				c2 += 32
			}
			if c1 != c2 {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// NowISO 返回当前时间的 ISO8601 字符串（供手工追加日志时使用）
func NowISO() string {
	return time.Now().Format("2006-01-02T15:04:05.000Z07:00")
}

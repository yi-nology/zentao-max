// Package event 提供进程内事件总线
// 用于解耦后端业务层（如 scheduler）与前端推送层（如 Wails runtime）
package event

import (
	"sync"
	"sync/atomic"
)

// Topic 事件主题
type Topic string

const (
	// TaskCompleted 定时任务执行完成
	TaskCompleted Topic = "task.completed"
	// TaskFailed 定时任务执行失败
	TaskFailed Topic = "task.failed"
	// CacheInvalidated 缓存被失效
	CacheInvalidated Topic = "cache.invalidated"
)

// Event 事件对象
type Event struct {
	Topic   Topic
	Payload interface{}
}

// Handler 事件处理函数
type Handler func(event Event)

// subscription 内部订阅记录（带唯一 ID 便于取消）
type subscription struct {
	id      uint64
	handler Handler
}

// Bus 进程内事件总线（pub/sub）
type Bus struct {
	mu       sync.RWMutex
	handlers map[Topic][]subscription
}

// NewBus 创建新的事件总线
func NewBus() *Bus {
	return &Bus{
		handlers: make(map[Topic][]subscription),
	}
}

// Subscribe 订阅主题，返回取消订阅函数
func (b *Bus) Subscribe(topic Topic, handler Handler) func() {
	id := nextSubscriptionID()
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[topic] = append(b.handlers[topic], subscription{id: id, handler: handler})

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		subs := b.handlers[topic]
		for i, s := range subs {
			if s.id == id {
				b.handlers[topic] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
	}
}

// Publish 发布事件（非阻塞，每个 handler 在独立 goroutine 跑）
// handler 内部不应阻塞，否则会影响其他订阅者
func (b *Bus) Publish(topic Topic, payload interface{}) {
	b.mu.RLock()
	subs := make([]subscription, len(b.handlers[topic]))
	copy(subs, b.handlers[topic])
	b.mu.RUnlock()

	event := Event{Topic: topic, Payload: payload}
	for _, s := range subs {
		go s.handler(event)
	}
}

// SubscriberCount 返回某主题的订阅者数量（供调试用）
func (b *Bus) SubscriberCount(topic Topic) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.handlers[topic])
}

// globalBus 全局事件总线（便于不便于注入的场景调用）
var (
	globalBus     *Bus
	globalBusOnce sync.Once
	subscriberSeq atomic.Uint64
)

// GetGlobalBus 获取全局事件总线
func GetGlobalBus() *Bus {
	globalBusOnce.Do(func() {
		globalBus = NewBus()
	})
	return globalBus
}

// 给订阅者分配全局唯一 ID（避免同一 bus 实例内 ID 冲突）
func nextSubscriptionID() uint64 {
	return subscriberSeq.Add(1)
}

package event

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBus_PublishSubscribe(t *testing.T) {
	bus := NewBus()
	var got int32

	bus.Subscribe(TaskCompleted, func(e Event) {
		if e.Topic != TaskCompleted {
			t.Errorf("topic = %s, want %s", e.Topic, TaskCompleted)
		}
		atomic.StoreInt32(&got, 1)
	})

	bus.Publish(TaskCompleted, map[string]interface{}{"taskID": "t1"})

	// 等待 goroutine 完成
	time.Sleep(50 * time.Millisecond)
	if atomic.LoadInt32(&got) != 1 {
		t.Error("handler not called")
	}
}

func TestBus_MultipleSubscribers(t *testing.T) {
	bus := NewBus()
	var count int32

	for i := 0; i < 5; i++ {
		bus.Subscribe(TaskCompleted, func(e Event) {
			atomic.AddInt32(&count, 1)
		})
	}

	bus.Publish(TaskCompleted, nil)
	time.Sleep(100 * time.Millisecond)

	if got := atomic.LoadInt32(&count); got != 5 {
		t.Errorf("count = %d, want 5", got)
	}
}

func TestBus_Unsubscribe(t *testing.T) {
	bus := NewBus()
	var count int32

	unsub := bus.Subscribe(TaskCompleted, func(e Event) {
		atomic.AddInt32(&count, 1)
	})

	if bus.SubscriberCount(TaskCompleted) != 1 {
		t.Error("expected 1 subscriber")
	}

	unsub()

	if bus.SubscriberCount(TaskCompleted) != 0 {
		t.Error("expected 0 subscribers after unsubscribe")
	}

	bus.Publish(TaskCompleted, nil)
	time.Sleep(50 * time.Millisecond)

	if got := atomic.LoadInt32(&count); got != 0 {
		t.Errorf("after unsubscribe, count = %d, want 0", got)
	}
}

func TestBus_ConcurrentAccess(t *testing.T) {
	bus := NewBus()
	var wg sync.WaitGroup

	// 并发订阅
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.Subscribe(TaskCompleted, func(e Event) {})
		}()
	}

	// 并发发布
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.Publish(TaskCompleted, nil)
		}()
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond) // 等待 handler goroutines 完成

	if bus.SubscriberCount(TaskCompleted) != 50 {
		t.Errorf("subscriber count = %d, want 50", bus.SubscriberCount(TaskCompleted))
	}
}

func TestBus_DifferentTopics(t *testing.T) {
	bus := NewBus()
	var aCount, bCount int32

	bus.Subscribe(TaskCompleted, func(e Event) { atomic.AddInt32(&aCount, 1) })
	bus.Subscribe(TaskFailed, func(e Event) { atomic.AddInt32(&bCount, 1) })

	bus.Publish(TaskCompleted, nil)
	bus.Publish(TaskFailed, nil)
	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt32(&aCount) != 1 || atomic.LoadInt32(&bCount) != 1 {
		t.Errorf("aCount=%d, bCount=%d; want both 1", aCount, bCount)
	}
}

func TestGetGlobalBus(t *testing.T) {
	bus1 := GetGlobalBus()
	bus2 := GetGlobalBus()
	if bus1 != bus2 {
		t.Error("GetGlobalBus should return the same instance")
	}
}

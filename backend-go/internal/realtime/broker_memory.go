package realtime

import (
	"context"
	"sync"
)

// MemoryBroker 进程内 fanout 实现。
//
// 限制：仅适合单实例部署。集群必须用 Redis 实现。
type MemoryBroker struct {
	mu       sync.RWMutex
	subs     map[string]map[*subscription]struct{} // topic -> set
	closed   bool
	bufferSz int
}

type subscription struct {
	ch chan Message
}

// NewMemoryBroker 创建。bufferSize 是每订阅者通道的缓冲（建议 32~256）。
func NewMemoryBroker(bufferSize int) *MemoryBroker {
	if bufferSize <= 0 {
		bufferSize = 64
	}
	return &MemoryBroker{
		subs:     make(map[string]map[*subscription]struct{}),
		bufferSz: bufferSize,
	}
}

// Publish 把消息扇出到该 topic 的所有订阅者。
// 慢消费者：channel 满则丢弃该消息（不阻塞发布方）。
func (b *MemoryBroker) Publish(_ context.Context, topic string, msg Message) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return nil
	}
	msg.Topic = topic
	for sub := range b.subs[topic] {
		select {
		case sub.ch <- msg:
		default:
			// 慢消费者：丢弃，避免阻塞发布。
		}
	}
	return nil
}

// Subscribe 订阅 topic。ctx 取消后通道被关闭并自动退订。
func (b *MemoryBroker) Subscribe(ctx context.Context, topic string) (<-chan Message, error) {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		empty := make(chan Message)
		close(empty)
		return empty, nil
	}
	sub := &subscription{ch: make(chan Message, b.bufferSz)}
	if _, ok := b.subs[topic]; !ok {
		b.subs[topic] = make(map[*subscription]struct{})
	}
	b.subs[topic][sub] = struct{}{}
	b.mu.Unlock()

	go func() {
		<-ctx.Done()
		b.mu.Lock()
		if set, ok := b.subs[topic]; ok {
			delete(set, sub)
			if len(set) == 0 {
				delete(b.subs, topic)
			}
		}
		close(sub.ch)
		b.mu.Unlock()
	}()
	return sub.ch, nil
}

// Close 停止 broker 并关闭所有订阅。
func (b *MemoryBroker) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	for _, set := range b.subs {
		for sub := range set {
			close(sub.ch)
		}
	}
	b.subs = nil
	return nil
}

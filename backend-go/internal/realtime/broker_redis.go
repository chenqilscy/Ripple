package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"
)

// RedisBroker 用 Redis Pub/Sub 实现跨实例广播。
//
// Topic → Redis channel 名一一对应（透传）。
// 慢消费者：本地 channel 满则丢弃（与 MemoryBroker 一致），不阻塞 Redis。
type RedisBroker struct {
	rdb      *redis.Client
	bufferSz int

	mu     sync.Mutex
	closed bool
}

// NewRedisBroker 用现成的 Redis 客户端构建。bufferSize 建议 64~256。
func NewRedisBroker(rdb *redis.Client, bufferSize int) *RedisBroker {
	if bufferSize <= 0 {
		bufferSize = 64
	}
	return &RedisBroker{rdb: rdb, bufferSz: bufferSize}
}

// Publish 把 msg 序列化后发到 Redis channel = topic。
func (b *RedisBroker) Publish(ctx context.Context, topic string, msg Message) error {
	if b.isClosed() {
		return errors.New("redis broker closed")
	}
	msg.Topic = topic
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("redis broker: marshal: %w", err)
	}
	if err := b.rdb.Publish(ctx, topic, data).Err(); err != nil {
		return fmt.Errorf("redis broker: publish: %w", err)
	}
	return nil
}

// Subscribe 订阅 topic。ctx 取消后 PubSub 关闭，channel 关闭。
func (b *RedisBroker) Subscribe(ctx context.Context, topic string) (<-chan Message, error) {
	if b.isClosed() {
		empty := make(chan Message)
		close(empty)
		return empty, nil
	}
	ps := b.rdb.Subscribe(ctx, topic)
	// 等首次 sub 确认（避免后续 Publish 丢消息）
	if _, err := ps.Receive(ctx); err != nil {
		_ = ps.Close()
		return nil, fmt.Errorf("redis broker: subscribe %s: %w", topic, err)
	}
	out := make(chan Message, b.bufferSz)
	rawCh := ps.Channel()

	go func() {
		defer close(out)
		defer ps.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case raw, ok := <-rawCh:
				if !ok {
					return
				}
				var m Message
				if err := json.Unmarshal([]byte(raw.Payload), &m); err != nil {
					// 无法解析的消息丢弃，不阻塞
					continue
				}
				m.Topic = raw.Channel // Topic 是 json:"-"，序列化丢失，从 redis channel 还原
				select {
				case out <- m:
				default:
					// 慢消费者丢弃
				}
			}
		}
	}()
	return out, nil
}

// Close 标记关闭。已建立的订阅在自身 ctx 取消时自然结束。
func (b *RedisBroker) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	return nil
}

func (b *RedisBroker) isClosed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closed
}

// Package realtime 实现实时消息广播：Broker（pub/sub 抽象）+ Hub（连接管理）。
//
// Broker 接口允许后续替换为 Redis Streams 实现，
// 当前 memory 实现仅在单进程内 fanout（开发用）。
package realtime

import "context"

// Message 是发送给客户端的事件载荷。
// Topic 通常是 "lake:{lake_id}"，由发布方构造。
type Message struct {
	Topic   string         `json:"-"`
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload,omitempty"`
}

// Broker 是 pub/sub 抽象。Subscribe 返回的 channel 当 ctx 取消时关闭。
type Broker interface {
	Publish(ctx context.Context, topic string, msg Message) error
	Subscribe(ctx context.Context, topic string) (<-chan Message, error)
	Close() error
}

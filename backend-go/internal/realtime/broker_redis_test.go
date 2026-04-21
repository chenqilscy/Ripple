package realtime

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// 走真 Redis（fn.cky:16379）；CI 跳过条件 RIPPLE_REDIS_ADDR 缺失。
func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("RIPPLE_REDIS_ADDR")
	if addr == "" {
		t.Skip("RIPPLE_REDIS_ADDR 未设置 → 跳过 RedisBroker 真集成测")
	}
	cli := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: os.Getenv("RIPPLE_REDIS_PASS"),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := cli.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis 不可达：%v", err)
	}
	return cli
}

func TestRedisBroker_PublishSubscribe(t *testing.T) {
	cli := newTestRedis(t)
	defer cli.Close()
	b := NewRedisBroker(cli, 16)
	defer b.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	topic := "test:broker:" + time.Now().Format("150405.000000")
	ch, err := b.Subscribe(ctx, topic)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = b.Publish(ctx, topic, Message{Type: "ping", Payload: map[string]any{"k": "v"}})
	}()

	select {
	case msg := <-ch:
		if msg.Type != "ping" || msg.Payload["k"] != "v" {
			t.Fatalf("unexpected msg: %+v", msg)
		}
		if msg.Topic != topic {
			t.Fatalf("topic mismatch: %s", msg.Topic)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for message")
	}
}

func TestRedisBroker_TwoSubscribersFanout(t *testing.T) {
	cli := newTestRedis(t)
	defer cli.Close()
	b := NewRedisBroker(cli, 16)
	defer b.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	topic := "test:fanout:" + time.Now().Format("150405.000000")
	ch1, err := b.Subscribe(ctx, topic)
	if err != nil {
		t.Fatal(err)
	}
	ch2, err := b.Subscribe(ctx, topic)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		time.Sleep(80 * time.Millisecond)
		_ = b.Publish(ctx, topic, Message{Type: "broadcast"})
	}()

	for i, ch := range []<-chan Message{ch1, ch2} {
		select {
		case msg := <-ch:
			if msg.Type != "broadcast" {
				t.Fatalf("sub %d got %s", i, msg.Type)
			}
		case <-ctx.Done():
			t.Fatalf("sub %d timeout", i)
		}
	}
}

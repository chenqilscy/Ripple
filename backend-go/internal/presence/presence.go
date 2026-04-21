// Package presence 维护"哪些用户正在线于哪个湖"的瞬时状态。
//
// 设计：
//   - Redis ZSET：key=lake:{lake_id}:presence，成员=user_id，score=过期毫秒
//   - Join：ZADD + 广播 presence.joined；首次心跳或重复 Join 亦走此路径
//   - Heartbeat：ZADD（覆盖 score），不广播
//   - Leave：ZREM + 广播 presence.left
//   - List：ZRANGEBYSCORE (now, +inf) 并顺手 ZREMRANGEBYSCORE (-inf, now) 清理过期
//
// 备注：
//   - 无 Redis（rdb=nil）时回退到进程内 map，仅限单实例部署。
//   - TTL 设定 = 2×心跳间隔；客户端掉线最多 2×interval 后被发现。
package presence

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/realtime"
	"github.com/redis/go-redis/v9"
)

// DefaultTTL 会话默认有效期（对应前端 30s 心跳的 2× 容忍）。
const DefaultTTL = 60 * time.Second

// Service 在线状态服务。
type Service struct {
	rdb    *redis.Client   // 可空：单机部署回退到 memStore
	mem    *memStore       // 仅 rdb=nil 时启用
	broker realtime.Broker // 可空：静默
	ttl    time.Duration
}

// NewService 装配。rdb 或 broker 可为 nil；ttl<=0 用 DefaultTTL。
func NewService(rdb *redis.Client, broker realtime.Broker, ttl time.Duration) *Service {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	s := &Service{rdb: rdb, broker: broker, ttl: ttl}
	if rdb == nil {
		s.mem = newMemStore()
	}
	return s
}

func presenceKey(lakeID string) string { return "lake:" + lakeID + ":presence" }

// Join 将用户加入湖 presence，并广播 presence.joined。重复 Join 仅更新 score。
func (s *Service) Join(ctx context.Context, lakeID, userID string) error {
	if lakeID == "" || userID == "" {
		return fmt.Errorf("presence: lake_id and user_id required")
	}
	first, err := s.add(ctx, lakeID, userID)
	if err != nil {
		return err
	}
	if first {
		s.publish(ctx, lakeID, "presence.joined", userID)
	}
	return nil
}

// Heartbeat 刷新 score；不广播（避免噪声）。
func (s *Service) Heartbeat(ctx context.Context, lakeID, userID string) error {
	_, err := s.add(ctx, lakeID, userID)
	return err
}

// Leave 把用户移出 presence 并广播 presence.left（幂等：不存在也算成功）。
func (s *Service) Leave(ctx context.Context, lakeID, userID string) error {
	if lakeID == "" || userID == "" {
		return nil
	}
	existed, err := s.remove(ctx, lakeID, userID)
	if err != nil {
		return err
	}
	if existed {
		s.publish(ctx, lakeID, "presence.left", userID)
	}
	return nil
}

// List 返回当前湖内在线 user_id（过滤过期；顺手清理）。
func (s *Service) List(ctx context.Context, lakeID string) ([]string, error) {
	now := time.Now()
	if s.rdb == nil {
		return s.mem.list(lakeID, now), nil
	}
	nowMs := now.UnixMilli()
	// 清理 score <= now 的过期条目（不致命，失败不影响 list）。
	_ = s.rdb.ZRemRangeByScore(ctx, presenceKey(lakeID), "-inf", fmt.Sprintf("%d", nowMs)).Err()
	out, err := s.rdb.ZRangeByScore(ctx, presenceKey(lakeID), &redis.ZRangeBy{
		Min: fmt.Sprintf("(%d", nowMs), // 排除 score == now
		Max: "+inf",
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("presence list: %w", err)
	}
	return out, nil
}

// --- 内部 ---

// add 写 ZADD；返回 first=true 表示此前该用户不在此湖的活跃 set 里。
func (s *Service) add(ctx context.Context, lakeID, userID string) (first bool, err error) {
	now := time.Now()
	exp := now.Add(s.ttl)
	if s.rdb == nil {
		return s.mem.add(lakeID, userID, exp, now), nil
	}
	key := presenceKey(lakeID)
	// 先查此前活跃状态再写，避免原子性丧失时误判。
	prevScore, _ := s.rdb.ZScore(ctx, key, userID).Result()
	nowMs := now.UnixMilli()
	first = prevScore <= float64(nowMs) // 过期或不存在都视作首次
	if _, err := s.rdb.ZAdd(ctx, key, redis.Z{Score: float64(exp.UnixMilli()), Member: userID}).Result(); err != nil {
		return false, fmt.Errorf("presence add: %w", err)
	}
	// 给 key 一个上限 TTL，避免冷湖残留：ttl 的 10 倍。
	_ = s.rdb.Expire(ctx, key, s.ttl*10).Err()
	return first, nil
}

func (s *Service) remove(ctx context.Context, lakeID, userID string) (bool, error) {
	if s.rdb == nil {
		return s.mem.remove(lakeID, userID), nil
	}
	n, err := s.rdb.ZRem(ctx, presenceKey(lakeID), userID).Result()
	if err != nil {
		return false, fmt.Errorf("presence remove: %w", err)
	}
	return n > 0, nil
}

func (s *Service) publish(ctx context.Context, lakeID, eventType, userID string) {
	if s.broker == nil {
		return
	}
	_ = s.broker.Publish(ctx, realtime.LakeTopic(lakeID), realtime.Message{
		Type: eventType,
		Payload: map[string]any{
			"user_id": userID,
			"lake_id": lakeID,
			"ts":      time.Now().Unix(),
		},
	})
}

// --- memStore: rdb=nil 的单机回退 ---

type memStore struct {
	mu   sync.Mutex
	data map[string]map[string]time.Time // lakeID -> userID -> expireAt
}

func newMemStore() *memStore { return &memStore{data: map[string]map[string]time.Time{}} }

func (m *memStore) add(lakeID, userID string, exp, now time.Time) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	lake, ok := m.data[lakeID]
	if !ok {
		lake = map[string]time.Time{}
		m.data[lakeID] = lake
	}
	old, existed := lake[userID]
	lake[userID] = exp
	return !existed || !old.After(now)
}

func (m *memStore) remove(lakeID, userID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	lake, ok := m.data[lakeID]
	if !ok {
		return false
	}
	_, existed := lake[userID]
	delete(lake, userID)
	return existed
}

func (m *memStore) list(lakeID string, now time.Time) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []string{}
	lake, ok := m.data[lakeID]
	if !ok {
		return out
	}
	for uid, exp := range lake {
		if exp.After(now) {
			out = append(out, uid)
		} else {
			delete(lake, uid)
		}
	}
	return out
}

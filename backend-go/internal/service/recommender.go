// Package service · M3-S3 推荐器（M4-T3 增强：Redis 缓存 + 全局热度信号融合）。
//
// 算法（item-based + 简化 user-based + 全局热度）：
//  1. 取当前用户已 LIKE 的 N 个 target；
//  2. 对每个 target 找其他 LIKE 过它的用户（邻居）；
//  3. 取邻居合计 LIKE 过、当前用户未交互的 target，按 LIKE 频次排序（协同分）；
//  4. 融合全局热度信号：score = 2*collab + 1*global_hot；
//  5. 结果写入 Redis 缓存（5 分钟），命中直接返回。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
	"github.com/redis/go-redis/v9"
)

// RecommenderService 推荐服务。
type RecommenderService struct {
	feedback store.FeedbackRepository
	rdb      *redis.Client
	cacheTTL time.Duration
}

// NewRecommenderService 装配；rdb 可为 nil，nil 时跳过缓存。
func NewRecommenderService(feedback store.FeedbackRepository, rdb *redis.Client) *RecommenderService {
	return &RecommenderService{feedback: feedback, rdb: rdb, cacheTTL: 5 * time.Minute}
}

// Recommendation 单条推荐结果。
type Recommendation struct {
	TargetID string `json:"target_id"`
	Score    int64  `json:"score"`
}

// RecommendInput 推荐请求。
type RecommendInput struct {
	TargetType string
	Limit      int
}

// Recommend 返回 Top-K 推荐 target_id。空结果不报错。
func (s *RecommenderService) Recommend(ctx context.Context, actor *domain.User, in RecommendInput) ([]Recommendation, error) {
	if actor == nil || actor.ID == "" {
		return nil, errors.New("actor required")
	}
	if in.TargetType == "" {
		return nil, errors.New("target_type required")
	}
	if in.Limit <= 0 || in.Limit > 50 {
		in.Limit = 20
	}

	cacheKey := "reco:v2:" + actor.ID + ":" + in.TargetType + ":" + strconv.Itoa(in.Limit)
	if s.rdb != nil {
		if raw, err := s.rdb.Get(ctx, cacheKey).Result(); err == nil && raw != "" {
			var out []Recommendation
			if json.Unmarshal([]byte(raw), &out) == nil {
				return out, nil
			}
		}
	}

	mine, err := s.feedback.ListUserPositiveTargets(ctx, actor.ID, in.TargetType, 100)
	if err != nil {
		return nil, err
	}

	collabScores := map[string]int64{}
	if len(mine) > 0 {
		neighborSet := map[string]struct{}{}
		for _, tid := range mine {
			users, err := s.feedback.ListUsersWhoLiked(ctx, in.TargetType, tid, 100)
			if err != nil {
				return nil, err
			}
			for _, u := range users {
				if u == actor.ID {
					continue
				}
				neighborSet[u] = struct{}{}
			}
		}
		if len(neighborSet) > 0 {
			neighbors := make([]string, 0, len(neighborSet))
			for u := range neighborSet {
				neighbors = append(neighbors, u)
			}
			tcs, err := s.feedback.ListLikedByUsers(ctx, neighbors, in.TargetType, mine, in.Limit*2)
			if err != nil {
				return nil, err
			}
			for _, tc := range tcs {
				collabScores[tc.TargetID] = tc.Count
			}
		}
	}

	hotSignals, err := s.feedback.TopLikedTargets(ctx, in.TargetType, mine, in.Limit*2)
	if err != nil {
		return nil, err
	}
	hotScores := map[string]int64{}
	for _, tc := range hotSignals {
		hotScores[tc.TargetID] = tc.Count
	}

	merged := map[string]int64{}
	for tid, c := range collabScores {
		merged[tid] += 2 * c
	}
	for tid, h := range hotScores {
		merged[tid] += h
	}
	if len(merged) == 0 {
		return nil, nil
	}
	out := make([]Recommendation, 0, len(merged))
	for tid, sc := range merged {
		out = append(out, Recommendation{TargetID: tid, Score: sc})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].TargetID < out[j].TargetID
	})
	if len(out) > in.Limit {
		out = out[:in.Limit]
	}

	if s.rdb != nil {
		if b, err := json.Marshal(out); err == nil {
			_ = s.rdb.Set(ctx, cacheKey, b, s.cacheTTL).Err()
		}
	}
	return out, nil
}

// InvalidateUser 清除用户的推荐缓存。
func (s *RecommenderService) InvalidateUser(ctx context.Context, userID string) {
	if s.rdb == nil || userID == "" {
		return
	}
	pattern := "reco:v2:" + userID + ":*"
	iter := s.rdb.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		_ = s.rdb.Del(ctx, iter.Val()).Err()
	}
}

// Package service · M3-S3 推荐器骨架（最小协同过滤）。
//
// 算法（item-based + 简化 user-based）：
//  1. 取当前用户已 LIKE 的 N 个 target；
//  2. 对每个 target 找其他 LIKE 过它的用户；
//  3. 取这些"邻居"用户合计 LIKE 过的 target，去除当前用户已交互过的；
//  4. 按 LIKE 频次排序返回 Top-K。
//
// 注意：这是骨架，未做时间衰减/类目偏置/向量召回。后续 M3-S3.1 升级。
package service

import (
	"context"
	"errors"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
)

// RecommenderService 推荐服务。
type RecommenderService struct {
	feedback store.FeedbackRepository
}

// NewRecommenderService 装配。
func NewRecommenderService(feedback store.FeedbackRepository) *RecommenderService {
	return &RecommenderService{feedback: feedback}
}

// Recommendation 单条推荐结果。
type Recommendation struct {
	TargetID string `json:"target_id"`
	Score    int64  `json:"score"`
}

// RecommendInput 推荐请求。
type RecommendInput struct {
	TargetType string // 例如 "perma_node" / "lake"
	Limit      int    // <= 50；默认 20
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
	// 1. 我已 LIKE 的 target
	mine, err := s.feedback.ListUserPositiveTargets(ctx, actor.ID, in.TargetType, 100)
	if err != nil {
		return nil, err
	}
	if len(mine) == 0 {
		// 冷启动：暂返回空（后续可降级到全局热度）
		return nil, nil
	}
	// 2. 收集邻居用户（去重 + 排除自己）
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
	if len(neighborSet) == 0 {
		return nil, nil
	}
	neighbors := make([]string, 0, len(neighborSet))
	for u := range neighborSet {
		neighbors = append(neighbors, u)
	}
	// 3. 邻居们 LIKE 过、且我没交互过的 target
	tcs, err := s.feedback.ListLikedByUsers(ctx, neighbors, in.TargetType, mine, in.Limit)
	if err != nil {
		return nil, err
	}
	out := make([]Recommendation, 0, len(tcs))
	for _, tc := range tcs {
		out = append(out, Recommendation{TargetID: tc.TargetID, Score: tc.Count})
	}
	return out, nil
}

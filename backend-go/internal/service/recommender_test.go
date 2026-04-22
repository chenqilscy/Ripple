package service

import (
	"context"
	"sort"
	"testing"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
)

// memFeedbackRepo 内存 FeedbackRepository for test.
type memFeedbackRepo struct {
	// likes[(userID, targetType)] -> set of targetID
	likes map[string]map[string]struct{}
}

func newMemFeedback() *memFeedbackRepo {
	return &memFeedbackRepo{likes: map[string]map[string]struct{}{}}
}
func keyUT(u, t string) string { return u + "|" + t }

func (r *memFeedbackRepo) AddEvent(_ context.Context, ev store.FeedbackEvent) error {
	if ev.EventType != "LIKE" {
		return nil
	}
	k := keyUT(ev.UserID, ev.TargetType)
	if r.likes[k] == nil {
		r.likes[k] = map[string]struct{}{}
	}
	r.likes[k][ev.TargetID] = struct{}{}
	return nil
}
func (r *memFeedbackRepo) ListUserPositiveTargets(_ context.Context, userID, targetType string, _ int) ([]string, error) {
	s := r.likes[keyUT(userID, targetType)]
	out := make([]string, 0, len(s))
	for t := range s {
		out = append(out, t)
	}
	sort.Strings(out)
	return out, nil
}
func (r *memFeedbackRepo) ListUsersWhoLiked(_ context.Context, targetType, targetID string, _ int) ([]string, error) {
	out := []string{}
	for k, set := range r.likes {
		// k = "user|targetType"
		if len(k) < len(targetType)+1 || k[len(k)-len(targetType):] != targetType {
			continue
		}
		if _, ok := set[targetID]; ok {
			out = append(out, k[:len(k)-len(targetType)-1])
		}
	}
	sort.Strings(out)
	return out, nil
}
func (r *memFeedbackRepo) ListLikedByUsers(_ context.Context, userIDs []string, targetType string, exclude []string, limit int) ([]store.TargetCount, error) {
	excl := map[string]struct{}{}
	for _, e := range exclude {
		excl[e] = struct{}{}
	}
	count := map[string]int64{}
	for _, u := range userIDs {
		for t := range r.likes[keyUT(u, targetType)] {
			if _, skip := excl[t]; skip {
				continue
			}
			count[t]++
		}
	}
	out := make([]store.TargetCount, 0, len(count))
	for t, c := range count {
		out = append(out, store.TargetCount{TargetID: t, Count: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].TargetID < out[j].TargetID
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
func (r *memFeedbackRepo) TopLikedTargets(_ context.Context, targetType string, exclude []string, limit int) ([]store.TargetCount, error) {
	excl := map[string]struct{}{}
	for _, e := range exclude {
		excl[e] = struct{}{}
	}
	count := map[string]int64{}
	for k, set := range r.likes {
		if len(k) < len(targetType)+1 || k[len(k)-len(targetType):] != targetType {
			continue
		}
		for t := range set {
			if _, skip := excl[t]; skip {
				continue
			}
			count[t]++
		}
	}
	out := make([]store.TargetCount, 0, len(count))
	for t, c := range count {
		out = append(out, store.TargetCount{TargetID: t, Count: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].TargetID < out[j].TargetID
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func TestRecommender_HappyPath(t *testing.T) {
	fb := newMemFeedback()
	ctx := context.Background()
	// u1 likes A, B
	_ = fb.AddEvent(ctx, store.FeedbackEvent{UserID: "u1", TargetType: "perma", TargetID: "A", EventType: "LIKE"})
	_ = fb.AddEvent(ctx, store.FeedbackEvent{UserID: "u1", TargetType: "perma", TargetID: "B", EventType: "LIKE"})
	// u2 likes A, C, D
	_ = fb.AddEvent(ctx, store.FeedbackEvent{UserID: "u2", TargetType: "perma", TargetID: "A", EventType: "LIKE"})
	_ = fb.AddEvent(ctx, store.FeedbackEvent{UserID: "u2", TargetType: "perma", TargetID: "C", EventType: "LIKE"})
	_ = fb.AddEvent(ctx, store.FeedbackEvent{UserID: "u2", TargetType: "perma", TargetID: "D", EventType: "LIKE"})
	// u3 likes B, D, E
	_ = fb.AddEvent(ctx, store.FeedbackEvent{UserID: "u3", TargetType: "perma", TargetID: "B", EventType: "LIKE"})
	_ = fb.AddEvent(ctx, store.FeedbackEvent{UserID: "u3", TargetType: "perma", TargetID: "D", EventType: "LIKE"})
	_ = fb.AddEvent(ctx, store.FeedbackEvent{UserID: "u3", TargetType: "perma", TargetID: "E", EventType: "LIKE"})

	svc := NewRecommenderService(fb, nil)
	recs, err := svc.Recommend(ctx, &domain.User{ID: "u1"}, RecommendInput{TargetType: "perma"})
	if err != nil {
		t.Fatalf("recommend: %v", err)
	}
	if len(recs) == 0 {
		t.Fatal("want recommendations")
	}
	// D 应该出现两次（u2 和 u3 都 like 了），最高分
	if recs[0].TargetID != "D" {
		t.Errorf("want top=D, got %s (all=%v)", recs[0].TargetID, recs)
	}
}

func TestRecommender_ColdStart(t *testing.T) {
	fb := newMemFeedback()
	svc := NewRecommenderService(fb, nil)
	recs, err := svc.Recommend(context.Background(), &domain.User{ID: "newbie"}, RecommendInput{TargetType: "perma"})
	if err != nil {
		t.Fatalf("recommend: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("cold start should return empty, got %d", len(recs))
	}
}

func TestRecommender_RequiresTargetType(t *testing.T) {
	svc := NewRecommenderService(newMemFeedback(), nil)
	_, err := svc.Recommend(context.Background(), &domain.User{ID: "u"}, RecommendInput{})
	if err == nil {
		t.Fatal("want error")
	}
}

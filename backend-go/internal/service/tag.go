package service

import (
	"context"
	"fmt"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
)

// TagService P13-C：节点标签服务。
type TagService struct {
	tags    store.TagRepository
	members store.MembershipRepository
	nodes   store.NodeRepository
}

// NewTagService 构造。
func NewTagService(tags store.TagRepository, members store.MembershipRepository, nodes store.NodeRepository) *TagService {
	return &TagService{tags: tags, members: members, nodes: nodes}
}

// SetNodeTags 覆盖写节点标签。actor 必须是该湖成员。
func (s *TagService) SetNodeTags(ctx context.Context, actor *domain.User, nodeID string, tags []string) error {
	// 验证节点存在并获取 lakeID
	node, err := s.nodes.GetByID(ctx, nodeID)
	if err != nil {
		return domain.ErrNotFound
	}
	// 验证 actor 是湖成员（任意角色）
	if _, err = s.members.GetRole(ctx, actor.ID, node.LakeID); err != nil {
		return domain.ErrPermissionDenied
	}
	// 校验标签格式
	for _, t := range tags {
		if !store.ValidTag(t) {
			return fmt.Errorf("%w: invalid tag %q", domain.ErrInvalidInput, t)
		}
	}
	if len(tags) > 20 {
		return fmt.Errorf("%w: too many tags, max 20 per node", domain.ErrInvalidInput)
	}
	return s.tags.SetTags(ctx, nodeID, node.LakeID, tags)
}

// GetNodeTags 返回节点标签列表。actor 必须是湖成员。
func (s *TagService) GetNodeTags(ctx context.Context, actor *domain.User, nodeID string) ([]string, error) {
	node, err := s.nodes.GetByID(ctx, nodeID)
	if err != nil {
		return nil, domain.ErrNotFound
	}
	if _, err = s.members.GetRole(ctx, actor.ID, node.LakeID); err != nil {
		return nil, domain.ErrPermissionDenied
	}
	return s.tags.GetTags(ctx, nodeID)
}

// ListLakeTags 返回湖内所有已使用标签。actor 必须是湖成员。
func (s *TagService) ListLakeTags(ctx context.Context, actor *domain.User, lakeID string) ([]string, error) {
	if _, err := s.members.GetRole(ctx, actor.ID, lakeID); err != nil {
		return nil, domain.ErrPermissionDenied
	}
	return s.tags.ListLakeTags(ctx, lakeID)
}

// ListNodesByTag 返回湖内带指定标签的节点 ID 列表。actor 必须是湖成员。
func (s *TagService) ListNodesByTag(ctx context.Context, actor *domain.User, lakeID, tag string) ([]string, error) {
	if _, err := s.members.GetRole(ctx, actor.ID, lakeID); err != nil {
		return nil, domain.ErrPermissionDenied
	}
	return s.tags.ListNodesByTag(ctx, lakeID, tag)
}

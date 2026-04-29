package httpapi

import (
	"context"
	"errors"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
)

func canAccessPromptTemplate(ctx context.Context, orgs *service.OrgService, userID string, tpl *domain.PromptTemplate) (bool, error) {
	if tpl == nil {
		return false, nil
	}
	switch tpl.Scope {
	case domain.PromptScopePrivate:
		return tpl.CreatedBy == userID, nil
	case domain.PromptScopeOrg:
		if tpl.OrgID == "" || orgs == nil {
			return false, nil
		}
		return orgs.IsMember(ctx, userID, tpl.OrgID)
	default:
		return false, nil
	}
}

func canManagePromptTemplate(ctx context.Context, orgs *service.OrgService, userID string, tpl *domain.PromptTemplate) (bool, error) {
	if tpl == nil {
		return false, nil
	}
	if tpl.CreatedBy == userID {
		return true, nil
	}
	if tpl.Scope != domain.PromptScopeOrg || tpl.OrgID == "" || orgs == nil {
		return false, nil
	}
	role, err := orgs.GetMemberRole(ctx, tpl.OrgID, userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return role.AtLeast(domain.OrgRoleAdmin), nil
}

func visiblePromptTemplateOrgIDs(ctx context.Context, orgs *service.OrgService, actor *domain.User) ([]string, error) {
	if orgs == nil {
		return []string{}, nil
	}
	items, err := orgs.ListMyOrgs(ctx, actor)
	if err != nil {
		return nil, err
	}
	orgIDs := make([]string, 0, len(items))
	for _, item := range items {
		if item.ID == "" {
			continue
		}
		orgIDs = append(orgIDs, item.ID)
	}
	return orgIDs, nil
}

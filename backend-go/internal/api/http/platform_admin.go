package httpapi

import (
	"context"
	"strings"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
)

func isPlatformAdminEmail(email string, adminEmails map[string]struct{}) bool {
	_, ok := adminEmails[strings.ToLower(strings.TrimSpace(email))]
	return ok
}

func isPlatformAdmin(ctx context.Context, actor *domain.User, adminEmails map[string]struct{}, platformAdmins store.PlatformAdminRepository) (bool, error) {
	_, ok, err := platformAdminRole(ctx, actor, adminEmails, platformAdmins)
	return ok, err
}

func isPlatformOwner(ctx context.Context, actor *domain.User, adminEmails map[string]struct{}, platformAdmins store.PlatformAdminRepository) (bool, error) {
	role, ok, err := platformAdminRole(ctx, actor, adminEmails, platformAdmins)
	if err != nil || !ok {
		return false, err
	}
	return role == domain.PlatformAdminRoleOwner, nil
}

func platformAdminRole(ctx context.Context, actor *domain.User, adminEmails map[string]struct{}, platformAdmins store.PlatformAdminRepository) (domain.PlatformAdminRole, bool, error) {
	if actor == nil {
		return "", false, nil
	}
	if _, ok := CurrentAPIKey(ctx); ok {
		return "", false, nil
	}
	if isPlatformAdminEmail(actor.Email, adminEmails) {
		return domain.PlatformAdminRoleOwner, true, nil
	}
	if platformAdmins == nil {
		return "", false, nil
	}
	admin, err := platformAdmins.GetActive(ctx, actor.ID)
	if err != nil {
		if err == domain.ErrNotFound {
			return "", false, nil
		}
		return "", false, err
	}
	return admin.Role, true, nil
}

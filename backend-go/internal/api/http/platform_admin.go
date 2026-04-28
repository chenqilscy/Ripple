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
	if actor == nil {
		return false, nil
	}
	if _, ok := CurrentAPIKey(ctx); ok {
		return false, nil
	}
	if isPlatformAdminEmail(actor.Email, adminEmails) {
		return true, nil
	}
	if platformAdmins == nil {
		return false, nil
	}
	return platformAdmins.IsActive(ctx, actor.ID)
}

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/platform"
)

// 内存版用户仓库（测试桩）。
type memUserRepo struct {
	byEmail map[string]*domain.User
	byID    map[string]*domain.User
}

type memGraylistRepo struct {
	allowed map[string]bool
}

func newMemUserRepo() *memUserRepo {
	return &memUserRepo{byEmail: map[string]*domain.User{}, byID: map[string]*domain.User{}}
}

func (r *memUserRepo) Create(_ context.Context, u *domain.User) error {
	if _, ok := r.byEmail[u.Email]; ok {
		return domain.ErrAlreadyExists
	}
	r.byEmail[u.Email] = u
	r.byID[u.ID] = u
	return nil
}
func (r *memUserRepo) GetByID(_ context.Context, id string) (*domain.User, error) {
	if u, ok := r.byID[id]; ok {
		return u, nil
	}
	return nil, domain.ErrNotFound
}
func (r *memUserRepo) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	if u, ok := r.byEmail[email]; ok {
		return u, nil
	}
	return nil, domain.ErrNotFound
}

func newAuthSvc() *AuthService {
	jwt := platform.NewJWTSigner("test-secret-32-chars-long-xxxxxx", time.Hour)
	return NewAuthService(newMemUserRepo(), jwt)
}

func (r *memGraylistRepo) List(context.Context, int) ([]domain.GraylistEntry, error) { return nil, nil }
func (r *memGraylistRepo) Upsert(_ context.Context, entry *domain.GraylistEntry) (*domain.GraylistEntry, error) {
	if r.allowed == nil {
		r.allowed = map[string]bool{}
	}
	r.allowed[entry.Email] = true
	copyEntry := *entry
	return &copyEntry, nil
}
func (r *memGraylistRepo) Delete(context.Context, string) error { return nil }
func (r *memGraylistRepo) IsAllowedEmail(_ context.Context, email string) (bool, error) {
	return r.allowed[email], nil
}

func TestAuth_RegisterAndLogin(t *testing.T) {
	ctx := context.Background()
	svc := newAuthSvc()

	u, err := svc.Register(ctx, RegisterInput{Email: "Alice@X.io", Password: "password-123", DisplayName: "Alice"})
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "alice@x.io" {
		t.Fatalf("email not normalized: %s", u.Email)
	}

	tok, lu, err := svc.Login(ctx, "alice@x.io", "password-123")
	if err != nil {
		t.Fatal(err)
	}
	if tok == "" || lu.ID != u.ID {
		t.Fatalf("login result mismatch")
	}

	mu, err := svc.VerifyToken(ctx, tok)
	if err != nil {
		t.Fatal(err)
	}
	if mu.ID != u.ID {
		t.Fatalf("verify user mismatch")
	}
}

func TestAuth_Register_DuplicateEmail(t *testing.T) {
	ctx := context.Background()
	svc := newAuthSvc()
	_, err := svc.Register(ctx, RegisterInput{Email: "a@b.c", Password: "password-123", DisplayName: "A"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Register(ctx, RegisterInput{Email: "a@b.c", Password: "password-123", DisplayName: "A"})
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestAuth_Login_WrongPassword(t *testing.T) {
	ctx := context.Background()
	svc := newAuthSvc()
	_, _ = svc.Register(ctx, RegisterInput{Email: "a@b.c", Password: "password-123", DisplayName: "A"})
	_, _, err := svc.Login(ctx, "a@b.c", "wrong-pwd")
	if !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied, got %v", err)
	}
}

func TestAuth_Login_UnknownUser(t *testing.T) {
	svc := newAuthSvc()
	_, _, err := svc.Login(context.Background(), "ghost@x.io", "any")
	if !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied, got %v", err)
	}
}

func TestAuth_Register_InvalidEmail(t *testing.T) {
	_, err := newAuthSvc().Register(context.Background(),
		RegisterInput{Email: "no-at-sign", Password: "password-123", DisplayName: "X"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestAuth_Register_GraylistRejectsUnknownEmail(t *testing.T) {
	svc := newAuthSvc().WithRegistrationGraylist(&memGraylistRepo{allowed: map[string]bool{}}, true, []string{"admin@test.local"})
	_, err := svc.Register(context.Background(), RegisterInput{Email: "user@test.local", Password: "password-123", DisplayName: "User"})
	if !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied, got %v", err)
	}
}

func TestAuth_Register_GraylistAllowsListedEmail(t *testing.T) {
	svc := newAuthSvc().WithRegistrationGraylist(&memGraylistRepo{allowed: map[string]bool{"user@test.local": true}}, true, []string{"admin@test.local"})
	u, err := svc.Register(context.Background(), RegisterInput{Email: "user@test.local", Password: "password-123", DisplayName: "User"})
	if err != nil {
		t.Fatalf("register listed email: %v", err)
	}
	if u.Email != "user@test.local" {
		t.Fatalf("unexpected email: %s", u.Email)
	}
}

func TestAuth_Register_GraylistAllowsAdminBypass(t *testing.T) {
	svc := newAuthSvc().WithRegistrationGraylist(&memGraylistRepo{allowed: map[string]bool{}}, true, []string{"admin@test.local"})
	u, err := svc.Register(context.Background(), RegisterInput{Email: "admin@test.local", Password: "password-123", DisplayName: "Admin"})
	if err != nil {
		t.Fatalf("register admin bypass: %v", err)
	}
	if u.Email != "admin@test.local" {
		t.Fatalf("unexpected email: %s", u.Email)
	}
}

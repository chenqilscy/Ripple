// Package service 是业务用例编排层。
// 不依赖 HTTP/WS，仅依赖 store 接口与 platform 工具。
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/platform"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
)

// AuthService 提供注册、登录、Token 校验。
type AuthService struct {
	users            store.UserRepository
	jwt              *platform.JWTSigner
	graylist         store.GraylistRepository
	graylistEnabled  bool
	adminEmailsByKey map[string]struct{}
}

// NewAuthService 装配。
func NewAuthService(users store.UserRepository, jwt *platform.JWTSigner) *AuthService {
	return &AuthService{users: users, jwt: jwt, adminEmailsByKey: map[string]struct{}{}}
}

// WithRegistrationGraylist 注入灰度名单仓库与平台管理员邮箱列表。
func (s *AuthService) WithRegistrationGraylist(repo store.GraylistRepository, enabled bool, adminEmails []string) *AuthService {
	s.graylist = repo
	s.graylistEnabled = enabled
	s.adminEmailsByKey = make(map[string]struct{}, len(adminEmails))
	for _, email := range adminEmails {
		normalized := strings.ToLower(strings.TrimSpace(email))
		if normalized != "" {
			s.adminEmailsByKey[normalized] = struct{}{}
		}
	}
	return s
}

// RegisterInput 注册入参。
type RegisterInput struct {
	Email       string
	Password    string
	DisplayName string
}

// Register 创建新用户。Email 唯一。
func (s *AuthService) Register(ctx context.Context, in RegisterInput) (*domain.User, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if email == "" || !strings.Contains(email, "@") {
		return nil, fmt.Errorf("%w: invalid email", domain.ErrInvalidInput)
	}
	if s.graylistEnabled {
		if _, ok := s.adminEmailsByKey[email]; !ok {
			if s.graylist == nil {
				return nil, fmt.Errorf("%w: registration graylist not configured", domain.ErrPermissionDenied)
			}
			allowed, err := s.graylist.IsAllowedEmail(ctx, email)
			if err != nil {
				return nil, err
			}
			if !allowed {
				return nil, fmt.Errorf("%w: registration email not in graylist", domain.ErrPermissionDenied)
			}
		}
	}
	if in.DisplayName == "" {
		return nil, fmt.Errorf("%w: display_name required", domain.ErrInvalidInput)
	}
	hash, err := platform.HashPassword(in.Password)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrInvalidInput, err)
	}
	now := time.Now().UTC()
	u := &domain.User{
		ID:           platform.NewID(),
		Email:        email,
		PasswordHash: hash,
		DisplayName:  in.DisplayName,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// Login 校验密码并签发 Token。
func (s *AuthService) Login(ctx context.Context, email, password string) (string, *domain.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return "", nil, domain.ErrPermissionDenied
		}
		return "", nil, err
	}
	if !u.IsActive {
		return "", nil, domain.ErrPermissionDenied
	}
	if !platform.VerifyPassword(u.PasswordHash, password) {
		return "", nil, domain.ErrPermissionDenied
	}
	tok, err := s.jwt.Sign(u.ID, u.Email)
	if err != nil {
		return "", nil, err
	}
	return tok, u, nil
}

// VerifyToken 解析并加载用户。
func (s *AuthService) VerifyToken(ctx context.Context, tokenStr string) (*domain.User, error) {
	c, err := s.jwt.Parse(tokenStr)
	if err != nil {
		return nil, domain.ErrPermissionDenied
	}
	u, err := s.users.GetByID(ctx, c.UserID)
	if err != nil {
		return nil, domain.ErrPermissionDenied
	}
	if !u.IsActive {
		return nil, domain.ErrPermissionDenied
	}
	return u, nil
}

// GetUserByID 直接按 ID 加载用户（P10-A API Key 鉴权使用）。
func (s *AuthService) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	return s.users.GetByID(ctx, id)
}

// Package httpapi 提供 HTTP 入口。
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
)

type ctxKey int

const (
	ctxUserKey   ctxKey = 1
	ctxAPIKeyKey ctxKey = 2 // *domain.APIKey，仅 ApiKey 鉴权时注入
)

// AuthMiddleware 强制 JWT 校验，注入 *domain.User 到 ctx。
//
// 优先 Authorization: Bearer <token> header。
// 浏览器 WebSocket 不支持自定义 header，兜底从 ?access_token= 取。
// 注意：query token 会出现在 access log 中，仅推荐用于 WS 升级路径。
func AuthMiddleware(auth *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok := ""
			if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
				tok = strings.TrimPrefix(h, "Bearer ")
			} else if qt := r.URL.Query().Get("access_token"); qt != "" {
				tok = qt
			}
			if tok == "" {
				writeError(w, http.StatusUnauthorized, "missing bearer token")
				return
			}
			u, err := auth.VerifyToken(r.Context(), tok)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid token")
				return
			}
			ctx := context.WithValue(r.Context(), ctxUserKey, u)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// CurrentUser 从 ctx 取出当前用户。
func CurrentUser(ctx context.Context) (*domain.User, bool) {
	u, ok := ctx.Value(ctxUserKey).(*domain.User)
	return u, ok
}

// CurrentAPIKey 从 ctx 取出当前 API Key（仅 ApiKey 鉴权时有值）。
func CurrentAPIKey(ctx context.Context) (*domain.APIKey, bool) {
	k, ok := ctx.Value(ctxAPIKeyKey).(*domain.APIKey)
	return k, ok
}

// CombinedAuthMiddleware 支持 Bearer JWT 和 ApiKey 两种鉴权方式。
// apiKeys 为 nil 时退化为纯 JWT 鉴权（向后兼容）。
func CombinedAuthMiddleware(auth *service.AuthService, apiKeys store.APIKeyRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hdr := r.Header.Get("Authorization")

			// ① ApiKey 鉴权
			if apiKeys != nil && strings.HasPrefix(hdr, "ApiKey ") {
				rawKey := strings.TrimPrefix(hdr, "ApiKey ")
				prefix, ok := store.ExtractPrefix(rawKey)
				if !ok {
					writeError(w, http.StatusUnauthorized, "invalid api key format")
					return
				}
				key, err := apiKeys.GetByPrefix(r.Context(), prefix)
				if err != nil || key == nil || !key.IsValid() {
					writeError(w, http.StatusUnauthorized, "invalid or expired api key")
					return
				}
				if !store.VerifyAPIKey(rawKey, key.KeySalt, key.KeyHash) {
					writeError(w, http.StatusUnauthorized, "invalid api key")
					return
				}
				// 异步更新 last_used_at，不阻塞响应
				go func() {
					ctx2, cancel := context.WithTimeout(context.Background(), 3*time.Second)
					defer cancel()
					_ = apiKeys.UpdateLastUsed(ctx2, key.ID, time.Now())
				}()
				// 加载 owner user
				u, err := auth.GetUserByID(r.Context(), key.OwnerID)
				if err != nil {
					writeError(w, http.StatusUnauthorized, "api key owner not found")
					return
				}
				ctx := context.WithValue(r.Context(), ctxUserKey, u)
				ctx = context.WithValue(ctx, ctxAPIKeyKey, key)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// ② Bearer JWT 鉴权（原逻辑）
			tok := ""
			if strings.HasPrefix(hdr, "Bearer ") {
				tok = strings.TrimPrefix(hdr, "Bearer ")
			} else if qt := r.URL.Query().Get("access_token"); qt != "" {
				tok = qt
			}
			if tok == "" {
				writeError(w, http.StatusUnauthorized, "missing bearer token")
				return
			}
			u, err := auth.VerifyToken(r.Context(), tok)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid token")
				return
			}
			ctx := context.WithValue(r.Context(), ctxUserKey, u)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// writeJSON 输出 JSON。
func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// writeError 统一错误输出。
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// mapDomainError 把领域错误映射到 HTTP 状态。
func mapDomainError(err error) int {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		return http.StatusBadRequest
	case errors.Is(err, domain.ErrAlreadyExists):
		return http.StatusConflict
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, domain.ErrPermissionDenied):
		return http.StatusForbidden
	case errors.Is(err, domain.ErrQuotaExceeded):
		return http.StatusTooManyRequests
	case errors.Is(err, domain.ErrInvalidStateTransition):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

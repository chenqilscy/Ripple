// Package httpapi 提供 HTTP 入口。
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
)

type ctxKey int

const ctxUserKey ctxKey = 1

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
	case errors.Is(err, domain.ErrInvalidStateTransition):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

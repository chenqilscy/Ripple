package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/platform"
)

// WsTokenHandlers 签发 ws-only 短期 Token（P7-B）。
//
//	POST /api/v1/ws_token
//	→ { "token": "<jwt purpose=ws>", "expires_in": 300 }
//
// 调用方须携带有效主 JWT（通过 AuthMiddleware 校验）。
// 安全约束：ws-only token（purpose="ws"）不得用于续期，防止无限续期攻击。
// 返回的 ws-only token 仅用于 yjs-bridge 的 WebSocket 连接，有效期 5 分钟。
type WsTokenHandlers struct {
	JWT *platform.JWTSigner
}

// Issue POST /api/v1/ws_token
func (h *WsTokenHandlers) Issue(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// P4安全：禁止 ws-only token 自我续期
	if raw := extractBearer(r); raw != "" {
		if c, err := h.JWT.Parse(raw); err == nil && c.Purpose == "ws" {
			writeError(w, http.StatusForbidden, "ws token cannot be used to issue another ws token")
			return
		}
	}

	const wsTTL = 5 * time.Minute
	tok, err := h.JWT.SignWithPurpose(u.ID, u.Email, "ws", wsTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to sign ws token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token":      tok,
		"expires_in": int(wsTTL.Seconds()),
	})
}

// extractBearer 从 Authorization 头取出 Bearer token 原文。
func extractBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}

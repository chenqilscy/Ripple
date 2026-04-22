package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// APIKeyHandlers P10-A API Key 管理端点。
//
//	POST   /api/v1/api_keys        创建
//	GET    /api/v1/api_keys        列出（owner 自己的）
//	DELETE /api/v1/api_keys/{id}   撤销
type APIKeyHandlers struct {
	Repo store.APIKeyRepository
}

// createAPIKeyReq 创建 API Key 请求体。
type createAPIKeyReq struct {
	Name          string   `json:"name"`
	Scopes        []string `json:"scopes"`
	ExpiresInDays int      `json:"expires_in_days"` // 0 = 永不过期
}

// Create POST /api/v1/api_keys
func (h *APIKeyHandlers) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req createAPIKeyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(req.Name) == 0 || len(req.Name) > 100 {
		writeError(w, http.StatusBadRequest, "name must be 1-100 chars")
		return
	}
	if len(req.Scopes) == 0 {
		req.Scopes = []string{"read_lake", "read_node"}
	}

	rawKey, prefix, salt, hash, err := store.GenerateAPIKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate key")
		return
	}

	key := &domain.APIKey{
		ID:        uuid.NewString(),
		OwnerID:   actor.ID,
		Name:      req.Name,
		KeyPrefix: prefix,
		KeyHash:   hash,
		KeySalt:   salt,
		Scopes:    req.Scopes,
		CreatedAt: time.Now().UTC(),
	}
	if req.ExpiresInDays > 0 {
		t := time.Now().UTC().Add(time.Duration(req.ExpiresInDays) * 24 * time.Hour)
		key.ExpiresAt = &t
	}

	if err := h.Repo.Create(r.Context(), key); err != nil {
		writeError(w, http.StatusInternalServerError, "create failed")
		return
	}

	// rawKey 只返回一次，后端不再可重建
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         key.ID,
		"name":       key.Name,
		"key":        rawKey, // only shown once
		"key_prefix": key.KeyPrefix,
		"scopes":     key.Scopes,
		"expires_at": key.ExpiresAt,
		"created_at": key.CreatedAt,
	})
}

// List GET /api/v1/api_keys
func (h *APIKeyHandlers) List(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	keys, err := h.Repo.ListByOwner(r.Context(), actor.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list failed")
		return
	}

	type keyView struct {
		ID          string     `json:"id"`
		Name        string     `json:"name"`
		KeyPrefix   string     `json:"key_prefix"`
		Scopes      []string   `json:"scopes"`
		LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
		ExpiresAt   *time.Time `json:"expires_at,omitempty"`
		CreatedAt   time.Time  `json:"created_at"`
	}
	out := make([]keyView, 0, len(keys))
	for _, k := range keys {
		out = append(out, keyView{
			ID:         k.ID,
			Name:       k.Name,
			KeyPrefix:  "rpl_" + k.KeyPrefix + "...", // 脱敏展示
			Scopes:     k.Scopes,
			LastUsedAt: k.LastUsedAt,
			ExpiresAt:  k.ExpiresAt,
			CreatedAt:  k.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"keys": out})
}

// Revoke DELETE /api/v1/api_keys/{id}
func (h *APIKeyHandlers) Revoke(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.Repo.Revoke(r.Context(), id, actor.ID); err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers (shared with other handlers in this file set)
// ─────────────────────────────────────────────────────────────────────────────

// parseIntQuery 从 query string 中解析整数，若缺失/无效返回 fallback。
func parseIntQuery(r *http.Request, key string, fallback int) int {
	if v := r.URL.Query().Get(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
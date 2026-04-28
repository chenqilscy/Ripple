package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/platform"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
	"github.com/go-chi/chi/v5"
)

// GraylistHandlers 管理灰度准入邮箱名单。
type GraylistHandlers struct {
	Repo       store.GraylistRepository
	AdminEmails map[string]struct{}
}

type graylistEntryResp struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Note      string    `json:"note"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

func toGraylistResp(entry domain.GraylistEntry) graylistEntryResp {
	return graylistEntryResp{
		ID:        entry.ID,
		Email:     entry.Email,
		Note:      entry.Note,
		CreatedBy: entry.CreatedBy,
		CreatedAt: entry.CreatedAt,
	}
}

// List GET /api/v1/admin/graylist
func (h *GraylistHandlers) List(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if _, ok := h.AdminEmails[strings.ToLower(strings.TrimSpace(actor.Email))]; !ok {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	entries, err := h.Repo.List(r.Context(), 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list failed")
		return
	}
	out := make([]graylistEntryResp, 0, len(entries))
	for _, entry := range entries {
		out = append(out, toGraylistResp(entry))
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": out})
}

// Upsert POST /api/v1/admin/graylist
func (h *GraylistHandlers) Upsert(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if _, ok := h.AdminEmails[strings.ToLower(strings.TrimSpace(actor.Email))]; !ok {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	var body struct {
		Email string `json:"email"`
		Note  string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))
	if email == "" || !strings.Contains(email, "@") {
		writeError(w, http.StatusBadRequest, "invalid email")
		return
	}
	entry, err := h.Repo.Upsert(r.Context(), &domain.GraylistEntry{
		ID:        platform.NewID(),
		Email:     email,
		Note:      strings.TrimSpace(body.Note),
		CreatedBy: actor.ID,
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "upsert failed")
		return
	}
	writeJSON(w, http.StatusOK, toGraylistResp(*entry))
}

// Delete DELETE /api/v1/admin/graylist/{id}
func (h *GraylistHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if _, ok := h.AdminEmails[strings.ToLower(strings.TrimSpace(actor.Email))]; !ok {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		writeError(w, http.StatusBadRequest, "id required")
		return
	}
	if err := h.Repo.Delete(r.Context(), id); err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
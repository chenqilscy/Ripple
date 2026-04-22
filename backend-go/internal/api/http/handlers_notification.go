package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/go-chi/chi/v5"
)

// NotificationHandlers P13-B：通知 HTTP 处理器。
type NotificationHandlers struct {
	Svc *service.NotificationService
}

type notifResp struct {
	ID        int64           `json:"id"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	IsRead    bool            `json:"is_read"`
	CreatedAt time.Time       `json:"created_at"`
}

// List GET /api/v1/notifications?limit=20&before=<id>
func (h *NotificationHandlers) List(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > 100 {
		limit = 100
	}
	var before int64
	if b := r.URL.Query().Get("before"); b != "" {
		if v, err := strconv.ParseInt(b, 10, 64); err == nil {
			before = v
		}
	}
	items, err := h.Svc.List(r.Context(), actor, limit, before)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]notifResp, 0, len(items))
	for _, it := range items {
		out = append(out, notifResp{
			ID: it.ID, Type: it.Type, Payload: it.Payload,
			IsRead: it.IsRead, CreatedAt: it.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"notifications": out})
}

// MarkRead POST /api/v1/notifications/{id}/read
func (h *NotificationHandlers) MarkRead(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid notification id")
		return
	}
	if err := h.Svc.MarkRead(r.Context(), actor, id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// MarkAllRead POST /api/v1/notifications/read_all
func (h *NotificationHandlers) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := h.Svc.MarkAllRead(r.Context(), actor); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UnreadCount GET /api/v1/notifications/unread_count
func (h *NotificationHandlers) UnreadCount(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	count, err := h.Svc.CountUnread(r.Context(), actor)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": count})
}

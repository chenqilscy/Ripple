package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/platform"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
	"github.com/go-chi/chi/v5"
)

// LakeSnapshotHandlers P18-D：图谱布局快照。
type LakeSnapshotHandlers struct {
	Repo        store.LakeSnapshotRepository
	Memberships store.MembershipRepository
}

type snapshotResp struct {
	ID         string           `json:"id"`
	LakeID     string           `json:"lake_id"`
	Name       string           `json:"name"`
	Layout     json.RawMessage  `json:"layout"`
	GraphState *json.RawMessage `json:"graph_state,omitempty"`
	CreatedBy  string           `json:"created_by"`
	CreatedAt  time.Time        `json:"created_at"`
}

const maxSnapshotLayoutBytes = 65536  // 64 KB
const maxSnapshotGraphStateBytes = 131072 // 128 KB

// CreateSnapshot POST /api/v1/lakes/{id}/snapshots
func (h *LakeSnapshotHandlers) CreateSnapshot(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	lakeID := chi.URLParam(r, "id")

	// Actor must be at least PASSENGER in the lake.
	if _, err := h.Memberships.GetRole(r.Context(), u.ID, lakeID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusForbidden, domain.ErrPermissionDenied.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxSnapshotLayoutBytes+maxSnapshotGraphStateBytes+4096)
	var req struct {
		Name       string          `json:"name"`
		Layout     json.RawMessage `json:"layout"`
		GraphState json.RawMessage `json:"graph_state,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	if len(req.Name) > 100 {
		writeError(w, http.StatusBadRequest, "name too long (max 100)")
		return
	}
	if len(req.Layout) == 0 {
		writeError(w, http.StatusBadRequest, "layout required")
		return
	}
	if len(req.Layout) > maxSnapshotLayoutBytes {
		writeError(w, http.StatusBadRequest, "layout too large (max 64KB)")
		return
	}
	// Validate graph_state if provided.
	if len(req.GraphState) > 0 && len(req.GraphState) > maxSnapshotGraphStateBytes {
		writeError(w, http.StatusBadRequest, "graph_state too large (max 128KB)")
		return
	}
	// Validate JSON is an object.
	var layoutCheck map[string]interface{}
	if err := json.Unmarshal(req.Layout, &layoutCheck); err != nil {
		writeError(w, http.StatusBadRequest, "layout must be a JSON object")
		return
	}

	now := time.Now().UTC()
	var graphState []byte
	if len(req.GraphState) > 0 {
		graphState = []byte(req.GraphState)
	}
	s := &domain.LakeSnapshot{
		ID:         platform.NewID(),
		LakeID:     lakeID,
		Name:       req.Name,
		Layout:     []byte(req.Layout),
		GraphState: graphState,
		CreatedBy:  u.ID,
		CreatedAt:  now,
	}
	if err := h.Repo.Create(r.Context(), s); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var gsResp *json.RawMessage
	if len(s.GraphState) > 0 {
		rm := json.RawMessage(s.GraphState)
		gsResp = &rm
	}
	writeJSON(w, http.StatusCreated, snapshotResp{
		ID: s.ID, LakeID: s.LakeID, Name: s.Name,
		Layout: json.RawMessage(s.Layout), GraphState: gsResp,
		CreatedBy: s.CreatedBy, CreatedAt: s.CreatedAt,
	})
}

// ListSnapshots GET /api/v1/lakes/{id}/snapshots
func (h *LakeSnapshotHandlers) ListSnapshots(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	lakeID := chi.URLParam(r, "id")

	if _, err := h.Memberships.GetRole(r.Context(), u.ID, lakeID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusForbidden, domain.ErrPermissionDenied.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	snapshots, err := h.Repo.List(r.Context(), lakeID, 20)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := make([]snapshotResp, len(snapshots))
	for i, s := range snapshots {
		resp[i] = snapshotResp{
			ID: s.ID, LakeID: s.LakeID, Name: s.Name,
			Layout: json.RawMessage(s.Layout), CreatedBy: s.CreatedBy, CreatedAt: s.CreatedAt,
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"snapshots": resp})
}

// DeleteSnapshot DELETE /api/v1/lakes/{lakeID}/snapshots/{snapshotID}
func (h *LakeSnapshotHandlers) DeleteSnapshot(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	lakeID := chi.URLParam(r, "id")
	snapshotID := chi.URLParam(r, "snapshotID")

	// Verify actor is a member of the lake before attempting delete.
	if _, err := h.Memberships.GetRole(r.Context(), u.ID, lakeID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusForbidden, domain.ErrPermissionDenied.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	if err := h.Repo.Delete(r.Context(), snapshotID, u.ID); err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetSnapshot GET /api/v1/lakes/{id}/snapshots/{snapshotID} — P21：返回含 graph_state 的完整快照
func (h *LakeSnapshotHandlers) GetSnapshot(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	lakeID := chi.URLParam(r, "id")
	snapshotID := chi.URLParam(r, "snapshotID")

	if _, err := h.Memberships.GetRole(r.Context(), u.ID, lakeID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusForbidden, domain.ErrPermissionDenied.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	s, err := h.Repo.Get(r.Context(), snapshotID)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	// 验证快照属于该湖泊
	if s.LakeID != lakeID {
		writeError(w, http.StatusForbidden, domain.ErrPermissionDenied.Error())
		return
	}
	var gsResp *json.RawMessage
	if len(s.GraphState) > 0 {
		rm := json.RawMessage(s.GraphState)
		gsResp = &rm
	}
	writeJSON(w, http.StatusOK, snapshotResp{
		ID: s.ID, LakeID: s.LakeID, Name: s.Name,
		Layout: json.RawMessage(s.Layout), GraphState: gsResp,
		CreatedBy: s.CreatedBy, CreatedAt: s.CreatedAt,
	})
}

package httpapi

import (
	"errors"
	"io"
	"net/http"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
	"github.com/go-chi/chi/v5"
)

const maxDocStateBytes = 1 << 20 // 1 MiB（P8 限制）

// DocStateHandlers Y.Doc 快照 REST 端点（P8-C）。
//
//	GET  /api/v1/nodes/{id}/doc_state  → 200 application/octet-stream | 404
//	PUT  /api/v1/nodes/{id}/doc_state  → 204 No Content（需 PASSENGER+）
type DocStateHandlers struct {
	Nodes     *service.NodeService
	DocStates store.NodeDocStateRepository
}

// GetDocState GET /api/v1/nodes/{id}/doc_state
// 权限：节点所属湖的可读成员（assertReadable：私有湖需成员，公开湖任何人）。
func (h *DocStateHandlers) GetDocState(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	nodeID := chi.URLParam(r, "id")

	// NodeService.Get 已包含 assertReadable 权限校验。
	if _, err := h.Nodes.Get(r.Context(), actor, nodeID); err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeError(w, http.StatusNotFound, "node not found")
		case errors.Is(err, domain.ErrPermissionDenied):
			writeError(w, http.StatusForbidden, "forbidden")
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	snap, err := h.DocStates.Get(r.Context(), nodeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if snap == nil {
		// 节点存在但尚无快照 → 204，客户端使用空 Y.Doc
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(snap.State)
}

// PutDocState PUT /api/v1/nodes/{id}/doc_state
// 权限：节点所属湖的 PASSENGER 以上（可写成员）。
// 请求体为裸 Y.Doc 二进制，限制 1 MiB。
func (h *DocStateHandlers) PutDocState(w http.ResponseWriter, r *http.Request) {
	actor, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	nodeID := chi.URLParam(r, "id")

	// 先查节点（同时验证节点存在，并获取 LakeID）。
	node, err := h.Nodes.Get(r.Context(), actor, nodeID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeError(w, http.StatusNotFound, "node not found")
		case errors.Is(err, domain.ErrPermissionDenied):
			writeError(w, http.StatusForbidden, "forbidden")
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	// 校验写权限（PASSENGER+）。
	// node.LakeID 为空时（云节点），由 NodeService.Get 内已校验 owner，直接拒绝协作写入。
	if node.LakeID == "" {
		writeError(w, http.StatusForbidden, "forbidden: node is not in a lake")
		return
	}
	if err := h.Nodes.RequireWrite(r.Context(), actor, node.LakeID); err != nil {
		writeError(w, http.StatusForbidden, "forbidden: need write access")
		return
	}

	// 限制读取大小（防止超大 Yjs 状态撑爆内存）。
	lr := io.LimitReader(r.Body, maxDocStateBytes+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}
	if len(data) > maxDocStateBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "doc state exceeds 1 MiB limit")
		return
	}
	if len(data) == 0 {
		writeError(w, http.StatusBadRequest, "empty body")
		return
	}

	if err := h.DocStates.Put(r.Context(), nodeID, data); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

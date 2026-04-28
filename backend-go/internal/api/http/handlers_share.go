package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/domain"
	"github.com/chenqilscy/ripple/backend-go/internal/platform"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
	"github.com/go-chi/chi/v5"
	"golang.org/x/time/rate"
)

// NodeShareHandlers P18-B：节点外链分享。
type NodeShareHandlers struct {
	Shares       store.NodeShareRepository
	Nodes        *service.NodeService
	// ShareBaseURL 若非空，用于生成分享 URL 时覆盖 r.Host，
	// 防止请求伪造 Host 头生成钓鱼链接。
	// 对应 RIPPLE_SHARE_BASE_URL 环境变量。
	ShareBaseURL string
}

const (
	shareTokenLength = 43
	maxShareTTLHours = 24 * 365
)

var defaultShareLimiter = newShareRateLimiter(2, 20, 10*time.Minute)

type shareRateLimiter struct {
	mu        sync.Mutex
	entries   map[string]*shareRateEntry
	limit     rate.Limit
	burst     int
	ttl       time.Duration
	lastSweep time.Time
	now       func() time.Time
}

type shareRateEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newShareRateLimiter(rps float64, burst int, ttl time.Duration) *shareRateLimiter {
	if rps <= 0 {
		rps = 1
	}
	if burst <= 0 {
		burst = 1
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &shareRateLimiter{
		entries: make(map[string]*shareRateEntry),
		limit:   rate.Limit(rps),
		burst:   burst,
		ttl:     ttl,
		now:     time.Now,
	}
}

func (l *shareRateLimiter) allow(key string) bool {
	if key == "" {
		key = "unknown"
	}
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.lastSweep.IsZero() || now.Sub(l.lastSweep) >= time.Minute {
		for k, e := range l.entries {
			if now.Sub(e.lastSeen) > l.ttl {
				delete(l.entries, k)
			}
		}
		l.lastSweep = now
	}

	entry := l.entries[key]
	if entry == nil {
		entry = &shareRateEntry{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.entries[key] = entry
	}
	entry.lastSeen = now
	return entry.limiter.AllowN(now, 1)
}

type shareResp struct {
	ID        string     `json:"id"`
	NodeID    string     `json:"node_id"`
	Token     string     `json:"token"`
	URL       string     `json:"url"`
	PageURL   string     `json:"page_url"`
	APIURL    string     `json:"api_url"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Revoked   bool       `json:"revoked"`
	CreatedAt time.Time  `json:"created_at"`
}

func toShareResp(s domain.NodeShare, baseURL string) shareResp {
	pageURL := baseURL + "/share/" + s.Token
	apiURL := baseURL + "/api/v1/share/" + s.Token
	resp := shareResp{
		ID:        s.ID,
		NodeID:    s.NodeID,
		Token:     s.Token,
		URL:       pageURL,
		PageURL:   pageURL,
		APIURL:    apiURL,
		Revoked:   s.Revoked,
		CreatedAt: s.CreatedAt,
	}
	if !s.ExpiresAt.IsZero() {
		resp.ExpiresAt = &s.ExpiresAt
	}
	return resp
}

// generateToken 生成 URL-safe base64 随机 token（32 字节 = 43 字符）。
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// CreateShare POST /api/v1/nodes/{id}/share
func (h *NodeShareHandlers) CreateShare(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	nodeID := chi.URLParam(r, "id")

	// Must be able to read the node (also validates it exists).
	node, err := h.Nodes.Get(r.Context(), u, nodeID)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}

	var req struct {
		TTLHours int `json:"ttl_hours"` // 0 = never expires
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.TTLHours < 0 || req.TTLHours > maxShareTTLHours {
		writeError(w, http.StatusBadRequest, "ttl_hours must be between 0 and 8760")
		return
	}

	token, err := generateToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token generation failed")
		return
	}

	now := time.Now().UTC()
	share := &domain.NodeShare{
		ID:        platform.NewID(),
		NodeID:    node.ID,
		Token:     token,
		CreatedBy: u.ID,
		CreatedAt: now,
	}
	if req.TTLHours > 0 {
		exp := now.Add(time.Duration(req.TTLHours) * time.Hour)
		share.ExpiresAt = exp
	}

	if err := h.Shares.Create(r.Context(), share); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toShareResp(*share, h.resolveBaseURL(r)))
}

// ListShares GET /api/v1/nodes/{id}/shares
func (h *NodeShareHandlers) ListShares(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	nodeID := chi.URLParam(r, "id")

	if _, err := h.Nodes.Get(r.Context(), u, nodeID); err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}

	shares, err := h.Shares.ListByNode(r.Context(), nodeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	base := h.resolveBaseURL(r)
	resp := make([]shareResp, len(shares))
	for i, s := range shares {
		resp[i] = toShareResp(s, base)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"shares": resp})
}

// RevokeShare DELETE /api/v1/shares/{id}
func (h *NodeShareHandlers) RevokeShare(w http.ResponseWriter, r *http.Request) {
	u, _ := CurrentUser(r.Context())
	shareID := chi.URLParam(r, "id")

	if err := h.Shares.Revoke(r.Context(), shareID, u.ID); err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetSharedNode GET /api/v1/share/{token} — 公开端点，无需鉴权。
func (h *NodeShareHandlers) GetSharedNode(w http.ResponseWriter, r *http.Request) {
	if !defaultShareLimiter.allow(clientIP(r)) {
		writeError(w, http.StatusTooManyRequests, "too many share requests")
		return
	}

	token := chi.URLParam(r, "token")
	if !isShareTokenFormat(token) {
		writeError(w, http.StatusNotFound, "share not found")
		return
	}

	share, err := h.Shares.GetByToken(r.Context(), token)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "share not found")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	if share.Revoked {
		writeError(w, http.StatusGone, "share has been revoked")
		return
	}
	if !share.ExpiresAt.IsZero() && time.Now().After(share.ExpiresAt) {
		writeError(w, http.StatusGone, "share has expired")
		return
	}

	// Retrieve node without auth check — this is a public share endpoint.
	// We call the repo directly via the node service's Get method with a synthetic
	// system user to bypass lake membership checks.
	// Since this is a public endpoint, we must NOT reveal private lake info —
	// only the node content is returned.
	type publicNodeResp struct {
		Node struct {
			ID        string    `json:"id"`
			Content   string    `json:"content"`
			Type      string    `json:"type"`
			State     string    `json:"state"`
			CreatedAt time.Time `json:"created_at"`
			UpdatedAt time.Time `json:"updated_at"`
		} `json:"node"`
		ShareID   string     `json:"share_id"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	// We use a "system" actor (empty string owner) bypass — the node service
	// Get() checks membership, which is inappropriate for public shares.
	// Instead, we call h.Nodes.GetPublic which is implemented below or we accept
	// that this handler has direct access via share token validation.
	// Since we already validated the share token, we expose node content directly
	// via a dedicated service method.
	node, err := h.Nodes.GetPublicByID(r.Context(), share.NodeID)
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	resp := publicNodeResp{ShareID: share.ID}
	resp.Node.ID = node.ID
	resp.Node.Content = node.Content
	resp.Node.Type = string(node.Type)
	resp.Node.State = string(node.State)
	resp.Node.CreatedAt = node.CreatedAt
	resp.Node.UpdatedAt = node.UpdatedAt
	if !share.ExpiresAt.IsZero() {
		exp := share.ExpiresAt
		resp.ExpiresAt = &exp
	}
	writeJSON(w, http.StatusOK, resp)
}

// resolveBaseURL 返回分享 URL 的 base：优先使用配置的 ShareBaseURL，
// 未配置时回退到从 r.Host 推导（仅用于开发环境）。
func (h *NodeShareHandlers) resolveBaseURL(r *http.Request) string {
	if h.ShareBaseURL != "" {
		return h.ShareBaseURL
	}
	return baseURLFromRequest(r)
}

// baseURLFromRequest 从 request 中推导服务 base URL。
// 当 handler 的 ShareBaseURL 字段已配置时应直接使用该值（不调用此函数），
// 以避免 Host 头伪造风险。退而求其次时才使用 r.Host。
func baseURLFromRequest(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func isShareTokenFormat(token string) bool {
	if len(token) != shareTokenLength {
		return false
	}
	for i := 0; i < len(token); i++ {
		c := token[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			continue
		}
		return false
	}
	return true
}

func clientIP(r *http.Request) string {
	// 优先读反向代理设置的 X-Real-IP（nginx proxy_set_header X-Real-IP $remote_addr）。
	// 注意：仅在可信反向代理后端环境使用；若直接暴露到公网须去掉此逻辑。
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

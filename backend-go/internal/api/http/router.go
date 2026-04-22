package httpapi

import (
	"net/http"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/llm"
	"github.com/chenqilscy/ripple/backend-go/internal/metrics"
	"github.com/chenqilscy/ripple/backend-go/internal/presence"
	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/chenqilscy/ripple/backend-go/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// Deps 路由所需依赖。
type Deps struct {
	Auth        *service.AuthService
	Lakes       *service.LakeService
	Nodes       *service.NodeService
	Edges       *service.EdgeService
	Invites     *service.InviteService
	Clouds      *service.CloudService
	Spaces      *service.SpaceService
	Crystallize *service.CrystallizeService
	Recommender *service.RecommenderService
	// Feedback 仓库（与 Recommender 配套；为 nil 则不挂载 /feedback /recommendations）
	Feedback    store.FeedbackRepository
	// Attachments M4-B：本地 FS 附件；为 nil 则不挂载 /attachments
	Attachments *AttachmentHandlers
	Presence    *presence.Service
	WS          *WSHandlers
	// WsToken 非 nil 时挂载 POST /ws_token（P7-B ws-only 短期 token）。
	WsToken     *WsTokenHandlers
	// DocStates 非 nil 时挂载 GET/PUT /nodes/{id}/doc_state（P8-C Y.Doc 快照）。
	DocStates   store.NodeDocStateRepository
	// APIKeys 非 nil 时挂载 /api_keys 端点并开启 ApiKey 鉴权（P10-A）。
	APIKeys     store.APIKeyRepository
	// AuditLogs 非 nil 时挂载 GET /audit_logs（P10-B）。
	AuditLogs   store.AuditLogRepository
	// LLMRouter 可选：提供则挂载 SSE 流式编织端点。
	LLMRouter   llm.StreamProvider
	CORSOrigins []string
	// MetricsEnabled true 时挂载 GET /metrics（Prometheus 文本格式）；false 不暴露。
	MetricsEnabled bool
}

// NewRouter 装配 Chi 路由。
func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(metricsMiddleware)

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   d.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Prometheus 文本格式指标。受 cfg.MetricsEnabled 开关控制。
	if d.MetricsEnabled {
		r.Get("/metrics", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain; version=0.0.4")
			_ = metrics.Default.WriteText(w)
		})
	}

	authH := &AuthHandlers{Auth: d.Auth}
	lakeH := &LakeHandlers{Lakes: d.Lakes, Spaces: d.Spaces}
	nodeH := &NodeHandlers{Nodes: d.Nodes}
	cloudH := &CloudHandlers{Clouds: d.Clouds}
	var edgeH *EdgeHandlers
	if d.Edges != nil {
		edgeH = &EdgeHandlers{Edges: d.Edges}
	}
	var inviteH *InviteHandlers
	if d.Invites != nil {
		inviteH = &InviteHandlers{Invites: d.Invites}
	}
	var presenceH *PresenceHandlers
	if d.Presence != nil {
		presenceH = &PresenceHandlers{Presence: d.Presence, Lakes: d.Lakes}
	}
	var spaceH *SpaceHandlers
	if d.Spaces != nil {
		spaceH = &SpaceHandlers{Spaces: d.Spaces}
	}
	var crysH *CrystallizeHandlers
	if d.Crystallize != nil {
		crysH = &CrystallizeHandlers{Svc: d.Crystallize}
	}

	r.Route("/api/v1", func(r chi.Router) {
		// 公开端点
		r.Post("/auth/register", authH.Register)
		r.Post("/auth/login", authH.Login)

		// 需鉴权（JWT 或 ApiKey 二选一）
		r.Group(func(r chi.Router) {
			r.Use(CombinedAuthMiddleware(d.Auth, d.APIKeys))
			r.Get("/auth/me", authH.Me)

			r.Post("/lakes", lakeH.Create)
			r.Get("/lakes", lakeH.ListMine)
			r.Get("/lakes/{id}", lakeH.Get)
			r.Patch("/lakes/{id}/space", lakeH.Move)
			r.Get("/lakes/{id}/nodes", nodeH.ListByLake)

			r.Post("/nodes", nodeH.Create)
			r.Get("/nodes/{id}", nodeH.Get)
			r.Post("/nodes/{id}/evaporate", nodeH.Evaporate)
			r.Post("/nodes/{id}/restore", nodeH.Restore)
			r.Post("/nodes/{id}/condense", nodeH.Condense)
			r.Put("/nodes/{id}/content", nodeH.UpdateContent)
			r.Get("/nodes/{id}/revisions", nodeH.ListRevisions)
			r.Get("/nodes/{id}/revisions/{rev}", nodeH.GetRevision)
			r.Post("/nodes/{id}/rollback", nodeH.Rollback)

			if d.Clouds != nil {
				r.Post("/clouds", cloudH.Create)
				r.Get("/clouds", cloudH.ListMine)
				r.Get("/clouds/{id}", cloudH.Get)
			}

			if edgeH != nil {
				r.Post("/edges", edgeH.Create)
				r.Delete("/edges/{id}", edgeH.Delete)
				r.Get("/lakes/{id}/edges", edgeH.ListByLake)
			}

			if inviteH != nil {
				r.Post("/lakes/{id}/invites", inviteH.Create)
				r.Get("/lakes/{id}/invites", inviteH.ListByLake)
				r.Delete("/lake-invites/{id}", inviteH.Revoke)
				r.Get("/invites/preview", inviteH.Preview)
				r.Post("/invites/accept", inviteH.Accept)
			}

			if presenceH != nil {
				r.Get("/lakes/{id}/presence", presenceH.List)
			}

			if spaceH != nil {
				r.Post("/spaces", spaceH.Create)
				r.Get("/spaces", spaceH.ListMine)
				r.Get("/spaces/{id}", spaceH.Get)
				r.Patch("/spaces/{id}", spaceH.Update)
				r.Delete("/spaces/{id}", spaceH.Delete)
				r.Get("/spaces/{id}/members", spaceH.ListMembers)
				r.Post("/spaces/{id}/members", spaceH.AddMember)
				r.Delete("/spaces/{id}/members/{userID}", spaceH.RemoveMember)
			}

			if crysH != nil {
				r.Post("/perma_nodes", crysH.Create)
			}

			if d.LLMRouter != nil {
				weaveH := &WeaveStreamHandlers{Lakes: d.Lakes, Router: d.LLMRouter}
				r.Get("/lakes/{id}/weave/stream", weaveH.Stream)
			}

			if d.Recommender != nil && d.Feedback != nil {
				rh := &RecommenderHandlers{Svc: d.Recommender, Feedback: d.Feedback}
				r.Get("/recommendations", rh.Recommend)
				r.Post("/feedback", rh.AddFeedback)
			}

			if d.Attachments != nil {
				r.Post("/attachments", d.Attachments.Upload)
				r.Get("/attachments/{id}", d.Attachments.Download)
			}

			if d.WS != nil {
				r.Get("/lakes/{id}/ws", d.WS.LakeWS)
			}

			// P7-B：ws-only 短期 token 签发
			if d.WsToken != nil {
				r.Post("/ws_token", d.WsToken.Issue)
			}

			// P8-C：Y.Doc 快照读写
			if d.DocStates != nil {
				docStateH := &DocStateHandlers{Nodes: d.Nodes, DocStates: d.DocStates}
				r.Get("/nodes/{id}/doc_state", docStateH.GetDocState)
				r.Put("/nodes/{id}/doc_state", docStateH.PutDocState)
			}

			// P10-A：API Key 管理
			if d.APIKeys != nil {
				apiKeyH := &APIKeyHandlers{Repo: d.APIKeys}
				r.Post("/api_keys", apiKeyH.Create)
				r.Get("/api_keys", apiKeyH.List)
				r.Delete("/api_keys/{id}", apiKeyH.Revoke)
			}

			// P10-B：审计日志查询
			if d.AuditLogs != nil {
				auditH := &AuditLogHandlers{Repo: d.AuditLogs}
				r.Get("/audit_logs", auditH.List)
			}

			// P10-C：湖成员角色变更
			r.Put("/lakes/{id}/members/{userID}/role", lakeH.UpdateMemberRole)
			// P11-C：湖成员列表（VIEWER+）
			r.Get("/lakes/{id}/members", lakeH.ListMembers)
		})
	})

	return r
}

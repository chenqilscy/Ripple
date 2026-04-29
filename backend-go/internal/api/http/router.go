package httpapi

import (
	"net/http"
	"strings"
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
	Feedback store.FeedbackRepository
	// Attachments M4-B：本地 FS 附件；为 nil 则不挂载 /attachments
	Attachments *AttachmentHandlers
	Presence    *presence.Service
	WS          *WSHandlers
	// WsToken 非 nil 时挂载 POST /ws_token（P7-B ws-only 短期 token）。
	WsToken *WsTokenHandlers
	// DocStates 非 nil 时挂载 GET/PUT /nodes/{id}/doc_state（P8-C Y.Doc 快照）。
	DocStates store.NodeDocStateRepository
	// APIKeys 非 nil 时挂载 /api_keys 端点并开启 ApiKey 鉴权（P10-A）。
	APIKeys store.APIKeyRepository
	// NodeCounts P14.3：组织运营视图需要节点用量聚合。
	NodeCounts store.NodeRepository
	// AuditLogs 非 nil 时挂载 GET /audit_logs（P10-B）。
	AuditLogs store.AuditLogRepository
	// Graylist 非 nil 时挂载 /admin/graylist（P14.3 灰度名单入口）。
	Graylist store.GraylistRepository
	// AdminEmails 平台管理员邮箱名单。
	AdminEmails []string
	// OrgRepo / OrgQuotas 用于平台管理员总览（P14.3）。
	OrgRepo        store.OrgRepository
	OrgQuotas      store.OrgQuotaRepository
	PlatformAdmins store.PlatformAdminRepository
	// Orgs 非 nil 时挂载 /organizations 端点（P12-C）。
	Orgs *service.OrgService
	// Notifications 非 nil 时挂载 /notifications 端点（P13-B）。
	Notifications *service.NotificationService
	// Tags 非 nil 时挂载节点标签端点（P13-C）。
	Tags *service.TagService
	// LLMRouter 可选：提供则挂载 SSE 流式编织端点。
	LLMRouter   llm.StreamProvider
	CORSOrigins []string
	// MetricsEnabled true 时挂载 GET /metrics（Prometheus 文本格式）；false 不暴露。
	MetricsEnabled bool
	// P18-C：节点模板库；非 nil 时挂载 /templates 端点。
	NodeTemplates store.NodeTemplateRepository
	// P18-D：图谱快照；非 nil 时挂载 /lakes/{id}/snapshots 端点。
	LakeSnapshots store.LakeSnapshotRepository
	// P18-B：节点外链分享；非 nil 时挂载 /nodes/{id}/share 端点。
	NodeShares store.NodeShareRepository
	// ShareBaseURL 若配置则作为分享链接 base（防 Host 伪造）；对应 RIPPLE_SHARE_BASE_URL。
	ShareBaseURL string
	// Memberships 用于快照权限校验；与 Lakes 配套注入。
	Memberships store.MembershipRepository
	// Users 用于按 email 查找用户（P12-C Org by_email 邀请）。
	Users store.UserRepository

	// Phase 15-C: AI 节点填充任务
	// AiJobs 非 nil 时挂载 /nodes/{id}/ai_trigger 及 /nodes/{id}/ai_status 端点。
	AiJobs          store.AiJobRepository
	PromptTemplates store.PromptTemplateRepository

	// Phase 15-D: 订阅 & LLM 用量
	// Subscriptions 非 nil 时挂载套餐订阅端点。
	Subscriptions *service.SubscriptionService
	// LLMUsage 非 nil 时挂载 /organizations/{id}/llm_usage 端点。
	LLMUsage *service.LLMUsageService
	// StubPaymentEnabled 透传 config.StubPaymentEnabled 给 SubscriptionHandlers。
	StubPaymentEnabled bool
}

// NewRouter 装配 Chi 路由。
func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	adminEmails := adminEmailSet(d.AdminEmails)

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
	lakeH := &LakeHandlers{Lakes: d.Lakes, Spaces: d.Spaces, Orgs: d.Orgs, Nodes: d.Nodes, Edges: d.Edges}
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
			r.Post("/lakes/{id}/nodes/batch", nodeH.BatchImport)
			r.Post("/lakes/{id}/nodes/batch_op", nodeH.BatchOperate) // P14-C

			r.Post("/nodes", nodeH.Create)
			r.Get("/nodes/{id}", nodeH.Get)
			r.Post("/nodes/{id}/evaporate", nodeH.Evaporate)
			r.Post("/nodes/{id}/restore", nodeH.Restore)
			r.Post("/nodes/{id}/condense", nodeH.Condense)
			r.Put("/nodes/{id}/content", nodeH.UpdateContent)
			r.Get("/nodes/{id}/revisions", nodeH.ListRevisions)
			r.Get("/nodes/{id}/revisions/{rev}", nodeH.GetRevision)
			r.Post("/nodes/{id}/rollback", nodeH.Rollback)
			// P16-B: AI 节点摘要
			if d.LLMRouter != nil {
				aiH := &NodeAIHandlers{Nodes: d.Nodes, Router: d.LLMRouter}
				r.Post("/nodes/{id}/ai_summary", aiH.AISummary)
			}

			r.Get("/search", nodeH.Search)
			// P20-C: 语义搜索增强（mode=semantic，有 LLM Router 时启用，无时等同全文搜索）
			{
				var ssRouter llm.Router
				if d.LLMRouter != nil {
					if r2, ok := d.LLMRouter.(llm.Router); ok {
						ssRouter = r2
					}
				}
				semanticH := &SemanticSearchHandlers{Nodes: d.Nodes, Router: ssRouter}
				r.Get("/semantic-search", semanticH.Search)
			}

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
				apiKeyH := &APIKeyHandlers{Repo: d.APIKeys, Orgs: d.Orgs}
				r.Post("/api_keys", apiKeyH.Create)
				r.Get("/api_keys", apiKeyH.List)
				r.Delete("/api_keys/{id}", apiKeyH.Revoke)
			}

			// P10-B：审计日志查询
			if d.AuditLogs != nil {
				auditH := &AuditLogHandlers{Repo: d.AuditLogs, Lakes: d.Lakes, Nodes: d.Nodes, Orgs: d.Orgs, PlatformAdmins: d.PlatformAdmins, AdminEmails: adminEmails}
				r.Get("/audit_logs", auditH.List)
			}
			if d.OrgRepo != nil && d.OrgQuotas != nil && hasPlatformAdminSource(adminEmails, d.PlatformAdmins) {
				adminH := &AdminHandlers{
					OrgRepo:        d.OrgRepo,
					Quotas:         d.OrgQuotas,
					Lakes:          d.Lakes,
					Users:          d.Users,
					AuditLogs:      d.AuditLogs,
					Graylist:       d.Graylist,
					PlatformAdmins: d.PlatformAdmins,
					AdminEmails:    adminEmails,
				}
				if counter, ok := d.NodeCounts.(orgNodeCounter); ok {
					adminH.Nodes = counter
				}
				if counter, ok := d.APIKeys.(apiKeyOrgCounter); ok {
					adminH.APIKeys = counter
				}
				if d.Attachments != nil {
					if usage, ok := d.Attachments.Repo.(orgAttachmentUsageReader); ok {
						adminH.Attachments = usage
					}
				}
				r.Get("/admin/overview", adminH.Overview)
			}
			if d.Graylist != nil && hasPlatformAdminSource(adminEmails, d.PlatformAdmins) {
				graylistH := &GraylistHandlers{Repo: d.Graylist, AuditLogs: d.AuditLogs, PlatformAdmins: d.PlatformAdmins, AdminEmails: adminEmails}
				r.Get("/admin/graylist", graylistH.List)
				r.Post("/admin/graylist", graylistH.Upsert)
				r.Delete("/admin/graylist/{id}", graylistH.Delete)
			}
			if d.PlatformAdmins != nil && hasPlatformAdminSource(adminEmails, d.PlatformAdmins) {
				platformAdminH := &PlatformAdminHandlers{Repo: d.PlatformAdmins, Users: d.Users, AuditLogs: d.AuditLogs, AdminEmails: adminEmails}
				r.Get("/admin/platform_admins", platformAdminH.List)
				r.Post("/admin/platform_admins", platformAdminH.Grant)
				r.Delete("/admin/platform_admins/{user_id}", platformAdminH.Revoke)
			}

			// P10-C：湖成员角色变更
			r.Put("/lakes/{id}/members/{userID}/role", lakeH.UpdateMemberRole)
			// P11-C：湖成员列表（VIEWER+）
			r.Get("/lakes/{id}/members", lakeH.ListMembers)
			// P16-C：移除湖成员（OWNER only）
			r.Delete("/lakes/{id}/members/{userID}", lakeH.RemoveMember)

			// P12-C：组织
			if d.Orgs != nil {
				orgH := &OrgHandlers{Orgs: d.Orgs, Lakes: d.Lakes, Users: d.Users, AuditLogs: d.AuditLogs}
				if counter, ok := d.NodeCounts.(orgNodeCounter); ok {
					orgH.Nodes = counter
				}
				if counter, ok := d.APIKeys.(apiKeyOrgCounter); ok {
					orgH.APIKeys = counter
				}
				if d.Attachments != nil {
					if usage, ok := d.Attachments.Repo.(orgAttachmentUsageReader); ok {
						orgH.Attachments = usage
					}
				}
				r.Post("/organizations", orgH.CreateOrg)
				r.Get("/organizations", orgH.ListMyOrgs)
				r.Get("/organizations/overview", orgH.ListOrgOverviews)
				r.Get("/organizations/{id}", orgH.GetOrg)
				r.Get("/organizations/{id}/overview", orgH.GetOrgOverview)
				r.Get("/organizations/{id}/quota", orgH.GetOrgQuota)
				r.Patch("/organizations/{id}/quota", orgH.UpdateOrgQuota)
				r.Get("/organizations/{id}/members", orgH.ListMembers)
				r.Post("/organizations/{id}/members", orgH.AddMember)
				r.Post("/organizations/{id}/members/by_email", orgH.AddMemberByEmail)
				r.Patch("/organizations/{id}/members/{userId}/role", orgH.UpdateMemberRole)
				r.Delete("/organizations/{id}/members/{userId}", orgH.RemoveMember)
				// P13-A：组织下的湖
				r.Get("/organizations/{id}/lakes", orgH.ListOrgLakes)
			}
			// P13-A：设置湖组织归属
			r.Patch("/lakes/{id}/org", lakeH.SetLakeOrg)
			// P13-D：内容导出；P13-E：内容导入
			r.Get("/lakes/{id}/export", lakeH.Export)
			r.Post("/lakes/{id}/import", lakeH.Import)

			// P13-B：通知系统
			if d.Notifications != nil {
				notifH := &NotificationHandlers{Svc: d.Notifications}
				r.Get("/notifications", notifH.List)
				r.Get("/notifications/unread_count", notifH.UnreadCount)
				r.Post("/notifications/{id}/read", notifH.MarkRead)
				r.Post("/notifications/read_all", notifH.MarkAllRead)
			}

			// P13-C：节点标签系统
			if d.Tags != nil {
				tagH := &TagHandlers{Svc: d.Tags}
				r.Get("/lakes/{id}/tags", tagH.ListLakeTags)
				r.Get("/lakes/{id}/nodes/by_tag", tagH.ListNodesByTag)
				r.Get("/nodes/{id}/tags", tagH.GetNodeTags)
				r.Put("/nodes/{id}/tags", tagH.SetNodeTags)
			}

			// P19-A：AI 图谱探索
			if d.LLMRouter != nil {
				if exploreRouter, ok := d.LLMRouter.(llm.Router); ok {
					exploreH := &ExploreHandlers{Nodes: d.Nodes, Router: exploreRouter}
					r.Post("/lakes/{id}/explore", exploreH.Explore)
				}
			}

			// P20-A：自由文本一键转图谱（Paste-to-Graph）
			// P20-B：图谱节点智能摘要（Summarize-Graph）
			if d.LLMRouter != nil {
				if importRouter, ok := d.LLMRouter.(llm.Router); ok {
					importTextH := &ImportTextHandlers{
						Nodes:  d.Nodes,
						Edges:  d.Edges,
						Router: importRouter,
					}
					r.Post("/lakes/{id}/import/text", importTextH.ImportText)

					summarizeH := &SummarizeGraphHandlers{
						NodeGetter:  d.Nodes,
						NodeCreator: d.Nodes,
						EdgeCreator: d.Edges,
						Router:      importRouter,
					}
					r.Post("/lakes/{id}/nodes/summarize", summarizeH.SummarizeGraph)
				}
			}

			// P18-A：节点关联推荐
			r.Get("/nodes/{id}/related", nodeH.GetRelated)

			// P18-C：节点模板库
			if d.NodeTemplates != nil {
				tplH := &NodeTemplateHandlers{Repo: d.NodeTemplates, Nodes: d.Nodes}
				r.Get("/templates", tplH.ListTemplates)
				r.Post("/templates", tplH.CreateTemplate)
				r.Delete("/templates/{id}", tplH.DeleteTemplate)
				r.Post("/lakes/{id}/nodes/from_template", tplH.CreateNodeFromTemplate)
			}

			// P18-D：图谱快照
			if d.LakeSnapshots != nil && d.Memberships != nil {
				snapH := &LakeSnapshotHandlers{Repo: d.LakeSnapshots, Memberships: d.Memberships}
				r.Post("/lakes/{id}/snapshots", snapH.CreateSnapshot)
				r.Get("/lakes/{id}/snapshots", snapH.ListSnapshots)
				r.Get("/lakes/{id}/snapshots/{snapshotID}", snapH.GetSnapshot)
				r.Delete("/lakes/{id}/snapshots/{snapshotID}", snapH.DeleteSnapshot)
			}

			// P18-B：节点外链分享（需鉴权端点）
			if d.NodeShares != nil {
				shareH := &NodeShareHandlers{Shares: d.NodeShares, Nodes: d.Nodes, ShareBaseURL: d.ShareBaseURL}
				r.Post("/nodes/{id}/share", shareH.CreateShare)
				r.Get("/nodes/{id}/shares", shareH.ListShares)
				r.Delete("/shares/{id}", shareH.RevokeShare)
			}

			// Phase 15-C: AI 节点填充任务 + Prompt 模板库
			if d.AiJobs != nil && d.PromptTemplates != nil {
				aiTriggerH := &AiTriggerHandlers{
					Jobs:            d.AiJobs,
					Nodes:           d.Nodes,
					Memberships:     d.Memberships,
					PromptTemplates: d.PromptTemplates,
					Orgs:            d.Orgs,
				}
				r.Post("/lakes/{lake_id}/nodes/{node_id}/ai_trigger", aiTriggerH.Trigger)
				r.Get("/lakes/{lake_id}/nodes/{node_id}/ai_status", aiTriggerH.Status)

				promptTplH := &PromptTemplateHandlers{Repo: d.PromptTemplates, Orgs: d.Orgs}
				r.Post("/prompt_templates", promptTplH.Create)
				r.Get("/prompt_templates", promptTplH.List)
				r.Get("/prompt_templates/{id}", promptTplH.Get)
				r.Patch("/prompt_templates/{id}", promptTplH.Update)
				r.Delete("/prompt_templates/{id}", promptTplH.Delete)
			}

			// Phase 15-D: 套餐订阅
			if d.Subscriptions != nil {
				subH := &SubscriptionHandlers{Svc: d.Subscriptions, Orgs: d.Orgs, StubPaymentEnabled: d.StubPaymentEnabled}
				r.Get("/subscriptions/plans", subH.GetPlans)
				r.Get("/organizations/{id}/subscription", subH.GetSubscription)
				r.Post("/organizations/{id}/subscription", subH.CreateSubscription)
				r.Get("/organizations/{id}/usage", subH.GetOrgUsage) // Phase 16
			}

			// Phase 15-D: LLM 用量
			if d.LLMUsage != nil {
				usageH := &LLMUsageHandlers{Svc: d.LLMUsage, Orgs: d.Orgs}
				r.Get("/organizations/{id}/llm_usage", usageH.GetUsage)
			}
		})

		// P18-B：公开分享端点（无需鉴权）
		if d.NodeShares != nil {
			shareH := &NodeShareHandlers{Shares: d.NodeShares, Nodes: d.Nodes, ShareBaseURL: d.ShareBaseURL}
			r.Get("/share/{token}", shareH.GetSharedNode)
		}
	})

	return r
}

func adminEmailSet(emails []string) map[string]struct{} {
	out := make(map[string]struct{}, len(emails))
	for _, email := range emails {
		key := strings.ToLower(strings.TrimSpace(email))
		if key == "" {
			continue
		}
		out[key] = struct{}{}
	}
	return out
}

func hasPlatformAdminSource(adminEmails map[string]struct{}, platformAdmins store.PlatformAdminRepository) bool {
	return len(adminEmails) > 0 || platformAdmins != nil
}

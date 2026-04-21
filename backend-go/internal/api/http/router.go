package httpapi

import (
	"net/http"
	"time"

	"github.com/chenqilscy/ripple/backend-go/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// Deps 路由所需依赖。
type Deps struct {
	Auth        *service.AuthService
	Lakes       *service.LakeService
	Nodes       *service.NodeService
	WS          *WSHandlers
	CORSOrigins []string
}

// NewRouter 装配 Chi 路由。
func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

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

	authH := &AuthHandlers{Auth: d.Auth}
	lakeH := &LakeHandlers{Lakes: d.Lakes}
	nodeH := &NodeHandlers{Nodes: d.Nodes}

	r.Route("/api/v1", func(r chi.Router) {
		// 公开端点
		r.Post("/auth/register", authH.Register)
		r.Post("/auth/login", authH.Login)

		// 需鉴权
		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware(d.Auth))
			r.Get("/auth/me", authH.Me)

			r.Post("/lakes", lakeH.Create)
			r.Get("/lakes", lakeH.ListMine)
			r.Get("/lakes/{id}", lakeH.Get)
			r.Get("/lakes/{id}/nodes", nodeH.ListByLake)

			r.Post("/nodes", nodeH.Create)
			r.Get("/nodes/{id}", nodeH.Get)
			r.Post("/nodes/{id}/evaporate", nodeH.Evaporate)
			r.Post("/nodes/{id}/restore", nodeH.Restore)

			if d.WS != nil {
				r.Get("/lakes/{id}/ws", d.WS.LakeWS)
			}
		})
	})

	return r
}

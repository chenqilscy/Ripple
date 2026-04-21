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
	Auth         *service.AuthService
	CORSOrigins  []string
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

	r.Route("/api/v1", func(r chi.Router) {
		// 公开端点
		r.Post("/auth/register", authH.Register)
		r.Post("/auth/login", authH.Login)

		// 需鉴权
		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware(d.Auth))
			r.Get("/auth/me", authH.Me)
		})
	})

	return r
}

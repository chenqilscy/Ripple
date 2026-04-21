package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/chenqilscy/ripple/backend-go/internal/service"
)

// AuthHandlers 注册/登录/我。
type AuthHandlers struct {
	Auth *service.AuthService
}

type registerReq struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type userResp struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResp struct {
	AccessToken string   `json:"access_token"`
	TokenType   string   `json:"token_type"`
	User        userResp `json:"user"`
}

// Register POST /api/v1/auth/register
func (h *AuthHandlers) Register(w http.ResponseWriter, r *http.Request) {
	var in registerReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	u, err := h.Auth.Register(r.Context(), service.RegisterInput{
		Email: in.Email, Password: in.Password, DisplayName: in.DisplayName,
	})
	if err != nil {
		writeError(w, mapDomainError(err), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, userResp{ID: u.ID, Email: u.Email, DisplayName: u.DisplayName})
}

// Login POST /api/v1/auth/login
func (h *AuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
	var in loginReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	tok, u, err := h.Auth.Login(r.Context(), in.Email, in.Password)
	if err != nil {
		writeError(w, mapDomainError(err), "login failed")
		return
	}
	writeJSON(w, http.StatusOK, loginResp{
		AccessToken: tok,
		TokenType:   "bearer",
		User:        userResp{ID: u.ID, Email: u.Email, DisplayName: u.DisplayName},
	})
}

// Me GET /api/v1/auth/me
func (h *AuthHandlers) Me(w http.ResponseWriter, r *http.Request) {
	u, ok := CurrentUser(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no user in context")
		return
	}
	writeJSON(w, http.StatusOK, userResp{ID: u.ID, Email: u.Email, DisplayName: u.DisplayName})
}

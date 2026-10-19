package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/Aabyss-Team/ARL/internal/domain"
	"github.com/Aabyss-Team/ARL/internal/platform/httpx"
	"github.com/Aabyss-Team/ARL/internal/platform/pdtools"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) setAuthCookies(w http.ResponseWriter, access, refresh, csrf string, accessExp, refreshExp time.Time) {
	secure := s.Config.Auth.CookieSecure
	http.SetCookie(w, &http.Cookie{
		Name: "arl_access", Value: access, Path: "/", HttpOnly: true, Secure: secure,
		SameSite: http.SameSiteLaxMode, Expires: accessExp,
	})
	http.SetCookie(w, &http.Cookie{
		Name: "arl_refresh", Value: refresh, Path: "/api/v1/auth", HttpOnly: true, Secure: secure,
		SameSite: http.SameSiteLaxMode, Expires: refreshExp,
	})
	http.SetCookie(w, &http.Cookie{
		Name: "arl_csrf", Value: csrf, Path: "/", HttpOnly: false, Secure: secure,
		SameSite: http.SameSiteLaxMode, Expires: refreshExp,
	})
}

func (s *Server) clearAuthCookies(w http.ResponseWriter) {
	for _, name := range []string{"arl_access", "arl_refresh", "arl_csrf"} {
		http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, HttpOnly: name != "arl_csrf", Secure: s.Config.Auth.CookieSecure})
	}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	res, err := s.Identity.Login(r.Context(), strings.TrimSpace(req.Username), req.Password, r.RemoteAddr, r.UserAgent())
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "auth.invalid_credentials", "invalid username or password")
		return
	}
	s.setAuthCookies(w, res.AccessToken, res.RefreshToken, res.CSRFToken, res.AccessExpires, res.RefreshExpires)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{"user": res.User, "csrfToken": res.CSRFToken},
	})
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("arl_refresh")
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "auth.missing_refresh", "refresh token missing")
		return
	}
	res, err := s.Identity.Refresh(r.Context(), c.Value, r.RemoteAddr, r.UserAgent())
	if err != nil {
		s.clearAuthCookies(w)
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "auth.invalid_refresh", err.Error())
		return
	}
	s.setAuthCookies(w, res.AccessToken, res.RefreshToken, res.CSRFToken, res.AccessExpires, res.RefreshExpires)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{"user": res.User, "csrfToken": res.CSRFToken},
	})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	if p == nil {
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "auth.unauthorized", "authentication required")
		return
	}
	var user *domain.User
	if !p.UserID.IsZero() {
		user, _ = s.Store.GetUserByID(r.Context(), p.UserID)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"user": user, "username": p.Username, "role": p.Role, "via": p.Via,
		},
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.Identity.Logout(r.Context(), principalFrom(r.Context()))
	s.clearAuthCookies(w)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": map[string]any{}})
}

type passwordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var req passwordRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if err := s.Identity.ChangePassword(r.Context(), principalFrom(r.Context()), req.OldPassword, req.NewPassword); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "auth.password_change_failed", err.Error())
		return
	}
	s.clearAuthCookies(w)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"message": "password changed; sign in again"}})
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	q := httpx.PageQuery(r)
	users, total, err := s.Store.ListUsers(r.Context(), q)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"items": users, "total": total, "page": q.Page, "pageSize": q.PageSize}})
}

type createUserRequest struct {
	Username string      `json:"username"`
	Password string      `json:"password"`
	Role     domain.Role `json:"role"`
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	u, err := s.Identity.CreateUser(r.Context(), req.Username, req.Password, req.Role)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "user_create_failed", err.Error())
		return
	}
	p := principalFrom(r.Context())
	s.Store.AddAudit(r.Context(), domain.AuditEvent{ActorID: p.UserID, Actor: p.Username, Action: "user.create", Resource: "user", Detail: map[string]any{"username": u.Username, "role": u.Role}, IP: r.RemoteAddr})
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"data": u})
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	p := principalFrom(r.Context())
	if p.UserID == id {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "user_self_delete", "cannot delete the current user")
		return
	}
	if err := s.Store.DeleteUser(r.Context(), id); err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"id": id.Hex()}})
}

func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := s.Store.ListAPIKeys(r.Context())
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"items": keys}})
}

type createAPIKeyRequest struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req createAPIKeyRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	raw, key, err := s.Identity.CreateAPIKey(r.Context(), principalFrom(r.Context()), req.Name, req.Scopes)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "api_key_create_failed", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"data": map[string]any{"key": key, "secret": raw}})
}

func (s *Server) handleRevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.Store.RevokeAPIKey(r.Context(), id); err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"id": id.Hex()}})
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	q := httpx.PageQuery(r)
	events, total, err := s.Store.ListAudit(r.Context(), q)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"items": events, "total": total, "page": q.Page, "pageSize": q.PageSize}})
}

func (s *Server) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"version": s.Version, "uptimeSec": int(time.Since(s.StartedAt).Seconds()),
		"queueDriver": s.Config.Queue.Driver, "scanners": pdtools.Versions(),
	}})
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	tasks, _ := s.Store.CountTasksByStatus(r.Context())
	assets, _ := s.Store.CountAssets(r.Context())
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"tasks": tasks, "assets": assets}})
}

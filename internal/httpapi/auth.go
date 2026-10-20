package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/platform/httpx"
	"github.com/red-copilot/red-x/internal/platform/pdtools"
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
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"user": res.User, "csrfToken": res.CSRFToken})
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
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"user": res.User, "csrfToken": res.CSRFToken})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	if p == nil {
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "auth.unauthorized", "authentication required")
		return
	}
	var user *domain.User
	if p.UserID != "" {
		user, _ = s.Store.GetUserByID(r.Context(), p.UserID)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"user": user, "username": p.Username, "role": p.Role, "via": p.Via,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.Identity.Logout(r.Context(), principalFrom(r.Context()))
	s.clearAuthCookies(w)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{})
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
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"message": "password changed; sign in again"})
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	q := httpx.PageQuery(r)
	users, total, err := s.Store.ListUsers(r.Context(), q)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": users, "total": total, "page": q.Page, "pageSize": q.PageSize})
}

func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	user, err := s.Store.GetUserByID(r.Context(), id)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	if user == nil {
		httpx.WriteProblem(w, r, http.StatusNotFound, "user_not_found", "user not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, user)
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
	httpx.WriteJSON(w, http.StatusCreated, u)
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	user, err := s.Store.GetUserByID(r.Context(), id)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	if user == nil {
		httpx.WriteProblem(w, r, http.StatusNotFound, "user_not_found", "user not found")
		return
	}
	var req struct {
		Role     *domain.Role `json:"role"`
		Disabled *bool        `json:"disabled"`
		Password *string      `json:"password"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	removeAdmin := user.Role == domain.RoleAdmin && !user.Disabled &&
		((req.Role != nil && *req.Role != domain.RoleAdmin) || (req.Disabled != nil && *req.Disabled))
	if removeAdmin {
		admins, err := s.Store.CountActiveAdmins(r.Context())
		if err != nil {
			httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		if admins <= 1 {
			httpx.WriteProblem(w, r, http.StatusConflict, "last_admin_protected", domain.ErrLastAdmin.Error())
			return
		}
	}
	if req.Role != nil {
		switch *req.Role {
		case domain.RoleViewer, domain.RoleOperator, domain.RoleAdmin:
			user.Role = *req.Role
		default:
			httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_role", "invalid role")
			return
		}
	}
	if req.Disabled != nil {
		user.Disabled = *req.Disabled
	}
	if err := s.Store.UpdateUser(r.Context(), user); err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	if req.Password != nil {
		if s.Identity == nil {
			httpx.WriteProblem(w, r, http.StatusServiceUnavailable, "identity_unavailable", "identity service is unavailable")
			return
		}
		if err := s.Identity.ResetPassword(r.Context(), id, *req.Password); err != nil {
			httpx.WriteProblem(w, r, http.StatusBadRequest, "password_reset_failed", err.Error())
			return
		}
	}
	if req.Role != nil || (req.Disabled != nil && *req.Disabled) {
		_ = s.Store.RevokeUserSessions(r.Context(), id)
	}
	p := principalFrom(r.Context())
	s.Store.AddAudit(r.Context(), domain.AuditEvent{
		ActorID: p.UserID, Actor: p.Username, Action: "user.update", Resource: id, IP: r.RemoteAddr,
	})
	updated, err := s.Store.GetUserByID(r.Context(), id)
	if err != nil || updated == nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", "updated user could not be loaded")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	p := principalFrom(r.Context())
	if err := s.Store.DeleteUser(r.Context(), id, p.Username, p.UserID); err != nil {
		switch {
		case errors.Is(err, domain.ErrSelfDelete):
			httpx.WriteProblem(w, r, http.StatusBadRequest, "user_self_delete", err.Error())
		case errors.Is(err, domain.ErrLastAdmin):
			httpx.WriteProblem(w, r, http.StatusConflict, "last_admin_protected", err.Error())
		case errors.Is(err, domain.ErrRecordNotFound):
			httpx.WriteProblem(w, r, http.StatusNotFound, "user_not_found", "user not found")
		default:
			httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		}
		return
	}
	s.Store.AddAudit(r.Context(), domain.AuditEvent{
		ActorID: p.UserID, Actor: p.Username, Action: "user.delete", Resource: id, IP: r.RemoteAddr,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id})
}

func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := s.Store.ListAPIKeys(r.Context())
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": keys})
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
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"key": key, "secret": raw})
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
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id})
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	q := httpx.PageQuery(r)
	events, total, err := s.Store.ListAudit(r.Context(), q)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": events, "total": total, "page": q.Page, "pageSize": q.PageSize})
}

func (s *Server) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"version": s.Version, "uptimeSec": int(time.Since(s.StartedAt).Seconds()),
		"queueDriver": s.Config.Queue.Driver, "scanners": pdtools.Versions(),
	})
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	tasks, _ := s.Store.CountTasksByStatus(r.Context())
	assets, _ := s.Store.CountAssets(r.Context())
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"tasks": tasks, "assets": assets})
}

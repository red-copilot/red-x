package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/modules/identity"
	"github.com/red-copilot/red-x/internal/platform/config"
	"github.com/red-copilot/red-x/internal/platform/logx"
)

func TestRefreshTokenCanRotateRepeatedly(t *testing.T) {
	st, ctx := newHTTPTestStore(t)
	cfg := config.Default()
	cfg.Auth.AccessSecret = "refresh-test-secret"
	cfg.Auth.AccessTTL = "15m"
	cfg.Auth.RefreshTTL = "1h"
	cfg.Auth.APIKeyPepper = "refresh-test-pepper"
	svc := identity.New(st, cfg, logx.New("error", false))
	if _, err := svc.CreateUser(ctx, "refresh-user", "long-password", domain.RoleOperator); err != nil {
		t.Fatal(err)
	}
	server := &Server{Config: cfg, Store: st, Identity: svc}

	login := httptest.NewRecorder()
	server.handleLogin(login, httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/auth/login",
		bytes.NewBufferString(`{"username":"refresh-user","password":"long-password"}`)))
	if login.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", login.Code, login.Body.String())
	}
	refresh := cookieByName(login.Result().Cookies(), "arl_refresh")
	if refresh == nil {
		t.Fatal("login did not set refresh cookie")
	}
	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/refresh", nil)
		req.AddCookie(refresh)
		server.handleRefresh(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("refresh %d status=%d body=%s", i+1, rec.Code, rec.Body.String())
		}
		refresh = cookieByName(rec.Result().Cookies(), "arl_refresh")
		if refresh == nil {
			t.Fatalf("refresh %d did not set a refresh cookie", i+1)
		}
	}
}

func TestAuthMiddlewareRequiresInitialPasswordChange(t *testing.T) {
	st, ctx := newHTTPTestStore(t)
	cfg := config.Default()
	cfg.Auth.AccessSecret = "password-gate-secret"
	cfg.Auth.AccessTTL = "15m"
	cfg.Auth.RefreshTTL = "1h"
	svc := identity.New(st, cfg, logx.New("error", false))
	if _, err := svc.CreateUser(ctx, "must-change", "long-password", domain.RoleOperator); err != nil {
		t.Fatal(err)
	}
	login, err := svc.Login(ctx, "must-change", "long-password", "127.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{Config: cfg, Store: st, Identity: svc}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	blocked := httptest.NewRecorder()
	blockedReq := httptest.NewRequest(http.MethodGet, "/api/v1/system/info", nil)
	blockedReq.AddCookie(&http.Cookie{Name: "arl_access", Value: login.AccessToken})
	server.authMiddleware(next).ServeHTTP(blocked, blockedReq)
	if blocked.Code != http.StatusForbidden || !bytes.Contains(blocked.Body.Bytes(), []byte("auth.password_change_required")) {
		t.Fatalf("blocked status=%d body=%s", blocked.Code, blocked.Body.String())
	}

	allowed := httptest.NewRecorder()
	allowedReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password", nil)
	allowedReq.AddCookie(&http.Cookie{Name: "arl_access", Value: login.AccessToken})
	server.authMiddleware(next).ServeHTTP(allowed, allowedReq)
	if allowed.Code != http.StatusNoContent {
		t.Fatalf("password endpoint status=%d body=%s", allowed.Code, allowed.Body.String())
	}
}

func TestPasswordChangeRevokesExistingAccessToken(t *testing.T) {
	st, ctx := newHTTPTestStore(t)
	cfg := config.Default()
	cfg.Auth.AccessSecret = "revoked-token-secret"
	cfg.Auth.AccessTTL = "15m"
	cfg.Auth.RefreshTTL = "1h"
	svc := identity.New(st, cfg, logx.New("error", false))
	user, err := svc.CreateUser(ctx, "revoke-me", "long-password", domain.RoleOperator)
	if err != nil {
		t.Fatal(err)
	}
	login, err := svc.Login(ctx, user.Username, "long-password", "127.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ChangePassword(ctx, &identity.Principal{UserID: user.ID, Username: user.Username}, "long-password", "new-long-password"); err != nil {
		t.Fatal(err)
	}
	server := &Server{Config: cfg, Store: st, Identity: svc}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "arl_access", Value: login.AccessToken})
	server.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked access token status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func cookieByName(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

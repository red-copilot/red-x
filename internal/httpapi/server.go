package httpapi

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/modules/easm"
	"github.com/red-copilot/red-x/internal/modules/github"
	"github.com/red-copilot/red-x/internal/modules/identity"
	"github.com/red-copilot/red-x/internal/modules/monitoring"
	"github.com/red-copilot/red-x/internal/modules/reporting"
	"github.com/red-copilot/red-x/internal/modules/tasking"
	"github.com/red-copilot/red-x/internal/modules/webintel"
	"github.com/red-copilot/red-x/internal/platform/config"
	"github.com/red-copilot/red-x/internal/platform/httpx"
	"github.com/red-copilot/red-x/internal/platform/jobs"
	"github.com/red-copilot/red-x/internal/store"
	"github.com/red-copilot/red-x/internal/webui"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type Server struct {
	Config    config.Config
	Store     *store.Store
	Queue     jobs.Queue
	Identity  *identity.Service
	Engine    *tasking.Engine
	Scheduler *monitoring.Scheduler
	GitHub    *github.Service
	Exporter  *reporting.Exporter
	EASM      *easm.Service
	Matcher   *webintel.FingerprintMatcher
	WIH       *webintel.WIHScanner
	Log       *slog.Logger
	Version   string
	StartedAt time.Time
}

type principalKey struct{}

func principalFrom(ctx context.Context) *identity.Principal {
	p, _ := ctx.Value(principalKey{}).(*identity.Principal)
	return p
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	r.Use(middleware.RealIP)
	r.Use(httpx.RequestLogger(s.Log))
	r.Use(httpx.Recoverer(s.Log))
	r.Use(httpx.SecurityHeaders)

	r.Get("/healthz", s.handleHealth)
	r.Get("/readyz", s.handleReady)
	r.Get("/metrics", s.handleMetrics)

	r.Route("/api/v1", func(api chi.Router) {
		api.Get("/openapi.json", s.handleOpenAPI)
		api.Get("/docs", s.handleDocs)
		api.Post("/auth/login", s.handleLogin)
		api.Post("/auth/refresh", s.handleRefresh)

		api.Group(func(protected chi.Router) {
			protected.Use(s.authMiddleware)
			protected.Use(s.csrfMiddleware)

			protected.Get("/auth/me", s.handleMe)
			protected.Post("/auth/logout", s.handleLogout)
			protected.Post("/auth/password", s.handleChangePassword)

			protected.Get("/system/info", s.handleSystemInfo)
			protected.Get("/dashboard", s.handleDashboard)
			protected.Get("/audit-events", s.handleAudit)

			protected.Get("/users", s.requireRole(domain.RoleAdmin, s.handleListUsers))
			protected.Post("/users", s.requireRole(domain.RoleAdmin, s.handleCreateUser))
			protected.Delete("/users/{id}", s.requireRole(domain.RoleAdmin, s.handleDeleteUser))
			protected.Get("/api-keys", s.requireRole(domain.RoleAdmin, s.handleListAPIKeys))
			protected.Post("/api-keys", s.requireRole(domain.RoleAdmin, s.handleCreateAPIKey))
			protected.Delete("/api-keys/{id}", s.requireRole(domain.RoleAdmin, s.handleRevokeAPIKey))

			protected.Get("/policies", s.handleListPolicies)
			protected.Post("/policies", s.requireWrite(s.handleCreatePolicy))
			protected.Put("/policies/{id}", s.requireWrite(s.handleUpdatePolicy))
			protected.Delete("/policies/{id}", s.requireAdmin(s.handleDeletePolicy))

			protected.Get("/scopes", s.handleListScopes)
			protected.Post("/scopes", s.requireWrite(s.handleCreateScope))
			protected.Get("/scopes/{id}", s.handleGetScope)
			protected.Delete("/scopes/{id}", s.requireAdmin(s.handleDeleteScope))
			protected.Get("/scopes/{id}/assets", s.handleScopeAssets)

			protected.Get("/tasks", s.handleListTasks)
			protected.Post("/tasks", s.requireWrite(s.handleCreateTask))
			protected.Get("/tasks/{id}", s.handleGetTask)
			protected.Delete("/tasks/{id}", s.requireWrite(s.handleDeleteTask))
			protected.Post("/tasks/{id}/cancel", s.requireWrite(s.handleCancelTask))
			protected.Post("/tasks/{id}/retry", s.requireWrite(s.handleRetryTask))
			protected.Post("/tasks/{id}/sync", s.requireWrite(s.handleSyncTask))
			protected.Get("/tasks/{id}/steps", s.handleTaskSteps)
			protected.Get("/tasks/{id}/observations", s.handleTaskObservations)
			protected.Get("/tasks/{id}/findings", s.handleTaskFindings)
			protected.Get("/tasks/{id}/events", s.handleTaskEvents)

			protected.Get("/assets/{kind}", s.handleAssets)
			protected.Get("/findings/{kind}", s.handleFindings)

			protected.Get("/monitors", s.handleListMonitors)
			protected.Post("/monitors", s.requireWrite(s.handleCreateMonitor))
			protected.Post("/monitors/{id}/pause", s.requireWrite(s.handlePauseMonitor))
			protected.Post("/monitors/{id}/resume", s.requireWrite(s.handleResumeMonitor))
			protected.Post("/monitors/{id}/run", s.requireWrite(s.handleRunMonitor))
			protected.Delete("/monitors/{id}", s.requireWrite(s.handleDeleteMonitor))

			protected.Get("/schedules", s.handleListSchedules)
			protected.Post("/schedules", s.requireWrite(s.handleCreateSchedule))
			protected.Delete("/schedules/{id}", s.requireWrite(s.handleDeleteSchedule))

			protected.Get("/github/tasks", s.handleListGitHubTasks)
			protected.Post("/github/tasks", s.requireWrite(s.handleCreateGitHubTask))
			protected.Get("/github/monitors", s.handleListGitHubMonitors)
			protected.Post("/github/monitors", s.requireWrite(s.handleCreateGitHubMonitor))
			protected.Delete("/github/monitors/{id}", s.requireWrite(s.handleDeleteGitHubMonitor))
			protected.Get("/github/findings", s.handleGitHubFindings)

			protected.Get("/fingerprints", s.handleListFingerprints)
			protected.Post("/fingerprints", s.requireWrite(s.handleCreateFingerprint))
			protected.Post("/fingerprints/import", s.requireWrite(s.handleImportFingerprints))
			protected.Delete("/fingerprints/{id}", s.requireWrite(s.handleDeleteFingerprint))

			protected.Get("/poc/plugins", s.handlePOCPlugins)
			protected.Post("/poc/sync", s.requireAdmin(s.handleSyncPOC))

			protected.Post("/exports", s.requireWrite(s.handleCreateExport))
			protected.Get("/exports/{name}", s.handleExportDownload)
			protected.Get("/screenshots/{taskId}/{name}", s.handleScreenshot)
		})
	})

	// SPA fallback. The embedded dist is replaced by the Vite build in release images.
	r.Handle("/*", s.serveSPA())
	return r
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		access := ""
		if c, err := r.Cookie("arl_access"); err == nil {
			access = c.Value
		}
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
				token := strings.TrimSpace(auth[7:])
				if strings.HasPrefix(token, "arl_") {
					apiKey = token
				} else {
					access = token
				}
			}
		}
		principal, err := s.Identity.Authenticate(r.Context(), access, apiKey)
		if err != nil {
			httpx.WriteProblem(w, r, http.StatusUnauthorized, "auth.unauthorized", "authentication required")
			return
		}
		ctx := context.WithValue(r.Context(), principalKey{}, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) csrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := principalFrom(r.Context())
		if p != nil && p.Via == "cookie" && r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			cookie, err := r.Cookie("arl_csrf")
			if err != nil || cookie.Value == "" || r.Header.Get("X-CSRF-Token") != cookie.Value {
				httpx.WriteProblem(w, r, http.StatusForbidden, "auth.csrf_failed", "CSRF token missing or invalid")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireRole(role domain.Role, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := principalFrom(r.Context())
		if p == nil || p.Role != role {
			httpx.WriteProblem(w, r, http.StatusForbidden, "auth.forbidden", "insufficient permissions")
			return
		}
		next(w, r)
	}
}

func (s *Server) requireWrite(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := principalFrom(r.Context())
		if p == nil || !p.CanWrite() {
			httpx.WriteProblem(w, r, http.StatusForbidden, "auth.forbidden", "write permission required")
			return
		}
		next(w, r)
	}
}

func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := principalFrom(r.Context())
		if p == nil || !p.IsAdmin() {
			httpx.WriteProblem(w, r, http.StatusForbidden, "auth.forbidden", "admin permission required")
			return
		}
		next(w, r)
	}
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (bson.ObjectID, bool) {
	id, err := store.ObjectID(chi.URLParam(r, name))
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_id", "invalid object id")
		return bson.NilObjectID, false
	}
	return id, true
}

func (s *Server) serveSPA() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			httpx.WriteProblem(w, r, http.StatusNotFound, "not_found", "route not found")
			return
		}
		if r.URL.Path != "/" {
			clean := path.Clean("/" + r.URL.Path)
			name := "dist" + clean
			if f, err := webui.Dist.Open(name); err == nil {
				defer f.Close()
				if ct := mime.TypeByExtension(path.Ext(name)); ct != "" {
					w.Header().Set("Content-Type", ct)
				}
				_, _ = io.Copy(w, f)
				return
			}
			if path.Ext(clean) != "" {
				httpx.WriteProblem(w, r, http.StatusNotFound, "not_found", "static asset not found")
				return
			}
		}
		index, err := webui.Dist.ReadFile("dist/index.html")
		if err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte("<!doctype html><title>ARL</title><h1>ARL API is running</h1><p>Frontend build is not embedded yet.</p>"))
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"status": "ok", "version": s.Version})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := s.Store.Ping(ctx); err != nil {
		httpx.WriteProblem(w, r, http.StatusServiceUnavailable, "dependency.mongo", err.Error())
		return
	}
	if err := s.Queue.Health(ctx); err != nil {
		httpx.WriteProblem(w, r, http.StatusServiceUnavailable, "dependency.queue", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"status": "ready"})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	counts, _ := s.Store.CountTasksByStatus(r.Context())
	assets, _ := s.Store.CountAssets(r.Context())
	var b strings.Builder
	b.WriteString("# HELP arl_tasks_total Number of tasks by status.\n# TYPE arl_tasks_total gauge\n")
	for k, v := range counts {
		fmt.Fprintf(&b, "arl_tasks_total{status=%q} %d\n", k, v)
	}
	b.WriteString("# HELP arl_assets_total Number of canonical assets by kind.\n# TYPE arl_assets_total gauge\n")
	for k, v := range assets {
		fmt.Fprintf(&b, "arl_assets_total{kind=%q} %d\n", k, v)
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = w.Write([]byte(b.String()))
}

package httpapi

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/modules/webintel"
	"github.com/red-copilot/red-x/internal/platform/httpx"
	"gopkg.in/yaml.v3"
)

func rPathParam(r *http.Request, name string) string {
	return chi.URLParam(r, name)
}

func webintelParse(rule string) (webintel.Expr, error) {
	return webintel.ParseFingerprint(rule)
}

func yamlNewDecoder(r *http.Request) *yaml.Decoder {
	return yaml.NewDecoder(r.Body)
}

func (s *Server) handlePOCPlugins(w http.ResponseWriter, r *http.Request) {
	s.Engine.InitPlugins()
	plugins := s.Engine.Plugins()
	type item struct {
		Name         string   `json:"name"`
		Type         string   `json:"type"`
		AppName      string   `json:"appName"`
		VulName      string   `json:"vulName"`
		Category     string   `json:"category"`
		Schemes      []string `json:"schemes"`
		DefaultPorts []int    `json:"defaultPorts"`
	}
	items := make([]item, 0, len(plugins))
	for _, p := range plugins {
		m := p.Meta()
		items = append(items, item{
			Name: m.Name, Type: string(m.Type), AppName: m.AppName, VulName: m.VulName,
			Category: m.Category, Schemes: m.Schemes, DefaultPorts: m.DefaultPorts,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (s *Server) handleSyncPOC(w http.ResponseWriter, r *http.Request) {
	s.Engine.InitPlugins()
	count := 0
	for _, p := range s.Engine.Plugins() {
		m := p.Meta()
		if err := s.Store.UpsertPOCPlugin(r.Context(), &domain.PocPlugin{
			Name: m.Name, Type: string(m.Type), AppName: m.AppName, VulName: m.VulName,
			Category: m.Category, Schemes: m.Schemes, Enabled: true,
		}); err == nil {
			count++
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"synced": count})
}

func (s *Server) handleCreateExport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TaskID string `json:"taskId"`
		Format string `json:"format"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	id, err := domain.ParseID(req.TaskID)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_task", "invalid task id")
		return
	}
	if req.Format == "" {
		req.Format = "xlsx"
	}
	path, err := s.Exporter.ExportTask(r.Context(), id, strings.ToLower(req.Format))
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "export_failed", err.Error())
		return
	}
	name := filepath.Base(path)
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"file": name, "downloadUrl": "/api/v1/exports/" + name})
}

func (s *Server) handleScreenshot(w http.ResponseWriter, r *http.Request) {
	taskID := rPathParam(r, "taskId")
	name := rPathParam(r, "name")
	if !safeSegment(taskID) || !safeSegment(name) {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_path", "invalid screenshot path")
		return
	}
	path := filepath.Join(s.Config.Storage.ScreenshotsDir, taskID, name)
	absDir, _ := filepath.Abs(s.Config.Storage.ScreenshotsDir)
	absPath, _ := filepath.Abs(path)
	if !strings.HasPrefix(absPath, absDir+string(os.PathSeparator)) {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_path", "invalid screenshot path")
		return
	}
	if _, err := os.Stat(path); err != nil {
		httpx.WriteProblem(w, r, http.StatusNotFound, "not_found", "screenshot not found")
		return
	}
	http.ServeFile(w, r, path)
}

func (s *Server) handleExportDownload(w http.ResponseWriter, r *http.Request) {
	name := rPathParam(r, "name")
	if !safeSegment(name) {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_path", "invalid export path")
		return
	}
	path := filepath.Join(s.Config.Storage.ExportsDir, name)
	absDir, _ := filepath.Abs(s.Config.Storage.ExportsDir)
	absPath, _ := filepath.Abs(path)
	if !strings.HasPrefix(absPath, absDir+string(os.PathSeparator)) {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "invalid_path", "invalid export path")
		return
	}
	if _, err := os.Stat(path); err != nil {
		httpx.WriteProblem(w, r, http.StatusNotFound, "not_found", "export not found")
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename="+name)
	http.ServeFile(w, r, path)
}

func safeSegment(s string) bool {
	if s == "" || s == "." || s == ".." {
		return false
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}

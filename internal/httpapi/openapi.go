package httpapi

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.json
var openAPISpec string

func (s *Server) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(openAPISpec))
}

func (s *Server) handleDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!doctype html><html><head><meta charset="utf-8"><title>ARL API</title>
<style>body{font-family:system-ui;margin:40px;line-height:1.5}code{background:#eee;padding:2px 4px}</style></head>
<body><h1>ARL API v1</h1><p>OpenAPI document: <a href="/api/v1/openapi.json">/api/v1/openapi.json</a></p>
<p>Authentication: HttpOnly cookie <code>arl_access</code> plus <code>X-CSRF-Token</code> for writes, or <code>X-API-Key</code>.</p>
<p>Errors use <code>application/problem+json</code>.</p></body></html>`))
}

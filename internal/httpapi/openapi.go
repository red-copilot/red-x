package httpapi

import (
	"net/http"
)

const openAPISpec = `{
  "openapi": "3.1.0",
  "info": {"title": "ARL API", "version": "3.0.0", "description": "Asset Reconnaissance Lighthouse modular Go API"},
  "servers": [{"url": "/api/v1"}],
  "security": [{"cookieAuth": []}, {"apiKey": []}],
  "components": {
    "securitySchemes": {
      "cookieAuth": {"type": "apiKey", "in": "cookie", "name": "arl_access"},
      "apiKey": {"type": "apiKey", "in": "header", "name": "X-API-Key"}
    },
    "schemas": {
      "Problem": {"type": "object", "properties": {
        "type": {"type": "string"}, "title": {"type": "string"}, "status": {"type": "integer"},
        "code": {"type": "string"}, "detail": {"type": "string"}, "requestId": {"type": "string"}
      }},
      "ScannerVersions": {"type": "object", "additionalProperties": {"type": "string"}}
    }
  },
  "paths": {
    "/auth/login": {"post": {"summary": "Login", "requestBody": {"required": true, "content": {"application/json": {"schema": {"type": "object", "required": ["username","password"], "properties": {"username": {"type": "string"}, "password": {"type": "string"}}}}}}, "responses": {"200": {"description": "Authenticated"}, "401": {"description": "Invalid credentials"}}}},
    "/auth/refresh": {"post": {"summary": "Rotate refresh token", "responses": {"200": {"description": "Refreshed"}}}},
    "/auth/me": {"get": {"summary": "Current principal", "responses": {"200": {"description": "Current user"}}}},
    "/tasks": {"get": {"summary": "List tasks"}, "post": {"summary": "Create task"}},
    "/tasks/{id}": {"get": {"summary": "Get task"}, "delete": {"summary": "Delete task"}},
    "/tasks/{id}/cancel": {"post": {"summary": "Cancel task"}},
    "/tasks/{id}/retry": {"post": {"summary": "Retry task"}},
    "/tasks/{id}/events": {"get": {"summary": "Task SSE stream"}},
    "/assets/{kind}": {"get": {"summary": "List assets"}},
    "/findings/{kind}": {"get": {"summary": "List findings"}},
    "/scopes": {"get": {"summary": "List scopes"}, "post": {"summary": "Create scope"}},
    "/policies": {"get": {"summary": "List policies"}, "post": {"summary": "Create policy"}},
    "/monitors": {"get": {"summary": "List monitors"}, "post": {"summary": "Create monitor"}},
    "/schedules": {"get": {"summary": "List schedules"}, "post": {"summary": "Create schedule"}},
    "/github/tasks": {"get": {"summary": "List GitHub tasks"}, "post": {"summary": "Create GitHub task"}},
    "/exports": {"post": {"summary": "Create export"}},
    "/poc/plugins": {"get": {"summary": "List PoC plugins"}},
    "/system/info": {"get": {"summary": "System information", "responses": {"200": {"description": "Version, uptime, queue driver and scanner SDK versions"}}}}
  }
}`

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

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
    "/organizations": {"get": {"summary": "List EASM organizations"}, "post": {"summary": "Create organization"}},
    "/organizations/import": {"post": {"summary": "Import organizations and candidate domains from CSV"}},
    "/organizations/{id}": {"get": {"summary": "Get organization and domains"}, "patch": {"summary": "Update organization"}},
    "/organizations/{id}/domains/confirm": {"post": {"summary": "Confirm candidate domains"}},
    "/organizations/{id}/domains/import": {"post": {"summary": "Add candidate domains"}},
    "/organizations/{id}/domains/{domainId}/reject": {"post": {"summary": "Reject or remove a domain attribution"}},
    "/organizations/{id}/domains/{domainId}/reassign": {"post": {"summary": "Reassign a confirmed domain"}},
    "/organizations/{id}/domains/{domainId}/evidence": {"get": {"summary": "Get ICP attribution evidence"}},
    "/easm/organizations/{id}/summary": {"get": {"summary": "Organization EASM summary"}},
    "/easm/organizations/{id}/assets": {"get": {"summary": "Organization EASM assets"}},
    "/easm/organizations/{id}/findings": {"get": {"summary": "Organization current findings"}},
    "/easm/organizations/{id}/findings/{findingId}/history": {"get": {"summary": "Finding history"}},
    "/easm/organizations/{id}/tasks": {"get": {"summary": "Organization EASM tasks"}},
    "/easm/organizations/{id}/scans": {"post": {"summary": "Create one EASM scan task per confirmed root domain"}},
    "/easm/organizations/{id}/exports": {"post": {"summary": "Export organization XLSX or CSV"}},
    "/easm/assets": {"get": {"summary": "Cross-organization EASM assets"}},
    "/easm/assets/{id}": {"get": {"summary": "EASM asset detail"}},
    "/easm/findings": {"get": {"summary": "Cross-organization current findings"}},
    "/easm/tasks": {"get": {"summary": "Cross-organization EASM tasks"}},
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

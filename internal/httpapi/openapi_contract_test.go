package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestOpenAPICoversEveryAPIRoute(t *testing.T) {
	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal([]byte(openAPISpec), &spec); err != nil {
		t.Fatalf("invalid OpenAPI JSON: %v", err)
	}

	documented := make(map[string]bool)
	for path, operations := range spec.Paths {
		for method := range operations {
			if method == "parameters" || method == "servers" || method == "summary" || method == "description" {
				continue
			}
			documented[strings.ToUpper(method)+" "+path] = true
		}
	}

	var missing []string
	err := chi.Walk((&Server{}).Router().(chi.Routes), func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if !strings.HasPrefix(route, "/api/v1/") {
			return nil
		}
		method = strings.ToUpper(method)
		if method == http.MethodHead {
			return nil
		}
		path := strings.TrimPrefix(route, "/api/v1")
		if !documented[method+" "+path] {
			missing = append(missing, method+" "+path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk routes: %v", err)
	}
	if len(missing) > 0 {
		t.Fatalf("OpenAPI is missing routes:\n%s", strings.Join(missing, "\n"))
	}
}

func TestOpenAPIOperationsHaveUniqueIDsAndResponses(t *testing.T) {
	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal([]byte(openAPISpec), &spec); err != nil {
		t.Fatalf("invalid OpenAPI JSON: %v", err)
	}

	seen := make(map[string]string)
	for path, methods := range spec.Paths {
		for method, raw := range methods {
			switch method {
			case "parameters", "servers", "summary", "description":
				continue
			}
			var operation struct {
				OperationID string                     `json:"operationId"`
				Responses   map[string]json.RawMessage `json:"responses"`
			}
			if err := json.Unmarshal(raw, &operation); err != nil {
				t.Errorf("%s %s is not an operation: %v", strings.ToUpper(method), path, err)
				continue
			}
			if operation.OperationID == "" {
				t.Errorf("%s %s has no operationId", strings.ToUpper(method), path)
			}
			if previous, ok := seen[operation.OperationID]; ok {
				t.Errorf("duplicate operationId %q for %s and %s", operation.OperationID, previous, method+" "+path)
			}
			seen[operation.OperationID] = fmt.Sprintf("%s %s", strings.ToUpper(method), path)
			if len(operation.Responses) == 0 {
				t.Errorf("%s %s has no responses", strings.ToUpper(method), path)
			}
		}
	}
}

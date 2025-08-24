package openapi

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Simple in-memory registry of dynamic operation definitions.
// In a production system you might derive these from database-registered
// connector capability metadata. Here we keep it minimal.

// Operation represents a single HTTP operation to surface in OpenAPI.
type Operation struct {
	Method      string         `json:"method"`
	Path        string         `json:"path"`
	Summary     string         `json:"summary,omitempty"`
	Description string         `json:"description,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Scopes      []string       `json:"x-required-scopes,omitempty"`
	RequestBody any            `json:"requestBody,omitempty"`
	Responses   map[string]any `json:"responses"`
}

// Registry holds registered operations (across all dynamic connectors).
type Registry struct {
	Ops []Operation
}

func NewRegistry() *Registry { return &Registry{Ops: []Operation{}} }

func (r *Registry) Register(op Operation) {
	if op.Method != "" {
		op.Method = strings.ToLower(op.Method)
	}
	r.Ops = append(r.Ops, op)
}

// Build produces a minimal OpenAPI 3.1 document representing the currently
// registered operations. Components/schemas are kept inline for brevity.
func (r *Registry) Build(serviceName, version string) map[string]any {
	paths := map[string]any{}
	for _, op := range r.Ops {
		if _, ok := paths[op.Path]; !ok {
			paths[op.Path] = map[string]any{}
		}
		m := map[string]any{
			"summary":     op.Summary,
			"description": op.Description,
			"tags":        op.Tags,
			"responses":   op.Responses,
		}
		if len(op.Scopes) > 0 {
			m["x-required-scopes"] = op.Scopes
		}
		if op.RequestBody != nil {
			m["requestBody"] = op.RequestBody
		}
		paths[op.Path].(map[string]any)[op.Method] = m
	}
	return map[string]any{
		"openapi": "3.1.0",
		"info":    map[string]any{"title": serviceName, "version": version},
		"paths":   paths,
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"oauth": map[string]any{
					"type": "oauth2",
					"flows": map[string]any{
						"clientCredentials": map[string]any{
							"tokenUrl": "/oauth/token",
							"scopes": map[string]string{
								"order:write":  "Create orders / checkouts",
								"refund:write": "Issue refunds",
							},
						},
					},
				},
			},
		},
		"security": []map[string]any{{"oauth": []string{}}},
	}
}

// ServeHandler returns an HTTP handler that serves the built OpenAPI JSON.
func (r *Registry) ServeHandler(serviceName, version string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(r.Build(serviceName, version))
	}
}

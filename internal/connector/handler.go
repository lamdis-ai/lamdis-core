// internal/connector/handler.go
package connector

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lamdis/internal/policy"
	"lamdis/pkg/config"
	"lamdis/pkg/connectors"
	"lamdis/pkg/middleware"
	"lamdis/pkg/openapi"
	"lamdis/pkg/tenants"
)

// CanonicalOperation describes a first-class platform action.
type CanonicalOperation struct {
	Method   string
	Path     string
	Summary  string
	Scopes   []string // any-of list
	ID       string   // canonical id e.g. orders.locate
	Mode     string   // default mode hint (execute|request|refer)
	UserOnly bool     // disallow client_credentials if true
}

var canonicalOps = []CanonicalOperation{
	{ID: "orders.locate", Method: "GET", Path: "/v1/orders/locate", Summary: "Find order across rails", Scopes: []string{"orders:read"}, Mode: "execute"},
	{ID: "orders.status", Method: "GET", Path: "/v1/orders/{rail}/{id}/status", Summary: "Get order status", Scopes: []string{"orders:read"}, Mode: "execute"},
	{ID: "orders.cancel", Method: "POST", Path: "/v1/orders/{rail}/{id}/cancel", Summary: "Attempt cancel", Scopes: []string{"orders:cancel"}, Mode: "execute", UserOnly: true},
	{ID: "refunds.request", Method: "POST", Path: "/v1/orders/{rail}/{id}/refund", Summary: "Request refund", Scopes: []string{"refunds:request"}, Mode: "request", UserOnly: true},
	{ID: "reorder.link", Method: "POST", Path: "/v1/reorder", Summary: "Reorder (deep link or first-party)", Scopes: []string{"reorder:create"}, Mode: "execute"},
}

// DynamicRouter builds routes dynamically for the active tenant using the connectors registry.
func DynamicRouter(r chi.Router, cfg config.Config, tenantProv tenants.Provider, reg *connectors.Registry, pool *pgxpool.Pool) {
	// Public endpoints (no auth)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})
	// CORS preflight and public OpenAPI
	r.Options("/.well-known/openapi.json", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.WriteHeader(http.StatusNoContent)
	})
	r.Get("/.well-known/openapi.json", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		serveOpenAPI(w, req, reg)
	})
	// Protected group
	r.Group(func(pr chi.Router) {
		pr.Use(middleware.JWTAuth(cfg, tenantProv, nil))
		// Two-phase policy endpoints
		policy.RegisterHTTP(pr, pool)
		// Mount canonical first
		for _, co := range canonicalOps {
			co := co
			pr.Method(co.Method, co.Path, canonicalHandler(co, pool))
		}
		// Dynamic connector-provided operations under /v1 (already include /v1 in path definitions)
		pr.Mount("/", dynamicOperationsRouter(reg, pool))
	})
}

func serveOpenAPI(w http.ResponseWriter, req *http.Request, reg *connectors.Registry) {
	ctx := req.Context()
	tenant := middleware.TenantFrom(ctx)
	ops, _ := reg.LoadOperations(ctx, tenant.ID)
	regDoc := openapi.NewRegistry()
	// Canonical
	for _, c := range canonicalOps {
		regDoc.Register(openapi.Operation{Method: c.Method, Path: c.Path, Summary: c.Summary, Scopes: c.Scopes, Responses: map[string]any{"200": map[string]any{"description": "OK"}}})
	}
	// Dynamic
	for _, o := range ops {
		regDoc.Register(openapi.Operation{Method: o.Method, Path: o.Path, Summary: o.Summary, Scopes: o.Scopes, Responses: map[string]any{"200": map[string]any{"description": "OK"}}})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(regDoc.Build("connector-service", "v1"))
}

// dynamicOperationsRouter builds a sub-router with concrete operation endpoints at request time (lazy building each call if uncached).
func dynamicOperationsRouter(reg *connectors.Registry, pool *pgxpool.Pool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		tenant := middleware.TenantFrom(ctx)
		ops, err := reg.LoadOperations(ctx, tenant.ID)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		router := chi.NewRouter()
		for _, o := range ops {
			method := strings.ToUpper(o.Method)
			p := o.Path
			if strings.HasPrefix(p, "/v1/") { // keep as-is
			}
			required := o.Scopes
			handler := makeDynamicOperationHandler(method, p, o.BaseURL, o.AuthRef, pool)
			router.Method(method, p, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !middleware.HasAnyScope(r.Context(), required) {
					http.Error(w, "insufficient_scope", http.StatusForbidden)
					return
				}
				handler.ServeHTTP(w, r)
			}))
		}
		router.ServeHTTP(w, req)
	})
}

// canonicalHandler implements mode selection + basic policy enforcement.
func canonicalHandler(co CanonicalOperation, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if !middleware.HasAnyScope(ctx, co.Scopes) {
			http.Error(w, "insufficient_scope", http.StatusForbidden)
			return
		}
		// Grant type (if machine & user-only) -> forbid
		if co.UserOnly {
			if gt := middleware.GrantTypeFrom(ctx); gt == "client_credentials" {
				http.Error(w, "grant_not_allowed", http.StatusForbidden)
				return
			}
		}
		start := time.Now()
		var body any
		if r.Body != nil && (r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch) {
			b, _ := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
			if len(b) > 0 {
				_ = json.Unmarshal(b, &body)
			}
		}
		// Placeholder business logic; choose mode from canonical default (later influenced by connector capability & policy)
		mode := co.Mode
		result := map[string]any{}
		// Very minimal sample responses
		switch co.ID {
		case "orders.locate":
			result["matches"] = []map[string]any{}
		case "orders.status":
			result["status"] = "UNKNOWN"
		case "orders.cancel":
			result["accepted"] = true
		case "refunds.request":
			result["case_id"] = "CASE-PLACEHOLDER"
		case "reorder.link":
			result["link"] = "https://example.com/reorder/demo"
		}
		actorSub := middleware.ActorSub(ctx)
		tenant := middleware.TenantFrom(ctx)
		reqID := ""
		if v := ctx.Value(middleware.CtxKeyRequestID); v != nil {
			if s, ok := v.(string); ok {
				reqID = s
			}
		}
		resp := map[string]any{
			"mode":   mode,
			"result": result,
			"audit": map[string]any{
				"action":    co.ID,
				"actor_sub": actorSub,
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			},
			"duration_ms": time.Since(start).Milliseconds(),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
		// log usage after response write
		if pool != nil && tenant.ID != "" {
			dur := time.Since(start)
			_, _ = pool.Exec(ctx, `
				INSERT INTO usage_events(tenant_id, action_id, method, path, mode, rail, actor_sub, request_id, status_code, duration_ms, started_at, finished_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
			`, tenant.ID, co.ID, co.Method, co.Path, mode, "", actorSub, reqID, http.StatusOK, int(dur.Milliseconds()), start.UTC(), time.Now().UTC())
		}
	}
}

// makeOperationHandler returns a placeholder execution handler for a dynamic operation with logging.
// makeDynamicOperationHandler executes dynamic operations. If baseURL is provided, it proxies to that upstream (passthrough); otherwise returns a simple echo payload.
func makeDynamicOperationHandler(method, opPath string, baseURL *string, authRef *string, pool *pgxpool.Pool) http.HandlerFunc {
	client := &http.Client{Timeout: 15 * time.Second}
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ctx := r.Context()
		tenant := middleware.TenantFrom(ctx)
		actorSub := middleware.ActorSub(ctx)
		reqID := ""
		if v := ctx.Value(middleware.CtxKeyRequestID); v != nil {
			if s, ok := v.(string); ok {
				reqID = s
			}
		}
		statusCode := http.StatusOK
		var resultBody any
		// Passthrough when baseURL present
		if baseURL != nil && *baseURL != "" {
			upBase := strings.TrimRight(*baseURL, "/")
			// Use the request's path (already concrete with params) but keep only the operation suffix (assumes opPath starts with /)
			reqPath := opPath
			// Build full URL
			full := upBase + reqPath
			if _, err := url.Parse(full); err != nil {
				http.Error(w, "invalid_upstream_url", http.StatusBadGateway)
				return
			}
			var bodyBytes []byte
			if r.Body != nil && (method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch) {
				bodyBytes, _ = io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
			}
			upReq, err := http.NewRequestWithContext(ctx, method, full, bytes.NewReader(bodyBytes))
			if err != nil {
				http.Error(w, "upstream_request_build_failed", http.StatusBadGateway)
				return
			}
			// Inject auth if configured via authRef (supports simple api_key)
			if authRef != nil && *authRef != "" && pool != nil {
				var typ string
				var cfgRaw, secEnc []byte
				if err := pool.QueryRow(ctx, `SELECT type, config, secrets_encrypted FROM tenant_auth_configs WHERE id=$1`, *authRef).Scan(&typ, &cfgRaw, &secEnc); err == nil {
					var cfg map[string]any
					_ = json.Unmarshal(cfgRaw, &cfg)
					if strings.EqualFold(typ, "api_key") {
						apiKey := ""
						if v, ok := cfg["api_key"].(string); ok && v != "" { // plain config override
							apiKey = v
						}
						if apiKey == "" && len(secEnc) > 0 { // decrypt blob
							if k := os.Getenv("ENCRYPTION_KEY"); k != "" {
								if secrets, err := decryptSecrets(secEnc, []byte(k)); err == nil {
									if v, ok := secrets["api_key"].(string); ok && v != "" {
										apiKey = v
									}
								}
							}
						}
						if apiKey != "" {
							upReq.Header.Set("x-api-key", apiKey)
						}
					}
				}
			}
			// Minimal header passthrough
			if ct := r.Header.Get("Content-Type"); ct != "" {
				upReq.Header.Set("Content-Type", ct)
			}
			if accept := r.Header.Get("Accept"); accept != "" {
				upReq.Header.Set("Accept", accept)
			}
			resp, err := client.Do(upReq)
			if err != nil {
				http.Error(w, "upstream_unreachable", http.StatusBadGateway)
				return
			}
			defer resp.Body.Close()
			statusCode = resp.StatusCode
			respBytes, _ := io.ReadAll(http.MaxBytesReader(w, resp.Body, 4<<20))
			ct := resp.Header.Get("Content-Type")
			if strings.Contains(ct, "application/json") {
				var jb any
				if err := json.Unmarshal(respBytes, &jb); err == nil {
					resultBody = jb
				} else {
					resultBody = string(respBytes)
				}
			} else {
				// Return raw as string (could be HTML/text); callers can inspect
				resultBody = string(respBytes)
			}
			w.Header().Set("X-Connector-Upstream", full)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"passthrough":     true,
				"upstream_status": statusCode,
				"operation":       map[string]any{"method": method, "path": opPath},
				"upstream":        resultBody,
				"duration_ms":     time.Since(start).Milliseconds(),
			})
		} else {
			// Fallback echo (no upstream)
			var body any
			if r.Body != nil && (method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch) {
				b, _ := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
				if len(b) > 0 {
					_ = json.Unmarshal(b, &body)
				}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":          true,
				"operation":   map[string]any{"method": method, "path": opPath},
				"received":    body,
				"duration_ms": time.Since(start).Milliseconds(),
			})
		}
		// Usage log (shared)
		if pool != nil && tenant.ID != "" {
			dur := time.Since(start)
			_, _ = pool.Exec(ctx, `
				INSERT INTO usage_events(tenant_id, action_id, method, path, mode, rail, actor_sub, request_id, status_code, duration_ms, started_at, finished_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
			`, tenant.ID, "", method, opPath, "", "", actorSub, reqID, statusCode, int(dur.Milliseconds()), start.UTC(), time.Now().UTC())
		}
	}
}

// decryptSecrets reverses the admin-api encryptJSON format (versioned: 0x01 | nonce | ciphertext[GCM]).
func decryptSecrets(blob []byte, key []byte) (map[string]any, error) {
	if len(blob) < 2 { // version + minimal nonce
		return nil, fmt.Errorf("invalid blob")
	}
	if blob[0] != 0x01 { // only support version 1
		return nil, fmt.Errorf("unsupported version")
	}
	h := sha256.Sum256(key)
	block, err := aes.NewCipher(h[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(blob) < 1+gcm.NonceSize() {
		return nil, fmt.Errorf("short nonce")
	}
	nonce := blob[1 : 1+gcm.NonceSize()]
	ct := blob[1+gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(plain, &m); err != nil {
		return nil, err
	}
	return m, nil
}

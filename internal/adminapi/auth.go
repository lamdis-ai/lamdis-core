package adminapi

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// mustJWKS fetches JWKS and panics on failure.
func mustJWKS(url string) jwk.Set {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	set, err := jwk.Fetch(ctx, url)
	if err != nil {
		panic(err)
	}
	return set
}

// cors returns a middleware that sets CORS headers and handles preflight requests.
// allowed may contain exact origins (e.g., http://localhost:3001) or "*" to allow all.
func cors(allowed []string) func(http.Handler) http.Handler {
	match := func(origin string) (string, bool) {
		if origin == "" {
			return "", false
		}
		for _, a := range allowed {
			a = strings.TrimSpace(a)
			if a == "*" || a == origin {
				return a, true
			}
		}
		return "", false
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if ao, ok := match(origin); ok {
				allowOrigin := ao
				if ao == "*" {
					allowOrigin = "*"
				}
				w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Tenant-ID")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Max-Age", "86400")
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// adminAuth validates admin bearer or allows dev header override when JWKS not configured.
func (a *App) adminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.adminJWKS == nil {
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			if tid := strings.TrimSpace(r.Header.Get("X-Tenant-ID")); tid != "" {
				if _, err := uuid.Parse(tid); err != nil {
					var resolved string
					_ = a.db.QueryRow(r.Context(), `SELECT id FROM tenants WHERE slug=$1 OR host=$1 LIMIT 1`, tid).Scan(&resolved)
					if strings.TrimSpace(resolved) == "" {
						http.Error(w, "invalid tenant id", http.StatusBadRequest)
						return
					}
					tid = resolved
				}
				ctx := context.WithValue(r.Context(), "tid", tid)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			http.Error(w, "missing tenant id", http.StatusBadRequest)
			return
		}

		authz := r.Header.Get("Authorization")
		if !strings.HasPrefix(strings.ToLower(authz), "bearer ") {
			http.Error(w, "missing bearer", http.StatusUnauthorized)
			return
		}
		tok := strings.TrimSpace(authz[len("Bearer "):])
		jt, err := jwt.Parse([]byte(tok),
			jwt.WithKeySet(a.adminJWKS),
			jwt.WithIssuer(a.adminIssuer),
			jwt.WithAudience(a.adminAud),
			jwt.WithValidate(true),
		)
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		role, _ := jt.Get("role")
		if role == nil || (role.(string) != "tenant_admin" && role.(string) != "lamdis_admin") {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		tid, _ := jt.Get("tid")
		if tid == nil {
			if h := r.Header.Get("X-Tenant-ID"); h != "" {
				tid = h
			}
		}
		if tid == nil {
			http.Error(w, "missing tenant id", http.StatusBadRequest)
			return
		}
		v := fmt.Sprint(tid)
		if _, err := uuid.Parse(v); err != nil {
			var resolved string
			_ = a.db.QueryRow(r.Context(), `SELECT id FROM tenants WHERE slug=$1 OR host=$1 LIMIT 1`, v).Scan(&resolved)
			if strings.TrimSpace(resolved) == "" {
				http.Error(w, "invalid tenant id", http.StatusBadRequest)
				return
			}
			v = resolved
		}
		ctx := context.WithValue(r.Context(), "tid", v)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

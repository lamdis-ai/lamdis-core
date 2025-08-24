// pkg/middleware/scope.go
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/lestrrat-go/jwx/v2/jwt"
)

// local context key type (unique to this file)
type scopeCtxKey string

const (
	ctxScopesKey scopeCtxKey = "scopes"
)

// WithScopes stores scopes slice in context.
func WithScopes(ctx context.Context, scopes []string) context.Context {
	return context.WithValue(ctx, ctxScopesKey, scopes)
}

// ScopesFrom extracts scopes slice from context.
func ScopesFrom(ctx context.Context) []string {
	if v := ctx.Value(ctxScopesKey); v != nil {
		if s, ok := v.([]string); ok {
			return s
		}
	}
	return nil
}

// RequireScope (legacy) parses the incoming token inline and checks for a single scope.
// Will be replaced by a dedicated JWT verification middleware that populates context.
func RequireScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authz := r.Header.Get("Authorization")
			if !strings.HasPrefix(strings.ToLower(authz), "bearer ") {
				http.Error(w, "missing bearer", http.StatusUnauthorized)
				return
			}
			tok := strings.TrimSpace(authz[len("Bearer "):])
			jt, err := jwt.Parse([]byte(tok))
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			var scopes []string
			if sc, ok := jt.Get("scope"); ok {
				parts := strings.Fields(sc.(string))
				scopes = append(scopes, parts...)
			}
			// store scopes for downstream handlers
			r = r.WithContext(WithScopes(r.Context(), scopes))
			if scope != "" {
				found := false
				for _, s := range scopes {
					if s == scope {
						found = true
						break
					}
				}
				if !found {
					http.Error(w, "insufficient_scope", http.StatusForbidden)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// HasAnyScope returns true if context holds at least one of the required scopes.
func HasAnyScope(ctx context.Context, required []string) bool {
	if len(required) == 0 {
		return true
	}
	curr := ScopesFrom(ctx)
	if len(curr) == 0 {
		return false
	}
	set := map[string]struct{}{}
	for _, s := range curr {
		set[s] = struct{}{}
	}
	for _, r := range required {
		if _, ok := set[r]; ok {
			return true
		}
	}
	return false
}

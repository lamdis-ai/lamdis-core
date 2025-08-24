// pkg/middleware/auth.go
package middleware

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"lamdis/pkg/config"
	"lamdis/pkg/tenants"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/redis/go-redis/v9"
)

// jwksCache caches JWKS sets per URL.
type jwksCache struct {
	mu   sync.RWMutex
	sets map[string]cachedJWKS
}

type cachedJWKS struct {
	set     jwk.Set
	expires time.Time
}

func (c *jwksCache) get(ctx context.Context, url string, ttl time.Duration) (jwk.Set, error) {
	c.mu.RLock()
	if e, ok := c.sets[url]; ok && time.Now().Before(e.expires) {
		c.mu.RUnlock()
		return e.set, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sets == nil {
		c.sets = map[string]cachedJWKS{}
	}
	if e, ok := c.sets[url]; ok && time.Now().Before(e.expires) {
		return e.set, nil
	}
	set, err := jwk.Fetch(ctx, url)
	if err != nil {
		return nil, err
	}
	c.sets[url] = cachedJWKS{set: set, expires: time.Now().Add(ttl)}
	return set, nil
}

// JWTAuth validates access tokens using tenant-specific config and populates scopes in context.
func JWTAuth(cfg config.Config, prov tenants.Provider, rdb *redis.Client) func(http.Handler) http.Handler {
	cache := &jwksCache{}
	jwksTTL := 6 * time.Hour
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Bypass auth for health and metrics endpoints
			if r.URL.Path == "/healthz" || r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}
			// Public well-known endpoints should not require auth
			if strings.HasPrefix(r.URL.Path, "/.well-known/") {
				next.ServeHTTP(w, r)
				return
			}

			tenant := TenantFrom(r.Context())
			issuer := strings.TrimRight(tenant.OAuthIssuer, "/")
			jwksURL := tenant.JWKSURL
			if issuer == "" {
				issuer = strings.TrimRight(cfg.Issuer, "/")
			}
			if jwksURL == "" {
				jwksURL = cfg.JWKSURL
			}
			// In dev, allow requests without Authorization to pass through (facilitates local bring-up)
			authz := r.Header.Get("Authorization")
			if cfg.Env == "dev" && strings.TrimSpace(authz) == "" {
				next.ServeHTTP(w, r)
				return
			}
			if issuer == "" || jwksURL == "" {
				http.Error(w, "auth not configured", http.StatusInternalServerError)
				return
			}

			set, err := cache.get(r.Context(), jwksURL, jwksTTL)
			if err != nil {
				http.Error(w, "jwks fetch failed", http.StatusInternalServerError)
				return
			}

			if !strings.HasPrefix(strings.ToLower(authz), "bearer ") {
				http.Error(w, "missing bearer", http.StatusUnauthorized)
				return
			}
			raw := strings.TrimSpace(authz[len("Bearer "):])

			parseOpts := []jwt.ParseOption{jwt.WithKeySet(set), jwt.WithIssuer(issuer), jwt.WithValidate(true), jwt.WithVerify(true), jwt.WithAcceptableSkew(cfg.DPoPClockSkew)}
			// Audience list (tenant accepted audiences or fallback)
			accepted := tenant.AcceptedAudiences
			if len(accepted) == 0 && cfg.Audience != "" {
				accepted = []string{cfg.Audience}
			}
			if len(accepted) == 1 {
				parseOpts = append(parseOpts, jwt.WithAudience(accepted[0]))
			}
			jt, perr := jwt.Parse([]byte(raw), parseOpts...)
			if perr != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			if len(accepted) > 1 {
				okAud := false
				for _, aud := range jt.Audience() {
					for _, a := range accepted {
						if aud == a {
							okAud = true
							break
						}
					}
				}
				if !okAud {
					http.Error(w, "aud_invalid", http.StatusUnauthorized)
					return
				}
			}
			// tenant ID claim enforcement (tid) optional
			if tid, ok := jt.Get("tid"); ok {
				if ts, _ := tid.(string); ts != "" && tenant.ID != "" && ts != tenant.ID {
					http.Error(w, "tenant_mismatch", http.StatusForbidden)
					return
				}
			}
			// scopes extraction
			var scopes []string
			if sc, ok := jt.Get("scope"); ok {
				scopes = append(scopes, strings.Fields(sc.(string))...)
			}
			// grant type for machine restrictions
			grantType := ""
			if gty, ok := jt.Get("gty"); ok {
				grantType, _ = gty.(string)
			}
			if grantType == "client_credentials" && len(tenant.MachineAllowedScopes) > 0 {
				allowed := map[string]struct{}{}
				for _, s := range tenant.MachineAllowedScopes {
					allowed[s] = struct{}{}
				}
				for _, s := range scopes {
					if _, ok := allowed[s]; !ok {
						http.Error(w, "scope_not_allowed_machine", http.StatusForbidden)
						return
					}
				}
			}
			// ACR per scope
			if len(tenant.RequiredACRByAction) > 0 {
				if a, ok := jt.Get("acr"); ok {
					acr, _ := a.(string)
					for scope, req := range tenant.RequiredACRByAction {
						if req != "" {
							for _, s := range scopes {
								if s == scope && acr != req {
									http.Error(w, "acr_required", http.StatusForbidden)
									return
								}
							}
						}
					}
				}
			}
			// Populate context
			ctx := WithScopes(r.Context(), scopes)
			ctx = context.WithValue(ctx, "jwt", jt)
			// TODO: DPoP verify & token hash binding
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GrantTypeFrom(ctx context.Context) string {
	if jt := tokenFromCtx(ctx); jt != nil {
		if gty, ok := jt.Get("gty"); ok {
			if s, _ := gty.(string); s != "" {
				return s
			}
		}
	}
	return ""
}

func ActorSub(ctx context.Context) string {
	if jt := tokenFromCtx(ctx); jt != nil {
		if sub, ok := jt.Get("sub"); ok {
			if s, _ := sub.(string); s != "" {
				return s
			}
		}
	}
	return ""
}

func tokenFromCtx(ctx context.Context) jwt.Token {
	if v := ctx.Value("jwt"); v != nil {
		if t, ok := v.(jwt.Token); ok {
			return t
		}
	}
	return nil
}

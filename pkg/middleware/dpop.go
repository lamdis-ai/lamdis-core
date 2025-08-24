// pkg/middleware/dpop.go
package middleware

import (
	"crypto"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"context"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jws"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/redis/go-redis/v9"
)

// Redis-backed DPoP verifier (per-tenant jti cache)
func dpopVerifyWithRedis(r *http.Request, access jwt.Token, rdb *redis.Client) error {
	proof := r.Header.Get("DPoP")
	if proof == "" {
		return errors.New("missing DPoP")
	}
	// Parse JWS to extract header (need jwk before we can verify)
	msg, err := jws.Parse([]byte(proof))
	if err != nil || len(msg.Signatures()) == 0 {
		return errors.New("bad DPoP")
	}
	h := msg.Signatures()[0].ProtectedHeaders()
	rawJWK, ok := h.Get("jwk")
	if !ok {
		return errors.New("missing jwk header")
	}
	b, _ := json.Marshal(rawJWK)
	key, err := jwk.ParseKey(b)
	if err != nil {
		return errors.New("bad jwk")
	}
	// derive alg from protected header (avoid key.Algorithm() signature differences)
	algName, ok := h.Get("alg")
	if !ok {
		return errors.New("missing alg")
	}
	alg := jwa.SignatureAlgorithm(fmt.Sprint(algName))

	// Parse & verify JWT with key
	pt, err := jwt.Parse([]byte(proof), jwt.WithKey(alg, key), jwt.WithValidate(true))
	if err != nil {
		return errors.New("bad DPoP")
	}
	// htm/htu binding
	htm, _ := pt.Get("htm")
	if !stringsEqual(htm, r.Method) {
		return errors.New("htm mismatch")
	}
	htu, _ := pt.Get("htu")
	if err := matchHTU(fmt.Sprint(htu), r.URL); err != nil {
		return err
	}
	// iat freshness
	iat := pt.IssuedAt()
	if iat.IsZero() || time.Since(iat) > 2*time.Minute {
		return errors.New("stale DPoP")
	}
	// jti replay
	jti, _ := pt.Get("jti")
	if rdb != nil {
		key := fmt.Sprintf("dpop:%s", jti)
		ok, _ := rdb.SetNX(context.Background(), key, 1, 5*time.Minute).Result()
		if !ok {
			return errors.New("replay")
		}
	}
	// cnf.jkt binding
	if cnf, ok := access.Get("cnf"); ok {
		if m, ok := cnf.(map[string]any); ok {
			if jkt, ok := m["jkt"].(string); ok {
				thumb, _ := key.Thumbprint(crypto.SHA256)
				if base64.RawURLEncoding.EncodeToString(thumb) != jkt {
					return errors.New("cnf mismatch")
				}
			}
		}
	}
	// ath optional
	if ath, ok := pt.Get("ath"); ok {
		h := sha256.Sum256([]byte(r.Header.Get("Authorization")[7:]))
		if base64.RawURLEncoding.EncodeToString(h[:]) != fmt.Sprint(ath) {
			return errors.New("ath mismatch")
		}
	}
	return nil
}

func matchHTU(htu string, u *url.URL) error {
	v, err := url.Parse(htu)
	if err != nil {
		return errors.New("bad htu")
	}
	if v.Path != u.Path {
		return fmt.Errorf("htu path mismatch")
	}
	return nil
}

func stringsEqual(a any, b string) bool { s, _ := a.(string); return s == b }

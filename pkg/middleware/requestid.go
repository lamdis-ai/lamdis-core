// pkg/middleware/requestid.go
package middleware

import (
	"context"
	"net/http"
	"github.com/google/uuid"
)

type ctxKey string

const CtxKeyRequestID ctxKey = "reqid"

func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-Id")
			if id == "" { id = uuid.NewString() }
			w.Header().Set("X-Request-Id", id)
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), CtxKeyRequestID, id)))
		})
	}
}
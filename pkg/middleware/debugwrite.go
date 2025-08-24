package middleware

import (
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"sync/atomic"
)

// DebugWriteHeader logs a stack trace if WriteHeader is called more than once.
// Enable by setting DEBUG_DOUBLE_WRITE=1 (or true/yes) in the environment.
func DebugWriteHeader() func(http.Handler) http.Handler {
	v := strings.ToLower(os.Getenv("DEBUG_DOUBLE_WRITE"))
	if v == "" || !(strings.HasPrefix(v, "1") || strings.HasPrefix(v, "t") || strings.HasPrefix(v, "y")) {
		return func(next http.Handler) http.Handler { return next }
	}
	log.Println("debug double-write middleware enabled")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			dw := &dwrapper{ResponseWriter: w, method: r.Method, path: r.URL.Path}
			next.ServeHTTP(dw, r)
		})
	}
}

type dwrapper struct {
	http.ResponseWriter
	wrote  int32
	method string
	path   string
	code   int
}

func (d *dwrapper) WriteHeader(code int) {
	if atomic.CompareAndSwapInt32(&d.wrote, 0, 1) {
		d.code = code
		d.ResponseWriter.WriteHeader(code)
		return
	}
	log.Printf("DOUBLE WriteHeader: %s %s first=%d second=%d\n%s", d.method, d.path, d.code, code, debug.Stack())
}

func (d *dwrapper) Write(b []byte) (int, error) {
	if atomic.LoadInt32(&d.wrote) == 0 {
		d.WriteHeader(http.StatusOK)
	}
	return d.ResponseWriter.Write(b)
}

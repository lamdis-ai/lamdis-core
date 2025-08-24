package adminapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	jmes "github.com/jmespath/go-jmespath"

	"lamdis/internal/facts"
)

// testJMESPath executes a JMESPath expression against a provided document
func (a *App) testJMESPath(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Doc  any    `json:"doc"`
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	if strings.TrimSpace(b.Path) == "" {
		http.Error(w, "missing path", 400)
		return
	}
	res, err := jmes.Search(b.Path, b.Doc)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()}, 200)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "result": res}, 200)
}

// Small helpers to avoid importing external packages repeatedly
func uuidNew() string { return uuid.New().String() }

func factsResolve(ctx context.Context, db *pgxpool.Pool, tid, action string, inputs map[string]any) (map[string]any, error) {
	return facts.ResolveFacts(ctx, db, tid, action, inputs)
}

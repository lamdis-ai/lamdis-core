package adminapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

func (a *App) getRegistry(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	cat := strings.TrimSpace(r.URL.Query().Get("category"))

	var rows pgx.Rows
	var err error
	if q == "" && cat == "" {
		rows, err = a.db.Query(r.Context(), `
			SELECT c.id, c.kind, c.display_name, COALESCE(c.category,''), COALESCE(c.tags, ARRAY[]::text[]),
				   c.capabilities, c.requirements, c.audit_mode,
				   COALESCE(tc.enabled,false) AS enabled, (tc.tenant_id IS NOT NULL) AS configured
			FROM connectors c
			LEFT JOIN tenant_connectors tc ON tc.tenant_id=$1 AND tc.connector_id=c.id
			ORDER BY c.id
		`, tid)
	} else {
		rows, err = a.db.Query(r.Context(), `
			SELECT c.id, c.kind, c.display_name, COALESCE(c.category,''), COALESCE(c.tags, ARRAY[]::text[]),
				   c.capabilities, c.requirements, c.audit_mode,
				   COALESCE(tc.enabled,false) AS enabled, (tc.tenant_id IS NOT NULL) AS configured
			FROM connectors c
			LEFT JOIN tenant_connectors tc ON tc.tenant_id=$1 AND tc.connector_id=c.id
			WHERE ($2='' OR c.category=$2)
			  AND ($3='' OR c.display_name ILIKE '%'||$3||'%' OR c.kind ILIKE '%'||$3||'%')
			ORDER BY c.id
		`, tid, cat, q)
	}
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var (
			id, kind, display, category, audit string
			tags                               []string
			capb, reqb                         []byte
			enabled, configured                bool
		)
		if err := rows.Scan(&id, &kind, &display, &category, &tags, &capb, &reqb, &audit, &enabled, &configured); err != nil {
			http.Error(w, "db error", 500)
			return
		}
		var caps []map[string]any
		var req map[string]any
		_ = json.Unmarshal(capb, &caps)
		_ = json.Unmarshal(reqb, &req)
		out = append(out, map[string]any{
			"id":           id,
			"kind":         kind,
			"display_name": display,
			"category":     category,
			"tags":         tags,
			"capabilities": caps,
			"requirements": req,
			"audit_mode":   audit,
			"enabled":      enabled,
			"configured":   configured,
		})
	}
	writeJSON(w, map[string]any{"connectors": out}, 200)
}

// upsertConnector creates or updates a connector spec in the registry table.
func (a *App) upsertConnector(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var b ConnectorSpec
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	if id == "" {
		id = b.ID
	}
	if id == "" || b.DisplayName == "" {
		http.Error(w, "missing id or display_name", 400)
		return
	}
	if b.Kind == "" {
		b.Kind = id
	}
	capb, _ := json.Marshal(b.Capabilities)
	reqb, _ := json.Marshal(b.Requirements)
	if _, err := a.db.Exec(r.Context(), `
		INSERT INTO connectors (id, kind, display_name, capabilities, requirements, audit_mode)
		VALUES ($1,$2,$3,$4,$5,COALESCE($6,'none'))
		ON CONFLICT (id) DO UPDATE SET
		  kind=EXCLUDED.kind,
		  display_name=EXCLUDED.display_name,
		  capabilities=EXCLUDED.capabilities,
		  requirements=EXCLUDED.requirements,
		  audit_mode=EXCLUDED.audit_mode,
		  updated_at=NOW()
	`, id, b.Kind, b.DisplayName, capb, reqb, b.AuditMode); err != nil {
		http.Error(w, "db error", 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "id": id}, 200)
}

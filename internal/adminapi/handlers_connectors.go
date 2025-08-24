package adminapi

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
)

type ConnectorBody struct {
	Enabled bool              `json:"enabled"`
	Secrets map[string]string `json:"secrets"`
}

func (a *App) putTenantConnector(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	cid := chi.URLParam(r, "connectorId")
	if cid == "" {
		http.Error(w, "missing connectorId", 400)
		return
	}

	var b ConnectorBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "bad json", 400)
		return
	}

	var enc []byte
	var err error
	if b.Secrets != nil {
		enc, err = a.encryptJSON(b.Secrets)
		if err != nil {
			http.Error(w, "encrypt", 500)
			return
		}
	}

	// Backward-compat: some deployments still have a legacy NOT NULL "kind" column.
	// If it exists, resolve kind and include it in the upsert to satisfy the constraint.
	var hasKind bool
	_ = a.db.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='tenant_connectors' AND column_name='kind')`).Scan(&hasKind)
	if hasKind {
		var kind string
		// Prefer custom definition mapping; fall back to builtin connectors table.
		_ = a.db.QueryRow(r.Context(), `SELECT kind FROM connector_definitions WHERE id::text=$1 AND tenant_id=$2`, cid, tid).Scan(&kind)
		if strings.TrimSpace(kind) == "" {
			_ = a.db.QueryRow(r.Context(), `SELECT kind FROM connectors WHERE id=$1`, cid).Scan(&kind)
		}
		_, err = a.db.Exec(r.Context(), `
			INSERT INTO tenant_connectors (tenant_id, connector_id, kind, enabled, secrets_encrypted)
			VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (tenant_id, connector_id) DO UPDATE SET
			  enabled=EXCLUDED.enabled,
			  secrets_encrypted=COALESCE(EXCLUDED.secrets_encrypted, tenant_connectors.secrets_encrypted),
			  updated_at=NOW()
		`, tid, cid, kind, b.Enabled, enc)
	} else {
		_, err = a.db.Exec(r.Context(), `
			INSERT INTO tenant_connectors (tenant_id, connector_id, enabled, secrets_encrypted)
			VALUES ($1,$2,$3,$4)
			ON CONFLICT (tenant_id, connector_id) DO UPDATE SET
			  enabled=EXCLUDED.enabled,
			  secrets_encrypted=COALESCE(EXCLUDED.secrets_encrypted, tenant_connectors.secrets_encrypted),
			  updated_at=NOW()
		`, tid, cid, b.Enabled, enc)
	}
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}

	// Auto-seed an action record when enabling a connector so /admin/actions isn't empty in dev
	if b.Enabled {
		// Try to derive a human-friendly display and action key from connector definition or registry
		var display string
		_ = a.db.QueryRow(r.Context(), `SELECT COALESCE(title, kind) FROM connector_definitions WHERE id::text=$1 AND tenant_id=$2`, cid, tid).Scan(&display)
		if strings.TrimSpace(display) == "" {
			_ = a.db.QueryRow(r.Context(), `SELECT COALESCE(display_name, kind) FROM connectors WHERE id=$1`, cid).Scan(&display)
		}
		if strings.TrimSpace(display) != "" {
			key := slugify(display)
			// Best-effort insert; ignore errors to avoid blocking the toggle path
			_, _ = a.db.Exec(r.Context(), `
				INSERT INTO actions(tenant_id, key, display_name)
				VALUES ($1,$2,$3)
				ON CONFLICT (tenant_id, key) DO NOTHING
			`, tid, key, display)
		}
	}

	writeJSON(w, map[string]any{"ok": true}, 200)
}

// ===== Marketplace: Custom connectors =====

type CustomConnectorBody struct {
	ID      string  `json:"id"`
	Display string  `json:"display_name"`
	Title   string  `json:"title"`
	Summary string  `json:"summary"`
	BaseURL string  `json:"base_url"`
	AuthRef *string `json:"auth_ref"`
	Enabled *bool   `json:"enabled"`
	// Accept legacy "operations" and new "actions" keys with identical shapes
	Operations []struct {
		Title       string           `json:"title"`
		Method      string           `json:"method"`
		Path        string           `json:"path"`
		Summary     string           `json:"summary"`
		Scopes      []string         `json:"scopes"`
		Params      []map[string]any `json:"params"`
		RequestTmpl map[string]any   `json:"request_tmpl"`
		Enabled     *bool            `json:"enabled"`
	} `json:"operations"`
	Actions []struct {
		Title       string           `json:"title"`
		Method      string           `json:"method"`
		Path        string           `json:"path"`
		Summary     string           `json:"summary"`
		Scopes      []string         `json:"scopes"`
		Params      []map[string]any `json:"params"`
		RequestTmpl map[string]any   `json:"request_tmpl"`
		Enabled     *bool            `json:"enabled"`
	} `json:"actions"`
}

func (a *App) listTenantConnectors(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	rows, err := a.db.Query(r.Context(), `
		SELECT d.id, d.kind, d.builtin_kind, d.config, d.base_url, d.auth_ref, d.title, d.summary,
				 COALESCE(tc.enabled,false) AS enabled
		FROM connector_definitions d
		LEFT JOIN tenant_connectors tc ON tc.tenant_id=$1 AND tc.connector_id=d.id::text
		WHERE d.tenant_id=$1
		ORDER BY d.kind`, tid)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	defer rows.Close()
	type Row struct {
		ID      string         `json:"id"`
		Kind    string         `json:"kind"`
		Builtin string         `json:"builtin_kind"`
		Config  map[string]any `json:"config"`
		BaseURL *string        `json:"base_url"`
		AuthRef *string        `json:"auth_ref"`
		Title   *string        `json:"title"`
		Summary *string        `json:"summary"`
		Enabled bool           `json:"enabled"`
	}
	var out []Row
	for rows.Next() {
		var id, kind, builtin string
		var cfgb []byte
		var baseURL *string
		var authRef *string
		var ctitle *string
		var csummary *string
		var enabled bool
		if err := rows.Scan(&id, &kind, &builtin, &cfgb, &baseURL, &authRef, &ctitle, &csummary, &enabled); err != nil {
			http.Error(w, "db error", 500)
			return
		}
		var cfg map[string]any
		_ = json.Unmarshal(cfgb, &cfg)
		out = append(out, Row{ID: id, Kind: kind, Builtin: builtin, Config: cfg, BaseURL: baseURL, AuthRef: authRef, Title: ctitle, Summary: csummary, Enabled: enabled})
	}
	writeJSON(w, map[string]any{"items": out}, 200)
}

func (a *App) createCustomConnector(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	var b CustomConnectorBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	if b.Display == "" || b.BaseURL == "" {
		http.Error(w, "missing fields", 400)
		return
	}
	defID := uuidNew()
	if _, err := a.db.Exec(r.Context(), `INSERT INTO connector_definitions(id,tenant_id,kind,builtin_kind,auth,config,secret,base_url,auth_ref,title,summary) VALUES ($1,$2,$3,'', '{}'::jsonb, '{}'::jsonb, '{}'::jsonb, $4, $5, $6, $7)`, defID, tid, b.Display, b.BaseURL, b.AuthRef, nullIfEmpty(b.Title), nullIfEmpty(b.Summary)); err != nil {
		http.Error(w, "db error", 500)
		return
	}
	// Prefer actions if provided; fallback to operations
	list := b.Actions
	if len(list) == 0 {
		list = b.Operations
	}
	for _, op := range list {
		enabled := true
		if op.Enabled != nil {
			enabled = *op.Enabled
		}
		_, _ = a.db.Exec(r.Context(), `INSERT INTO connector_operations(id,connector_id,method,path,summary,scopes,request_tmpl,params,enabled) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, uuidNew(), defID, strings.ToUpper(op.Method), op.Path, op.Summary, op.Scopes, op.RequestTmpl, op.Params, enabled)
	}
	if b.Enabled != nil && *b.Enabled {
		// Legacy compatibility: populate 'kind' if the column exists to satisfy NOT NULL/PK variants
		var hasKind bool
		_ = a.db.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='tenant_connectors' AND column_name='kind')`).Scan(&hasKind)
		if hasKind {
			var kind string
			_ = a.db.QueryRow(r.Context(), `SELECT kind FROM connector_definitions WHERE id=$1 AND tenant_id=$2`, defID, tid).Scan(&kind)
			_, _ = a.db.Exec(r.Context(), `INSERT INTO tenant_connectors(tenant_id,connector_id,kind,enabled) VALUES ($1::uuid,$2,$3,true) ON CONFLICT (tenant_id,connector_id) DO UPDATE SET enabled=true, updated_at=NOW()`, tid, defID, kind)
		} else {
			_, _ = a.db.Exec(r.Context(), `INSERT INTO tenant_connectors(tenant_id,connector_id,enabled) VALUES ($1::uuid,$2,true) ON CONFLICT (tenant_id,connector_id) DO UPDATE SET enabled=true, updated_at=NOW()`, tid, defID)
		}
		// Seed a corresponding action entry (idempotent)
		disp := strings.TrimSpace(b.Title)
		if disp == "" {
			disp = strings.TrimSpace(b.Display)
		}
		if disp != "" {
			_, _ = a.db.Exec(r.Context(), `INSERT INTO actions(tenant_id, key, display_name) VALUES ($1::uuid,$2,$3) ON CONFLICT (tenant_id, key) DO NOTHING`, tid, slugify(disp), disp)
		}
	}
	writeJSON(w, map[string]any{"ok": true, "id": defID}, 201)
}

func (a *App) updateCustomConnector(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	id := chi.URLParam(r, "id")
	var b CustomConnectorBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	var builtin string
	_ = a.db.QueryRow(r.Context(), `SELECT COALESCE(builtin_kind,'') FROM connector_definitions WHERE id=$1 AND tenant_id=$2`, id, tid).Scan(&builtin)
	if strings.TrimSpace(builtin) != "" {
		http.Error(w, "builtin_connectors_are_readonly", http.StatusForbidden)
		return
	}
	_, err := a.db.Exec(r.Context(), `UPDATE connector_definitions SET kind=COALESCE($1,kind), base_url=COALESCE($2,base_url), auth_ref=COALESCE($3,auth_ref), title=COALESCE($4,title), summary=COALESCE($5,summary) WHERE id=$6 AND tenant_id=$7`, nullIfEmpty(b.Display), nullIfEmpty(b.BaseURL), b.AuthRef, nullIfEmpty(b.Title), nullIfEmpty(b.Summary), id, tid)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	if b.Enabled != nil {
		var hasKind bool
		_ = a.db.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='tenant_connectors' AND column_name='kind')`).Scan(&hasKind)
		if hasKind {
			// We know this row is for a custom connector definition -> its kind lives on the definition
			var kind string
			_ = a.db.QueryRow(r.Context(), `SELECT kind FROM connector_definitions WHERE id=$1 AND tenant_id=$2`, id, tid).Scan(&kind)
			_, _ = a.db.Exec(r.Context(), `INSERT INTO tenant_connectors(tenant_id,connector_id,kind,enabled) VALUES ($1::uuid,$2,$3,$4) ON CONFLICT (tenant_id,connector_id) DO UPDATE SET enabled=EXCLUDED.enabled, updated_at=NOW()`, tid, id, kind, *b.Enabled)
		} else {
			_, _ = a.db.Exec(r.Context(), `INSERT INTO tenant_connectors(tenant_id,connector_id,enabled) VALUES ($1::uuid,$2,$3) ON CONFLICT (tenant_id,connector_id) DO UPDATE SET enabled=EXCLUDED.enabled, updated_at=NOW()`, tid, id, *b.Enabled)
		}
		// If enabling, ensure a simple action record exists keyed from title/display
		if *b.Enabled {
			disp := strings.TrimSpace(b.Title)
			if disp == "" {
				disp = strings.TrimSpace(b.Display)
			}
			if disp == "" {
				_ = a.db.QueryRow(r.Context(), `SELECT COALESCE(title, kind) FROM connector_definitions WHERE id=$1 AND tenant_id=$2`, id, tid).Scan(&disp)
			}
			if strings.TrimSpace(disp) != "" {
				_, _ = a.db.Exec(r.Context(), `INSERT INTO actions(tenant_id, key, display_name) VALUES ($1::uuid,$2,$3) ON CONFLICT (tenant_id, key) DO NOTHING`, tid, slugify(disp), disp)
			}
		}
	}
	// Prefer actions if provided; fallback to operations
	list := b.Actions
	if len(list) == 0 {
		list = b.Operations
	}
	if len(list) > 0 {
		_, _ = a.db.Exec(r.Context(), `DELETE FROM connector_operations WHERE connector_id=$1`, id)
		for _, op := range list {
			enabled := true
			if op.Enabled != nil {
				enabled = *op.Enabled
			}
			_, _ = a.db.Exec(r.Context(), `INSERT INTO connector_operations(id,connector_id,method,path,summary,scopes,request_tmpl,params,enabled) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, uuidNew(), id, strings.ToUpper(op.Method), op.Path, op.Summary, op.Scopes, op.RequestTmpl, op.Params, enabled)
		}
	}
	writeJSON(w, map[string]any{"ok": true}, 200)
}

func (a *App) listConfiguredConnectors(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	rows, err := a.db.Query(r.Context(), `
		SELECT src, id, kind, display_name, requirements, enabled
		FROM (
		  SELECT 'builtin'::text AS src, c.id::text AS id, c.kind, c.display_name, c.requirements, tc.enabled
		  FROM connectors c
		  JOIN tenant_connectors tc ON tc.connector_id=c.id AND tc.tenant_id=$1
		  UNION ALL
		  SELECT 'custom'::text AS src, d.id::text AS id, d.kind, COALESCE(d.title,d.kind) AS display_name, '{}'::jsonb AS requirements,
				 COALESCE(tc.enabled,false) AS enabled
		  FROM connector_definitions d
		  LEFT JOIN tenant_connectors tc ON tc.connector_id=d.id::text AND tc.tenant_id=$1
		  WHERE d.tenant_id=$1
		) x
		ORDER BY kind
	`, tid)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	defer rows.Close()
	type Row struct {
		Src, ID, Kind, Display string
		Reqb                   []byte
		Enabled                bool
	}
	var out []map[string]any
	for rows.Next() {
		var rec Row
		if err := rows.Scan(&rec.Src, &rec.ID, &rec.Kind, &rec.Display, &rec.Reqb, &rec.Enabled); err != nil {
			http.Error(w, "db error", 500)
			return
		}
		var req map[string]any
		_ = json.Unmarshal(rec.Reqb, &req)
		out = append(out, map[string]any{
			"id":           rec.ID,
			"type":         rec.Src,
			"kind":         rec.Kind,
			"display_name": rec.Display,
			"requirements": req,
			"enabled":      rec.Enabled,
		})
	}
	writeJSON(w, map[string]any{"items": out}, 200)
}

func (a *App) getCustomConnector(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	id := chi.URLParam(r, "id")
	// Quick sanity: ensure the provided id belongs to a custom definition for this tenant
	// Using id::text for robust comparison when URL param is text.
	var row struct {
		ID, Kind string
		BaseURL  *string
		AuthRef  *string
		Title    *string
		Summary  *string
	}
	err := a.db.QueryRow(r.Context(), `
		SELECT id::text, kind, base_url, auth_ref, title, summary
		FROM connector_definitions WHERE id::text=$1 AND tenant_id=$2
	`, id, tid).Scan(&row.ID, &row.Kind, &row.BaseURL, &row.AuthRef, &row.Title, &row.Summary)
	if err != nil {
		// Do not leak tenant info; but give a clearer message for debugging
		http.Error(w, "custom connector not found for tenant", 404)
		return
	}
	opr, err := a.db.Query(r.Context(), `
		SELECT id::text, method, path, summary, COALESCE(scopes, ARRAY[]::text[]), COALESCE(params,'[]'::jsonb), COALESCE(request_tmpl,'{}'::jsonb), COALESCE(enabled,true)
		FROM connector_operations WHERE connector_id::text=$1 ORDER BY id
	`, id)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	defer opr.Close()
	ops := []map[string]any{}
	for opr.Next() {
		var (
			opID                  string
			method, path, summary string
			scopes                []string
			paramsRaw, tmplRaw    []byte
			opEnabled             bool
		)
		if err := opr.Scan(&opID, &method, &path, &summary, &scopes, &paramsRaw, &tmplRaw, &opEnabled); err != nil {
			http.Error(w, "db error", 500)
			return
		}
		var params []map[string]any
		var tmpl map[string]any
		_ = json.Unmarshal(paramsRaw, &params)
		_ = json.Unmarshal(tmplRaw, &tmpl)
		ops = append(ops, map[string]any{
			"id":           opID,
			"method":       method,
			"path":         path,
			"summary":      summary,
			"scopes":       scopes,
			"params":       params,
			"request_tmpl": tmpl,
			"enabled":      opEnabled,
		})
	}
	writeJSON(w, map[string]any{
		"id":           row.ID,
		"display_name": row.Kind,
		"title":        row.Title,
		"summary":      row.Summary,
		"base_url":     row.BaseURL,
		"auth_ref":     row.AuthRef,
		// Return new key "actions"; keep "operations" for compatibility
		"actions":    ops,
		"operations": ops,
	}, 200)
}

// putConnectorAction enables/disables a single connector operation for a custom connector
func (a *App) putConnectorAction(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	cid := chi.URLParam(r, "id")
	opid := chi.URLParam(r, "opId")
	if cid == "" || opid == "" {
		http.Error(w, "missing ids", 400)
		return
	}
	// Ensure the operation belongs to the tenant's connector definition
	var exists bool
	_ = a.db.QueryRow(r.Context(), `SELECT EXISTS (
		SELECT 1 FROM connector_operations o JOIN connector_definitions d ON o.connector_id=d.id
		WHERE o.id::text=$1 AND d.id::text=$2 AND d.tenant_id=$3
	)`, opid, cid, tid).Scan(&exists)
	if !exists {
		http.Error(w, "not found", 404)
		return
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	if _, err := a.db.Exec(r.Context(), `UPDATE connector_operations SET enabled=$1 WHERE id::text=$2`, body.Enabled, opid); err != nil {
		http.Error(w, "db error", 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true}, 200)
}

// listConnectorActions returns only enabled operations (actions) for a custom connector
func (a *App) listConnectorActions(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	cid := chi.URLParam(r, "id")
	if strings.TrimSpace(cid) == "" {
		http.Error(w, "missing id", 400)
		return
	}
	// Ensure the connector belongs to tenant and fetch a nice display label
	var display string
	if err := a.db.QueryRow(r.Context(), `SELECT COALESCE(title, kind, id::text) FROM connector_definitions WHERE id::text=$1 AND tenant_id=$2`, cid, tid).Scan(&display); err != nil {
		http.Error(w, "not found", 404)
		return
	}
	rows, err := a.db.Query(r.Context(), `
		SELECT o.id::text, o.method, o.path, o.summary,
			   COALESCE(o.scopes, ARRAY[]::text[]),
			   COALESCE(o.params,'[]'::jsonb),
			   COALESCE(o.request_tmpl,'{}'::jsonb)
		FROM connector_operations o
		JOIN connector_definitions d ON o.connector_id=d.id
		WHERE d.id::text=$1 AND d.tenant_id=$2 AND COALESCE(o.enabled,true)=true
		ORDER BY o.id
	`, cid, tid)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	defer rows.Close()
	type Item struct {
		ID          string           `json:"id"`
		Method      string           `json:"method"`
		Path        string           `json:"path"`
		Summary     string           `json:"summary"`
		Scopes      []string         `json:"scopes"`
		Params      []map[string]any `json:"params"`
		RequestTmpl map[string]any   `json:"request_tmpl"`
	}
	var out []Item
	for rows.Next() {
		var (
			id, method, path, summary string
			scopes                    []string
			paramsRaw, tmplRaw        []byte
		)
		if err := rows.Scan(&id, &method, &path, &summary, &scopes, &paramsRaw, &tmplRaw); err != nil {
			http.Error(w, "db error", 500)
			return
		}
		var params []map[string]any
		var tmpl map[string]any
		_ = json.Unmarshal(paramsRaw, &params)
		_ = json.Unmarshal(tmplRaw, &tmpl)
		out = append(out, Item{ID: id, Method: method, Path: path, Summary: summary, Scopes: scopes, Params: params, RequestTmpl: tmpl})
	}
	writeJSON(w, map[string]any{
		"connector": map[string]any{"id": cid, "display": display},
		"items":     out,
	}, 200)
}

// Helpers to avoid importing database/sql here
type sqlNullString struct {
	Valid  bool
	String string
}

func (s sqlNullString) Value() any {
	if s.Valid {
		return s.String
	}
	return nil
}

// slugify produces a lowercase dash-separated key from a display string.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return s
	}
	// Replace non-alphanumeric with dash
	re := regexp.MustCompile(`[^a-z0-9]+`)
	s = re.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "action"
	}
	return s
}

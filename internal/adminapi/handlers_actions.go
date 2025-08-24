package adminapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type UpsertActionBody struct {
	DisplayName  *string        `json:"display_name"`
	InputsSchema map[string]any `json:"inputs_schema"`
}

func (a *App) listActions(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	rows, err := a.db.Query(r.Context(), `WITH s AS (SELECT set_config('app.tenant_id', $1, true)) SELECT key, display_name, inputs_schema, updated_at FROM actions ORDER BY key`, tid)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	defer rows.Close()
	type Row struct {
		Key, DisplayName string
		InputsSchema     map[string]any
		UpdatedAt        time.Time
	}
	out := []Row{}
	for rows.Next() {
		var rkey, disp string
		var js []byte
		var upd time.Time
		if err := rows.Scan(&rkey, &disp, &js, &upd); err != nil {
			http.Error(w, "db error", 500)
			return
		}
		var schema map[string]any
		_ = json.Unmarshal(js, &schema)
		out = append(out, Row{Key: rkey, DisplayName: disp, InputsSchema: schema, UpdatedAt: upd})
	}
	writeJSON(w, map[string]any{"items": out}, 200)
}

func (a *App) upsertAction(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	key := chi.URLParam(r, "key")
	var b UpsertActionBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	_, err := a.db.Exec(r.Context(), `WITH s AS (SELECT set_config('app.tenant_id', $1, true))
		INSERT INTO actions(tenant_id, key, display_name, inputs_schema) VALUES ($1::uuid,$2,COALESCE($3,''),COALESCE($4,'{}'::jsonb))
		ON CONFLICT (tenant_id, key) DO UPDATE SET display_name=COALESCE($3,actions.display_name), inputs_schema=COALESCE($4,actions.inputs_schema), updated_at=NOW()`, tid, key, b.DisplayName, b.InputsSchema)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true}, 200)
}

type UpsertResolverBody struct {
	ConnectorKey   *string          `json:"connector_key"`
	RequestTmpl    map[string]any   `json:"request_template"`
	ResponseSample map[string]any   `json:"response_sample"`
	Needs          []map[string]any `json:"needs"`
	Enabled        *bool            `json:"enabled"`
}

func (a *App) listResolvers(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	key := chi.URLParam(r, "key")
	rows, err := a.db.Query(r.Context(), `WITH s AS (SELECT set_config('app.tenant_id', $1, true))
		SELECT name, connector_key, COALESCE(request_template,'{}')::jsonb, COALESCE(response_sample,'{}')::jsonb, COALESCE(needs,'[]')::jsonb, enabled, updated_at
		FROM fact_resolvers WHERE action_key=$2 ORDER BY name`, tid, key)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	defer rows.Close()
	type Row struct {
		Name, ConnectorKey              string
		RequestTemplate, ResponseSample any
		Needs                           any
		Enabled                         bool
		UpdatedAt                       time.Time
	}
	out := []Row{}
	for rows.Next() {
		var name, ck string
		var rraw, sraw, nraw []byte
		var enabled bool
		var upd time.Time
		if err := rows.Scan(&name, &ck, &rraw, &sraw, &nraw, &enabled, &upd); err != nil {
			http.Error(w, "db error", 500)
			return
		}
		var rt, rs, nd any
		_ = json.Unmarshal(rraw, &rt)
		_ = json.Unmarshal(sraw, &rs)
		_ = json.Unmarshal(nraw, &nd)
		out = append(out, Row{Name: name, ConnectorKey: ck, RequestTemplate: rt, ResponseSample: rs, Needs: nd, Enabled: enabled, UpdatedAt: upd})
	}
	writeJSON(w, map[string]any{"items": out}, 200)
}

func (a *App) upsertResolver(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	key := chi.URLParam(r, "key")
	name := chi.URLParam(r, "name")
	var b UpsertResolverBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	_, err := a.db.Exec(r.Context(), `WITH s AS (SELECT set_config('app.tenant_id', $1, true))
		INSERT INTO fact_resolvers(tenant_id, action_key, name, connector_key, request_template, response_sample, needs, enabled)
		VALUES ($1,$2,$3,$4,$5,$6,$7,COALESCE($8,true))
		ON CONFLICT (tenant_id, action_key, name) DO UPDATE SET
		  connector_key=COALESCE($4,fact_resolvers.connector_key),
		  request_template=COALESCE($5,fact_resolvers.request_template),
		  response_sample=COALESCE($6,fact_resolvers.response_sample),
		  needs=COALESCE($7,fact_resolvers.needs),
		  enabled=COALESCE($8,fact_resolvers.enabled),
		  updated_at=NOW()`, tid, key, name, b.ConnectorKey, b.RequestTmpl, b.ResponseSample, b.Needs, b.Enabled)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true}, 200)
}

func (a *App) deleteResolver(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	key := chi.URLParam(r, "key")
	name := chi.URLParam(r, "name")
	_, err := a.db.Exec(r.Context(), `WITH s AS (SELECT set_config('app.tenant_id', $1, true)) DELETE FROM fact_resolvers WHERE action_key=$2 AND name=$3`, tid, key, name)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true}, 200)
}

type UpsertMappingBody struct {
	JMESPath      string  `json:"jmespath"`
	FactKey       string  `json:"fact_key"`
	Transform     *string `json:"transform"`
	TransformArgs []any   `json:"transform_args"`
	Required      *bool   `json:"required"`
}

func (a *App) listMappings(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	key := chi.URLParam(r, "key")
	rows, err := a.db.Query(r.Context(), `WITH s AS (SELECT set_config('app.tenant_id', $1, true))
		SELECT name, jmespath, fact_key, COALESCE(transform,''), COALESCE(transform_args,'[]')::jsonb, required, updated_at
		FROM fact_mappings WHERE action_key=$2 ORDER BY name`, tid, key)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	defer rows.Close()
	type Row struct {
		Name, JMESPath, FactKey, Transform string
		TransformArgs                      any
		Required                           bool
		UpdatedAt                          time.Time
	}
	out := []Row{}
	for rows.Next() {
		var n, p, fk, tr string
		var argsRaw []byte
		var req bool
		var upd time.Time
		if err := rows.Scan(&n, &p, &fk, &tr, &argsRaw, &req, &upd); err != nil {
			http.Error(w, "db error", 500)
			return
		}
		var args any
		_ = json.Unmarshal(argsRaw, &args)
		out = append(out, Row{Name: n, JMESPath: p, FactKey: fk, Transform: tr, TransformArgs: args, Required: req, UpdatedAt: upd})
	}
	writeJSON(w, map[string]any{"items": out}, 200)
}

func (a *App) upsertMapping(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	key := chi.URLParam(r, "key")
	name := chi.URLParam(r, "name")
	var b UpsertMappingBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	_, err := a.db.Exec(r.Context(), `WITH s AS (SELECT set_config('app.tenant_id', $1, true))
		INSERT INTO fact_mappings(tenant_id, action_key, name, jmespath, fact_key, transform, transform_args, required)
		VALUES ($1,$2,$3,$4,$5,$6,$7,COALESCE($8,false))
		ON CONFLICT (tenant_id, action_key, name) DO UPDATE SET
		  jmespath=COALESCE($4,fact_mappings.jmespath),
		  fact_key=COALESCE($5,fact_mappings.fact_key),
		  transform=COALESCE($6,fact_mappings.transform),
		  transform_args=COALESCE($7,fact_mappings.transform_args),
		  required=COALESCE($8,fact_mappings.required),
		  updated_at=NOW()`, tid, key, name, b.JMESPath, b.FactKey, b.Transform, b.TransformArgs, b.Required)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true}, 200)
}

func (a *App) deleteMapping(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	key := chi.URLParam(r, "key")
	name := chi.URLParam(r, "name")
	_, err := a.db.Exec(r.Context(), `WITH s AS (SELECT set_config('app.tenant_id', $1, true)) DELETE FROM fact_mappings WHERE action_key=$2 AND name=$3`, tid, key, name)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true}, 200)
}

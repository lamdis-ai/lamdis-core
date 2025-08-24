package adminapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type AuthBody struct {
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	Config  map[string]any    `json:"config"`
	Secrets map[string]string `json:"secrets"`
}

func (a *App) listAuth(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	rows, err := a.db.Query(r.Context(), `SELECT id, name, type, config, updated_at FROM tenant_auth_configs WHERE tenant_id=$1 ORDER BY name`, tid)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	defer rows.Close()
	type Row struct {
		ID, Name, Type string
		Config         map[string]any
		UpdatedAt      time.Time
	}
	var out []Row
	for rows.Next() {
		var id, name, typ string
		var cfgb []byte
		var ts time.Time
		if err := rows.Scan(&id, &name, &typ, &cfgb, &ts); err != nil {
			http.Error(w, "db error", 500)
			return
		}
		var cfg map[string]any
		_ = json.Unmarshal(cfgb, &cfg)
		out = append(out, Row{ID: id, Name: name, Type: typ, Config: cfg, UpdatedAt: ts})
	}
	writeJSON(w, map[string]any{"items": out}, 200)
}

func (a *App) createAuth(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	var b AuthBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	if b.Name == "" || b.Type == "" {
		http.Error(w, "missing fields", 400)
		return
	}
	enc, err := a.encryptJSON(b.Secrets)
	if err != nil {
		http.Error(w, "encrypt", 500)
		return
	}
	id := uuidNew()
	if _, err := a.db.Exec(r.Context(), `INSERT INTO tenant_auth_configs(id,tenant_id,name,type,config,secrets_encrypted) VALUES ($1,$2::uuid,$3,$4,$5,$6)`, id, tid, b.Name, b.Type, b.Config, enc); err != nil {
		http.Error(w, "db error", 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "id": id}, 201)
}

func (a *App) updateAuth(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	id := chi.URLParam(r, "id")
	var b AuthBody
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
	_, err = a.db.Exec(r.Context(), `UPDATE tenant_auth_configs SET name=COALESCE($1,name), type=COALESCE($2,type), config=COALESCE($3,config), secrets_encrypted=COALESCE($4,secrets_encrypted), updated_at=NOW() WHERE id=$5 AND tenant_id=$6`, nullIfEmpty(b.Name), nullIfEmpty(b.Type), b.Config, enc, id, tid)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true}, 200)
}

func (a *App) deleteAuth(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	id := chi.URLParam(r, "id")
	_, err := a.db.Exec(r.Context(), `DELETE FROM tenant_auth_configs WHERE id=$1 AND tenant_id=$2`, id, tid)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true}, 200)
}

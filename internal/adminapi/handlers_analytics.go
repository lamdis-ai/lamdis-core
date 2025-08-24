package adminapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

func (a *App) getAudit(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	rows, err := a.db.Query(r.Context(), `
        SELECT action, rail, order_id, actor_sub, mode, result_code, request_id, ts
        FROM audit_log WHERE tenant_id=$1
        ORDER BY ts DESC LIMIT 100
    `, tid)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	defer rows.Close()
	type rec struct {
		Action, Rail, OrderID, ActorSub, Mode, RequestID string
		ResultCode                                       int
		TS                                               time.Time
	}
	var out []rec
	for rows.Next() {
		var x rec
		if err := rows.Scan(&x.Action, &x.Rail, &x.OrderID, &x.ActorSub, &x.Mode, &x.ResultCode, &x.RequestID, &x.TS); err != nil {
			http.Error(w, "db error", 500)
			return
		}
		out = append(out, x)
	}
	writeJSON(w, map[string]any{"items": out}, 200)
}

func (a *App) getUsageSummary(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	rows, err := a.db.Query(r.Context(), `
		SELECT date_trunc('day', started_at) AS day,
			   COALESCE(NULLIF(action_id,''), method || ' ' || path) AS action,
			   COUNT(*) AS count,
			   COALESCE(AVG(duration_ms)::int,0) AS avg_ms,
			   SUM(CASE WHEN status_code BETWEEN 200 AND 299 THEN 1 ELSE 0 END) AS ok
		FROM usage_events
		WHERE tenant_id = $1
		GROUP BY 1,2
		ORDER BY 1 DESC, 2
	`, tid)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	defer rows.Close()
	type Row struct {
		Day              time.Time `json:"day"`
		Action           string    `json:"action"`
		Count, AvgMs, Ok int
	}
	out := []Row{}
	for rows.Next() {
		var x Row
		if err := rows.Scan(&x.Day, &x.Action, &x.Count, &x.AvgMs, &x.Ok); err != nil {
			http.Error(w, "db error", 500)
			return
		}
		out = append(out, x)
	}
	var total, okcnt, avgms int
	_ = a.db.QueryRow(r.Context(), `
		SELECT COUNT(*), SUM(CASE WHEN status_code BETWEEN 200 AND 299 THEN 1 ELSE 0 END), COALESCE(AVG(duration_ms)::int,0)
		FROM usage_events WHERE tenant_id=$1
	`, tid).Scan(&total, &okcnt, &avgms)
	writeJSON(w, map[string]any{"totals": map[string]any{"count": total, "ok": okcnt, "avg_ms": avgms}, "daily": out}, 200)
}

func (a *App) getActionsCoverage(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	rows, err := a.db.Query(r.Context(), `WITH s AS (SELECT set_config('app.tenant_id', $1, true))
		SELECT a.key, COALESCE(a.display_name,''),
			   COUNT(DISTINCT fr.id) AS resolvers,
			   COUNT(DISTINCT fm.id) AS mappings,
			   SUM(CASE WHEN COALESCE(fm.required,false) THEN 1 ELSE 0 END) AS required_mappings
		FROM actions a
		LEFT JOIN fact_resolvers fr ON fr.tenant_id=a.tenant_id AND fr.action_key=a.key AND fr.enabled
		LEFT JOIN fact_mappings fm ON fm.tenant_id=a.tenant_id AND fm.action_key=a.key
		GROUP BY a.key, a.display_name
		ORDER BY a.key`, tid)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	defer rows.Close()
	type Row struct {
		Key, DisplayName                      string
		Resolvers, Mappings, RequiredMappings int
		GuardrailOK                           bool
	}
	out := []Row{}
	for rows.Next() {
		var v Row
		if err := rows.Scan(&v.Key, &v.DisplayName, &v.Resolvers, &v.Mappings, &v.RequiredMappings); err != nil {
			http.Error(w, "db error", 500)
			return
		}
		v.GuardrailOK = v.RequiredMappings > 0
		out = append(out, v)
	}
	writeJSON(w, map[string]any{"items": out}, 200)
}

func (a *App) getActionsSummary(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	rows, err := a.db.Query(r.Context(), `WITH s AS (SELECT set_config('app.tenant_id', $1, true))
		SELECT status, COUNT(*) FROM decisions WHERE created_at > NOW() - INTERVAL '7 days' GROUP BY status`, tid)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	defer rows.Close()
	type Row struct {
		Status string `json:"status"`
		Count  int    `json:"count"`
	}
	out := []Row{}
	var total, allowed int
	for rows.Next() {
		var s string
		var c int
		if err := rows.Scan(&s, &c); err != nil {
			http.Error(w, "db error", 500)
			return
		}
		out = append(out, Row{Status: s, Count: c})
		total += c
		if s == "ALLOW" || s == "ALLOW_WITH_CONDITIONS" {
			allowed += c
		}
	}
	rate := 0.0
	if total > 0 {
		rate = float64(allowed) / float64(total)
	}
	writeJSON(w, map[string]any{"items": out, "allowed_rate": rate, "total": total}, 200)
}

// listDecisions returns recent policy decisions (auditing) with simple pagination.
func (a *App) listDecisions(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	// Parse limit & cursor (created_at desc, id tie-breaker) for naive pagination.
	q := r.URL.Query()
	limit := 100
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	after := q.Get("after") // expecting RFC3339 timestamp
	var rows pgx.Rows
	var err error
	if after != "" {
		if ts, e := time.Parse(time.RFC3339, after); e == nil {
			rows, err = a.db.Query(r.Context(), `WITH s AS (SELECT set_config('app.tenant_id', $1, true))
				SELECT id::text, action_key, status, policy_version, expires_at, created_at
				FROM decisions WHERE created_at < $2
				ORDER BY created_at DESC, id DESC LIMIT $3`, tid, ts, limit)
		}
	}
	if rows == nil && err == nil { // initial or parse fail fallback
		rows, err = a.db.Query(r.Context(), `WITH s AS (SELECT set_config('app.tenant_id', $1, true))
			SELECT id::text, action_key, status, policy_version, expires_at, created_at
			FROM decisions ORDER BY created_at DESC, id DESC LIMIT $2`, tid, limit)
	}
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	defer rows.Close()
	type Row struct {
		ID            string     `json:"id"`
		ActionKey     string     `json:"action_key"`
		Status        string     `json:"status"`
		PolicyVersion int        `json:"policy_version"`
		ExpiresAt     *time.Time `json:"expires_at"`
		CreatedAt     time.Time  `json:"created_at"`
	}
	out := []Row{}
	var last *time.Time
	for rows.Next() {
		var x Row
		if err := rows.Scan(&x.ID, &x.ActionKey, &x.Status, &x.PolicyVersion, &x.ExpiresAt, &x.CreatedAt); err != nil {
			http.Error(w, "db error", 500)
			return
		}
		out = append(out, x)
		if last == nil || x.CreatedAt.Before(*last) {
			tmp := x.CreatedAt
			last = &tmp
		}
	}
	next := ""
	if last != nil {
		next = last.UTC().Format(time.RFC3339)
	}
	writeJSON(w, map[string]any{"items": out, "next_after": next}, 200)
}

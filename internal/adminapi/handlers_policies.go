package adminapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lamdis/internal/orchestrator"
	pol "lamdis/internal/policy"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/open-policy-agent/opa/rego"
)

type PoliciesBody struct {
	MachineAllowedScopes []string       `json:"machine_allowed_scopes"`
	StepUp               map[string]any `json:"step_up"`
}

func (a *App) putPolicies(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	var b PoliciesBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "bad json", 400)
		return
	}

	_, err := a.db.Exec(r.Context(), `
				INSERT INTO policies (tenant_id, machine_allowed_scopes, step_up)
				VALUES ($1::uuid,$2,$3)
				ON CONFLICT (tenant_id) DO UPDATE SET
					machine_allowed_scopes=EXCLUDED.machine_allowed_scopes,
					step_up=EXCLUDED.step_up,
					updated_at=NOW()
		`, tid, b.MachineAllowedScopes, b.StepUp)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}

	writeJSON(w, map[string]any{"ok": true}, 200)
}

type PolicyVersionBody struct {
	ActionKey string `json:"action_key"`
	Version   *int   `json:"version"`
	Code      string `json:"code"`
}

// getPolicyVersion returns the code and status for a specific version
func (a *App) getPolicyVersion(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	actionKey := strings.TrimSpace(r.URL.Query().Get("action_key"))
	if actionKey == "" {
		http.Error(w, "missing action_key", 400)
		return
	}
	verStr := chi.URLParam(r, "version")
	if strings.TrimSpace(verStr) == "" {
		http.Error(w, "missing version", 400)
		return
	}
	verInt, err := strconv.Atoi(verStr)
	if err != nil {
		http.Error(w, "bad version", 400)
		return
	}
	var code string
	var status string
	err = a.db.QueryRow(r.Context(), `WITH s AS (SELECT set_config('app.tenant_id', $1, true))
		SELECT COALESCE(compiled_rego,''), COALESCE(status,'') FROM policy_versions WHERE action_key=$2 AND version=$3`, tid, actionKey, verInt).Scan(&code, &status)
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	writeJSON(w, map[string]any{"version": verInt, "status": status, "code": code}, 200)
}

// getActivePolicy returns the currently published policy version (latest published)
func (a *App) getActivePolicy(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	actionKey := strings.TrimSpace(r.URL.Query().Get("action_key"))
	if actionKey == "" {
		http.Error(w, "missing action_key", 400)
		return
	}
	var code string
	var ver int
	err := a.db.QueryRow(r.Context(), `WITH s AS (SELECT set_config('app.tenant_id', $1, true))
		SELECT COALESCE(compiled_rego,''), COALESCE(version,0) FROM policy_versions WHERE action_key=$2 AND status='published' ORDER BY version DESC LIMIT 1`, tid, actionKey).Scan(&code, &ver)
	if err != nil || ver == 0 {
		// no active policy yet -> return empty object
		writeJSON(w, map[string]any{"version": 0, "status": "none", "code": ""}, 200)
		return
	}
	writeJSON(w, map[string]any{"version": ver, "status": "published", "code": code}, 200)
}

func (a *App) createPolicyVersion(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	var b PolicyVersionBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || strings.TrimSpace(b.Code) == "" || strings.TrimSpace(b.ActionKey) == "" {
		http.Error(w, "bad json", 400)
		return
	}
	if _, err := rego.New(
		rego.Query("data.policy.decide"),
		rego.Module("policy.rego", b.Code),
	).PrepareForEval(r.Context()); err != nil {
		writeJSON(w, map[string]any{"ok": false, "errors": err.Error()}, 400)
		return
	}
	ver := 0
	if b.Version != nil {
		ver = *b.Version
	} else {
		_ = a.db.QueryRow(r.Context(), `WITH s AS (SELECT set_config('app.tenant_id', $1, true)) SELECT COALESCE(MAX(version)+1,1) FROM policy_versions WHERE action_key=$2`, tid, b.ActionKey).Scan(&ver)
		if ver == 0 {
			ver = 1
		}
	}
	_, err := a.db.Exec(r.Context(), `WITH s AS (
		SELECT set_config('app.tenant_id', $1, true)
	) INSERT INTO policy_versions(id,tenant_id,action_key,version,compiled_rego,status)
	  VALUES ($2::uuid,$1::uuid,$3,$4,$5,'draft')
	  ON CONFLICT (tenant_id,action_key,version) DO UPDATE SET compiled_rego=EXCLUDED.compiled_rego, status='draft', updated_at=NOW()`, tid, uuid.New(), b.ActionKey, ver, b.Code)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "version": ver}, 200)
}

func (a *App) publishPolicyVersion(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	actionKey := strings.TrimSpace(r.URL.Query().Get("action_key"))
	if actionKey == "" {
		http.Error(w, "missing action_key", 400)
		return
	}
	verStr := chi.URLParam(r, "version")
	if strings.TrimSpace(verStr) == "" {
		http.Error(w, "missing version", 400)
		return
	}
	verInt, err := strconv.Atoi(verStr)
	if err != nil {
		http.Error(w, "bad version", 400)
		return
	}
	// Single statement WITH + UPDATE (pgx disallows multiple statements here). Limit update scope to the specific action.
	tag, err := a.db.Exec(r.Context(), `WITH s AS (
		SELECT set_config('app.tenant_id', $1, true)
	)
	UPDATE policy_versions
	SET status = CASE WHEN version = $3 THEN 'published' ELSE 'archived' END,
		updated_at = NOW()
	WHERE tenant_id = $1::uuid AND action_key = $2`, tid, actionKey, verInt)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	if tag.RowsAffected() == 0 { // no such version/action
		http.Error(w, "not found", 404)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "version": verInt}, 200)
}

type CompileBody struct {
	Code string `json:"code"`
}

func (a *App) compilePolicy(w http.ResponseWriter, r *http.Request) {
	var b CompileBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	if _, err := rego.New(rego.Query("data.policy.decide"), rego.Module("policy.rego", b.Code)).PrepareForEval(r.Context()); err != nil {
		writeJSON(w, map[string]any{"ok": false, "errors": err.Error()}, 400)
		return
	}
	writeJSON(w, map[string]any{"ok": true}, 200)
}

type DryRunBody struct {
	Code      string         `json:"code"`
	ActionKey string         `json:"action_key"`
	Inputs    map[string]any `json:"inputs"`
	Trace     bool           `json:"trace"`
}

func (a *App) dryRunPolicy(w http.ResponseWriter, r *http.Request) {
	var b DryRunBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	evalFacts := map[string]any{}
	if strings.TrimSpace(b.ActionKey) != "" {
		if tidVal := r.Context().Value("tid"); tidVal != nil {
			if f, err := factsResolve(r.Context(), a.db, tidVal.(string), b.ActionKey, b.Inputs); err == nil {
				evalFacts = f
			}
		}
	}
	rr := rego.New(
		rego.Query("data.policy.decide"),
		rego.Module("policy.rego", b.Code),
		rego.Input(map[string]any{"inputs": b.Inputs, "facts": evalFacts}),
	)
	rs, err := rr.Eval(r.Context())
	if err != nil || len(rs) == 0 || len(rs[0].Expressions) == 0 {
		resp := map[string]any{"status": "BLOCKED", "reasons": []string{"policy_error"}, "facts_preview": evalFacts}
		if b.Trace {
			resp["trace"] = []any{
				map[string]any{"stage": "inputs", "data": b.Inputs},
				map[string]any{"stage": "facts", "data": evalFacts},
				map[string]any{"stage": "policy", "error": errErrorString(err)},
			}
		}
		writeJSON(w, resp, 200)
		return
	}
	out := rs[0].Expressions[0].Value
	if m, ok := out.(map[string]any); ok {
		if s, ok := m["status"].(string); ok {
			m["status"] = strings.ToUpper(s)
		}
		m["facts_preview"] = evalFacts
		if b.Trace {
			m["trace"] = []any{
				map[string]any{"stage": "inputs", "data": b.Inputs},
				map[string]any{"stage": "facts", "data": evalFacts},
				map[string]any{"stage": "policy", "decision": m},
			}
		}
		writeJSON(w, m, 200)
		return
	}
	resp := map[string]any{"status": "ALLOW", "facts_preview": evalFacts}
	if b.Trace {
		resp["trace"] = []any{
			map[string]any{"stage": "inputs", "data": b.Inputs},
			map[string]any{"stage": "facts", "data": evalFacts},
			map[string]any{"stage": "policy", "decision": resp},
		}
	}
	writeJSON(w, resp, 200)
}

type TestExecuteBody struct {
	ActionKey string         `json:"action_key"`
	Inputs    map[string]any `json:"inputs"`
	Trace     bool           `json:"trace"`
}

// testExecutePolicy performs a full evaluation + orchestrator execute cycle for the given inputs.
func (a *App) testExecutePolicy(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	var b TestExecuteBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	// Resolve facts and evaluate
	evalFacts := map[string]any{}
	if strings.TrimSpace(b.ActionKey) != "" {
		if f, err := factsResolve(r.Context(), a.db, tid, b.ActionKey, b.Inputs); err == nil {
			evalFacts = f
		}
	}
	dec, _ := pol.Evaluate(r.Context(), a.db, tid, b.ActionKey, b.Inputs, evalFacts)
	id, _ := pol.PersistDecision(r.Context(), a.db, tid, dec)
	dec.ID = id
	// Execute via orchestrator
	execRes, _ := orchestrator.Execute(r.Context(), a.db, tid, b.ActionKey, dec.ID, b.Inputs)

	resp := map[string]any{
		"decision": dec,
		"execute":  execRes,
	}
	if b.Trace {
		resp["trace"] = []any{
			map[string]any{"stage": "inputs", "data": b.Inputs},
			map[string]any{"stage": "facts", "data": evalFacts},
			map[string]any{"stage": "policy", "decision": dec},
			map[string]any{"stage": "orchestrator", "steps": execRes.Steps},
		}
	}
	writeJSON(w, resp, 200)
}

// errErrorString safely returns error text or an empty string
func errErrorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (a *App) listPolicyVersions(w http.ResponseWriter, r *http.Request) {
	tid := r.Context().Value("tid").(string)
	actionKey := strings.TrimSpace(r.URL.Query().Get("action_key"))
	if actionKey == "" {
		http.Error(w, "missing action_key", 400)
		return
	}
	rows, err := a.db.Query(r.Context(), `WITH s AS (SELECT set_config('app.tenant_id', $1, true))
		SELECT version, status, updated_at FROM policy_versions WHERE action_key=$2 ORDER BY version DESC`, tid, actionKey)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	defer rows.Close()
	type Row struct {
		Version   int       `json:"version"`
		Status    string    `json:"status"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	out := []Row{}
	for rows.Next() {
		var v Row
		if err := rows.Scan(&v.Version, &v.Status, &v.UpdatedAt); err != nil {
			http.Error(w, "db error", 500)
			return
		}
		out = append(out, v)
	}
	writeJSON(w, map[string]any{"items": out}, 200)
}

package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"lamdis/pkg/problems"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ExecuteResult struct {
	Steps    []map[string]any `json:"steps"`
	Result   map[string]any   `json:"result"`
	Status   string           `json:"status"`
	Problems []Problem        `json:"problems,omitempty"`
}

type Problem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Detail string `json:"detail,omitempty"`
	Step   string `json:"step,omitempty"`
}

// Execute binds to a prior decision id and performs side-effects via connector service (stubbed here).
func Execute(ctx context.Context, pool *pgxpool.Pool, tenantID, actionKey, decisionID string, input map[string]any) (ExecuteResult, error) {
	// Idempotency key: allow clients to pass one; else derive weak key from decisionID
	idempotencyKey := ""
	if v, ok := input["idempotency_key"].(string); ok {
		idempotencyKey = v
	}
	if idempotencyKey == "" {
		idempotencyKey = decisionID
	}
	// Resolve connector operation by splitting actionKey: <namespace>.<short>
	// namespace comes from slugified connector kind; short derived from last static segment of path.
	var (
		baseURL string
		method  string
		path    string
		tmplRaw []byte
	)
	ns := actionKey
	short := ""
	if i := strings.Index(actionKey, "."); i >= 0 {
		ns = actionKey[:i]
		if i+1 < len(actionKey) {
			short = actionKey[i+1:]
		}
	}
	slug := func(s string) string {
		s = strings.TrimSpace(s)
		if s == "" {
			return s
		}
		capRe := regexp.MustCompile(`([a-z0-9])([A-Z])`)
		s = capRe.ReplaceAllString(s, `$1-$2`)
		non := regexp.MustCompile(`[^a-zA-Z0-9]+`)
		s = non.ReplaceAllString(s, "-")
		s = strings.ToLower(strings.Trim(s, "-"))
		dh := regexp.MustCompile(`-+`)
		s = dh.ReplaceAllString(s, "-")
		return s
	}
	deriveShort := func(p string) string {
		p = strings.Split(p, "?")[0]
		p = strings.Trim(p, "/")
		if p == "" {
			return "root"
		}
		segs := strings.Split(p, "/")
		for i := len(segs) - 1; i >= 0; i-- {
			s := segs[i]
			if strings.Contains(s, "{") || strings.Contains(s, "}") || s == "v1" {
				continue
			}
			s = slug(s)
			if s != "" {
				return s
			}
		}
		return "action"
	}
	if pool != nil {
		rows, err := pool.Query(ctx, `WITH s AS (
			SELECT set_config('app.tenant_id', $1, true)
		) SELECT COALESCE(d.base_url,''), o.method, o.path, COALESCE(o.request_tmpl,'{}'::jsonb), d.kind, COALESCE(d.title,'')
		FROM connector_operations o
		JOIN connector_definitions d ON o.connector_id=d.id
		WHERE d.tenant_id=$1 AND COALESCE(o.enabled,true)=true`, tenantID)
		if err == nil {
			defer rows.Close()
			var fallbackSet bool
			for rows.Next() {
				var (
					b     string
					m     string
					p     string
					tr    []byte
					kind  string
					title string
				)
				if scanErr := rows.Scan(&b, &m, &p, &tr, &kind, &title); scanErr != nil {
					continue
				}
				kSlug := slug(kind)
				tSlug := slug(title)
				if kSlug != ns && tSlug != ns {
					continue
				}
				candShort := deriveShort(p)
				if candShort == short || !fallbackSet {
					baseURL, method, path, tmplRaw = b, m, p, tr
					if candShort == short {
						break
					} // best match
					fallbackSet = true
				}
			}
		}
	}
	// If we couldn't resolve any operation, return a problem
	if method == "" || path == "" {
		res := ExecuteResult{
			Steps:  []map[string]any{{"op": "request", "error": "no_operation_mapping"}},
			Result: map[string]any{"ok": false},
			Status: "FAILED",
			Problems: []Problem{{
				Type:   problems.Type("no-operation"),
				Title:  "No connector operation mapped to action",
				Detail: "No enabled connector operation was found for this action key. Ensure your custom connector title or kind matches the action key, or add an explicit mapping.",
			}},
		}
		if pool == nil {
			return res, nil
		}
		_, _ = pool.Exec(ctx, `WITH s AS (
			SELECT set_config('app.tenant_id', $1, true)
		) INSERT INTO executions(tenant_id, action_key, decision_id, idempotency_key, steps, result, status)
		  VALUES ($1,$2,$3,$4,$5,$6,$7)
		  ON CONFLICT DO NOTHING`, tenantID, actionKey, decisionID, idempotencyKey, toJSON(res.Steps), toJSON(res.Result), res.Status)
		return res, nil
	}
	// Build outgoing request using request_tmpl and inputs
	steps := []map[string]any{}
	problemList := []Problem{}
	// Prepare template
	var tmpl map[string]any
	_ = json.Unmarshal(tmplRaw, &tmpl)
	if tmpl == nil {
		tmpl = map[string]any{}
	}
	headers := map[string]string{}
	query := url.Values{}
	var body any
	// Resolve placeholders of the form {{key}} from input
	resolve := func(v any) string {
		s := fmt.Sprintf("%v", v)
		if !strings.Contains(s, "{{") {
			return s
		}
		re := regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_\.]+)\s*\}\}`)
		return re.ReplaceAllStringFunc(s, func(m string) string {
			g := re.FindStringSubmatch(m)
			if len(g) != 2 {
				return ""
			}
			key := g[1]
			// dot path support: a.b.c
			cur := any(input)
			for _, seg := range strings.Split(key, ".") {
				if mm, ok := cur.(map[string]any); ok {
					cur = mm[seg]
				} else {
					cur = nil
					break
				}
			}
			if cur == nil {
				return ""
			}
			return fmt.Sprintf("%v", cur)
		})
	}
	// headers
	if hv, ok := tmpl["headers"].(map[string]any); ok {
		for k, v := range hv {
			headers[k] = resolve(v)
		}
	}
	// query
	if qv, ok := tmpl["query"].(map[string]any); ok {
		// stable order for testing/logging
		keys := make([]string, 0, len(qv))
		for k := range qv {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			query.Set(k, resolve(qv[k]))
		}
	}
	// body
	if bv, ok := tmpl["body"].(map[string]any); ok {
		// deep resolve strings
		rb := map[string]any{}
		for k, v := range bv {
			switch t := v.(type) {
			case string:
				rb[k] = resolve(t)
			default:
				rb[k] = t
			}
		}
		body = rb
	}
	// path params substitution
	fullURL := strings.TrimRight(baseURL, "/") + path
	if pv, ok := tmpl["path_params"].(map[string]any); ok {
		re := regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)
		fullURL = re.ReplaceAllStringFunc(fullURL, func(m string) string {
			name := strings.Trim(m, "{}")
			if raw, ok := pv[name]; ok {
				val := url.PathEscape(resolve(raw))
				if val == "" {
					// leave curly braces to surface error
					return m
				}
				return val
			}
			// No mapping provided, keep placeholder
			return m
		})
	}
	// If unresolved placeholders remain, fail early with problem
	if strings.Contains(fullURL, "{") {
		steps = append(steps, map[string]any{"op": "request", "url": fullURL, "error": "unresolved_path_params"})
		res := ExecuteResult{Steps: steps, Result: map[string]any{"ok": false}, Status: "FAILED", Problems: []Problem{{
			Type:   problems.Type("unresolved-path-params"),
			Title:  "Unresolved path parameters",
			Detail: "One or more path placeholders were not bound. Ensure request_tmpl.path_params maps every {name} in the path and inputs provide values.",
		}}}
		if pool == nil {
			return res, nil
		}
		_, _ = pool.Exec(ctx, `WITH s AS (
			SELECT set_config('app.tenant_id', $1, true)
		) INSERT INTO executions(tenant_id, action_key, decision_id, idempotency_key, steps, result, status)
		  VALUES ($1,$2,$3,$4,$5,$6,$7)
		  ON CONFLICT DO NOTHING`, tenantID, actionKey, decisionID, idempotencyKey, toJSON(res.Steps), toJSON(res.Result), res.Status)
		return res, nil
	}
	// attach query
	if enc := query.Encode(); enc != "" {
		if strings.Contains(fullURL, "?") {
			fullURL += "&" + enc
		} else {
			fullURL += "?" + enc
		}
	}
	// Make HTTP request if we have a method and url
	step := map[string]any{"op": "request", "method": method, "url": fullURL}
	var resp map[string]any
	if method != "" && fullURL != "" {
		var reqBody *bytes.Reader
		if body != nil {
			bb, _ := json.Marshal(body)
			reqBody = bytes.NewReader(bb)
		} else {
			reqBody = bytes.NewReader(nil)
		}
		req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
		if err == nil {
			for k, v := range headers {
				req.Header.Set(k, v)
			}
			if body != nil && req.Header.Get("Content-Type") == "" {
				req.Header.Set("Content-Type", "application/json")
			}
			// Best-effort execute; we do not propagate network errors as failures at this layer yet.
			// In air-gapped or dev scenarios, this will be a no-op with empty response.
			if resp2, err2 := http.DefaultClient.Do(req); err2 == nil {
				defer resp2.Body.Close()
				step["status"] = resp2.StatusCode
				var out map[string]any
				_ = json.NewDecoder(resp2.Body).Decode(&out)
				resp = out
			} else {
				step["error"] = err2.Error()
			}
		} else {
			step["error"] = err.Error()
		}
	}
	steps = append(steps, step)
	res := ExecuteResult{Steps: steps, Result: resp, Status: "SUCCEEDED", Problems: problemList}
	if pool == nil {
		return res, nil
	}
	// Persist execution row
	_, _ = pool.Exec(ctx, `WITH s AS (
        SELECT set_config('app.tenant_id', $1, true)
	) INSERT INTO executions(tenant_id, action_key, decision_id, idempotency_key, steps, result, status)
	  VALUES ($1,$2,$3,$4,$5,$6,$7)
	  ON CONFLICT DO NOTHING`, tenantID, actionKey, decisionID, idempotencyKey, toJSON(res.Steps), toJSON(res.Result), res.Status)
	return res, nil
}

func toJSON(v any) []byte { b, _ := json.Marshal(v); return b }

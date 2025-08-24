package facts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	jmes "github.com/jmespath/go-jmespath"
)

// Resolver represents a configured fact resolver row
type Resolver struct {
	ID             string
	ActionKey      string
	Name           string
	ConnectorKey   string
	RequestTmpl    map[string]any
	ResponseSample map[string]any
	Needs          []map[string]any
}

// Mapping represents a configured mapping row
type Mapping struct {
	ID            string
	ActionKey     string
	Name          string
	Path          string
	FactKey       string
	Transform     string
	TransformArgs []any
	Required      bool
}

// ResolveFacts resolves facts for an action using response_sample as stubbed data.
// It collects response samples from resolvers and applies mappings to produce facts.
func ResolveFacts(ctx context.Context, pool *pgxpool.Pool, tenantID, actionKey string, inputs map[string]any) (map[string]any, error) {
	if pool == nil {
		// dev fallback: return inputs as facts
		out := make(map[string]any, len(inputs))
		for k, v := range inputs {
			out[k] = v
		}
		return out, nil
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return nil, err
	}

	// Load resolvers
	resolvers, err := loadResolvers(ctx, tx, actionKey)
	if err != nil {
		return nil, err
	}
	// Load mappings
	mappings, err := loadMappings(ctx, tx, actionKey)
	if err != nil {
		return nil, err
	}

	// Compose a shared document for JMESPath: { inputs, resolvers: {name: response_sample} }
	doc := map[string]any{"inputs": inputs, "resolvers": map[string]any{}}
	resMap := doc["resolvers"].(map[string]any)
	for _, r := range resolvers {
		resMap[r.Name] = r.ResponseSample
	}
	facts := map[string]any{}
	// Apply mappings
	for _, m := range mappings {
		val, err := jmes.Search(m.Path, doc)
		if err != nil {
			if m.Required {
				return nil, err
			}
			continue
		}
		// transforms
		if m.Transform != "" {
			tv, terr := applyTransform(m.Transform, val, m.TransformArgs...)
			if terr != nil {
				if m.Required {
					return nil, terr
				}
				continue
			}
			val = tv
		}
		facts[m.FactKey] = val
	}

	return facts, tx.Commit(ctx)
}

func loadResolvers(ctx context.Context, tx pgx.Tx, actionKey string) ([]Resolver, error) {
	rows, err := tx.Query(ctx, `SELECT id, action_key, name, connector_key, COALESCE(request_template,'{}')::jsonb, COALESCE(response_sample,'{}')::jsonb, COALESCE(needs,'[]')::jsonb 
		FROM fact_resolvers WHERE action_key=$1 AND enabled=true`, actionKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Resolver{}
	for rows.Next() {
		var r Resolver
		var reqRaw, respRaw, needsRaw []byte
		if err := rows.Scan(&r.ID, &r.ActionKey, &r.Name, &r.ConnectorKey, &reqRaw, &respRaw, &needsRaw); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(reqRaw, &r.RequestTmpl)
		_ = json.Unmarshal(respRaw, &r.ResponseSample)
		_ = json.Unmarshal(needsRaw, &r.Needs)
		out = append(out, r)
	}
	return out, nil
}

func loadMappings(ctx context.Context, tx pgx.Tx, actionKey string) ([]Mapping, error) {
	rows, err := tx.Query(ctx, `SELECT id, action_key, name, jmespath, fact_key, COALESCE(transform,''), COALESCE(transform_args,'[]')::jsonb, required FROM fact_mappings WHERE action_key=$1`, actionKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Mapping
	for rows.Next() {
		var m Mapping
		var argsRaw []byte
		if err := rows.Scan(&m.ID, &m.ActionKey, &m.Name, &m.Path, &m.FactKey, &m.Transform, &argsRaw, &m.Required); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(argsRaw, &m.TransformArgs)
		out = append(out, m)
	}
	return out, nil
}

func applyTransform(name string, v any, args ...any) (any, error) {
	switch name {
	case "count":
		if arr, ok := v.([]any); ok {
			return len(arr), nil
		}
		return 0, nil
	case "sum":
		if arr, ok := v.([]any); ok {
			var s float64
			for _, it := range arr {
				s += toFloat(it)
			}
			return s, nil
		}
		return toFloat(v), nil
	case "days_between":
		if len(args) == 2 {
			t1, e1 := toTime(args[0])
			t2, e2 := toTime(args[1])
			if e1 != nil || e2 != nil {
				return nil, errors.New("invalid time")
			}
			return int(time.Since(t1).Hours()/24 - time.Since(t2).Hours()/24), nil
		}
		if pair, ok := v.([]any); ok && len(pair) == 2 {
			t1, e1 := toTime(pair[0])
			t2, e2 := toTime(pair[1])
			if e1 != nil || e2 != nil {
				return nil, errors.New("invalid time")
			}
			return int(time.Since(t1).Hours()/24 - time.Since(t2).Hours()/24), nil
		}
		return nil, errors.New("days_between expects 2 args")
	case "now":
		return time.Now().UTC().Format(time.RFC3339), nil
	case "any":
		if len(args) == 2 {
			src := toArray(args[0])
			pred, _ := args[1].(string)
			for _, it := range src {
				ok, _ := matchPredicate(it, pred)
				if ok {
					return true, nil
				}
			}
			return false, nil
		}
		if arr, ok := v.([]any); ok {
			return len(arr) > 0, nil
		}
		return v != nil, nil
	case "all":
		if len(args) == 2 {
			src := toArray(args[0])
			pred, _ := args[1].(string)
			if len(src) == 0 {
				return false, nil
			}
			for _, it := range src {
				ok, _ := matchPredicate(it, pred)
				if !ok {
					return false, nil
				}
			}
			return true, nil
		}
		if arr, ok := v.([]any); ok {
			return len(arr) > 0, nil
		}
		return v != nil, nil
	case "exists":
		return v != nil, nil
	case "first":
		if arr, ok := v.([]any); ok {
			if len(arr) > 0 {
				return arr[0], nil
			}
			return nil, nil
		}
		return v, nil
	case "to_number":
		switch t := v.(type) {
		case string:
			f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
			if err != nil {
				return 0, nil
			}
			return f, nil
		default:
			return toFloat(v), nil
		}
	case "to_string":
		return fmt.Sprintf("%v", v), nil
	case "coalesce":
		if len(args) > 0 {
			for _, a := range args {
				if !isZero(a) {
					return a, nil
				}
			}
		}
		return v, nil
	default:
		return v, nil
	}
}

func toFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	default:
		return 0
	}
}

func toTime(v any) (time.Time, error) {
	switch t := v.(type) {
	case string:
		// try RFC3339
		if ts, err := time.Parse(time.RFC3339, t); err == nil {
			return ts, nil
		}
		// try date only
		return time.Parse("2006-01-02", t)
	default:
		return time.Time{}, errors.New("unsupported time type")
	}
}

func isZero(v any) bool {
	if v == nil {
		return true
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t) == ""
	case float64:
		return t == 0
	case int:
		return t == 0
	case []any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	default:
		return false
	}
}

func toArray(v any) []any {
	switch t := v.(type) {
	case []any:
		return t
	default:
		if v == nil {
			return nil
		}
		return []any{v}
	}
}

// Very limited predicate support: field=='value'
func matchPredicate(v any, pred string) (bool, error) {
	parts := strings.Split(pred, "==")
	if len(parts) != 2 {
		return false, nil
	}
	left := strings.TrimSpace(parts[0])
	right := strings.Trim(strings.TrimSpace(parts[1]), "'\"")
	if m, ok := v.(map[string]any); ok {
		if lv, ok := m[left]; ok {
			return fmt.Sprint(lv) == right, nil
		}
	}
	return false, nil
}

// ResolverNeeds returns the configured needs prompts for an action's resolvers.
func ResolverNeeds(ctx context.Context, pool *pgxpool.Pool, tenantID, actionKey string) ([]map[string]any, error) {
	if pool == nil {
		return nil, nil
	}
	rows, err := pool.Query(ctx, `WITH s AS (
		SELECT set_config('app.tenant_id', $1, true)
	) SELECT COALESCE(needs,'[]')::jsonb FROM fact_resolvers WHERE action_key=$2 AND enabled=true`, tenantID, actionKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var arr []map[string]any
		_ = json.Unmarshal(raw, &arr)
		out = append(out, arr...)
	}
	return out, nil
}

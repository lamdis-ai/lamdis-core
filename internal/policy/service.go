package policy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"lamdis/internal/facts"

	"lamdis/pkg/problems"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/open-policy-agent/opa/rego"
)

type DecisionStatus string

const (
	Allow               DecisionStatus = "ALLOW"
	AllowWithConditions DecisionStatus = "ALLOW_WITH_CONDITIONS"
	Blocked             DecisionStatus = "BLOCKED"
	NeedsInput          DecisionStatus = "NEEDS_INPUT"
)

type Decision struct {
	ID            string         `json:"id,omitempty"`
	ActionKey     string         `json:"action_key,omitempty"`
	Inputs        map[string]any `json:"inputs,omitempty"`
	Facts         map[string]any `json:"facts,omitempty"`
	PolicyVersion int            `json:"policy_version,omitempty"`
	Status        DecisionStatus `json:"status"`
	Reasons       any            `json:"reasons,omitempty"`
	Needs         any            `json:"needs,omitempty"`
	Alternatives  any            `json:"alternatives,omitempty"`
	ExpiresAt     *time.Time     `json:"expires_at,omitempty"`
}

// Evaluate loads the latest published policy for the tenant and evaluates it with inputs and facts.
func Evaluate(ctx context.Context, pool *pgxpool.Pool, tenantID, actionKey string, inputs, facts map[string]any) (Decision, error) {
	// Load compiled_rego for latest published version
	var mod string
	var ver int
	if pool != nil {
		row := pool.QueryRow(ctx, `WITH s AS (
			SELECT set_config('app.tenant_id', $1, true)
		) SELECT COALESCE(compiled_rego,''), COALESCE(version,0) FROM policy_versions WHERE action_key=$2 AND status='published' ORDER BY version DESC LIMIT 1`, tenantID, actionKey)
		_ = row.Scan(&mod, &ver)
	}
	// Default allow if no policy
	if mod == "" {
		// short TTL by default
		t := time.Now().Add(15 * time.Minute)
		return Decision{ActionKey: actionKey, Inputs: inputs, Facts: facts, PolicyVersion: ver, Status: Allow, ExpiresAt: &t}, nil
	}
	// Evaluate rego entrypoint `data.policy.decide`
	r := rego.New(
		rego.Query("data.policy.decide"),
		rego.Module("policy.rego", mod),
		rego.Input(map[string]any{"inputs": inputs, "facts": facts}),
	)
	rs, err := r.Eval(ctx)
	if err != nil || len(rs) == 0 || len(rs[0].Expressions) == 0 {
		t := time.Now().Add(5 * time.Minute)
		return Decision{ActionKey: actionKey, Inputs: inputs, Facts: facts, PolicyVersion: ver, Status: Blocked, Reasons: []string{"policy_error"}, ExpiresAt: &t}, nil
	}
	out := rs[0].Expressions[0].Value
	dec := Decision{ActionKey: actionKey, Inputs: inputs, Facts: facts, PolicyVersion: ver}
	if m, ok := out.(map[string]any); ok {
		if s, ok := m["status"].(string); ok {
			switch s {
			case "ALLOW":
				dec.Status = Allow
			case "ALLOW_WITH_CONDITIONS":
				dec.Status = AllowWithConditions
			case "NEEDS_INPUT":
				dec.Status = NeedsInput
			default:
				dec.Status = Blocked
			}
		} else {
			dec.Status = Blocked
		}
		dec.Reasons = m["reasons"]
		dec.Needs = m["needs"]
		dec.Alternatives = m["alternatives"]
		if ttl, ok := m["ttl_seconds"].(float64); ok && ttl > 0 {
			t := time.Now().Add(time.Duration(ttl) * time.Second)
			dec.ExpiresAt = &t
		} else {
			t := time.Now().Add(15 * time.Minute)
			dec.ExpiresAt = &t
		}
	} else {
		dec.Status = Allow
		t := time.Now().Add(15 * time.Minute)
		dec.ExpiresAt = &t
	}
	return dec, nil
}

// PersistDecision stores a decision and returns its id, computing a binding hash as well.
func PersistDecision(ctx context.Context, pool *pgxpool.Pool, tenantID string, d Decision) (string, error) {
	if pool == nil {
		return "dev-decision", nil
	}
	// Compute hash of inputs+facts+policy_version for binding
	h := sha256.Sum256([]byte(fmt.Sprintf("%x|%x|%d", mustJSON(d.Inputs), mustJSON(d.Facts), d.PolicyVersion)))
	hash := hex.EncodeToString(h[:])
	row := pool.QueryRow(ctx, `WITH s AS (
		SELECT set_config('app.tenant_id', $1, true)
	) INSERT INTO decisions(tenant_id, action_key, inputs, facts, policy_version, status, reasons, needs, alternatives, hash, expires_at)
	  VALUES ($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id`, tenantID, d.ActionKey, toJSON(d.Inputs), toJSON(d.Facts), d.PolicyVersion, string(d.Status), toJSON(d.Reasons), toJSON(d.Needs), toJSON(d.Alternatives), hash, d.ExpiresAt)
	var id string
	if err := row.Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

// ValidateDecision ensures the decision is executable (allow status and not expired).
func ValidateDecision(ctx context.Context, pool *pgxpool.Pool, tenantID, decisionID string) (bool, map[string]any) {
	if pool == nil {
		return true, nil
	}
	var status string
	var expiresAt *time.Time
	row := pool.QueryRow(ctx, `WITH s AS (
		SELECT set_config('app.tenant_id', $1, true)
	) SELECT status, expires_at FROM decisions WHERE id=$2`, tenantID, decisionID)
	if err := row.Scan(&status, &expiresAt); err != nil {
		return false, problem(problems.Type("invalid-decision"), "Invalid decision id", "The provided decision_id is unknown or not accessible")
	}
	if status != string(Allow) && status != string(AllowWithConditions) {
		return false, problem(problems.Type("decision-blocked"), "Decision is blocked", "The decision is not allowed for execution")
	}
	if expiresAt != nil && expiresAt.Before(time.Now()) {
		return false, problem(problems.Type("decision-expired"), "Decision expired", "The decision has expired; call eligibility again")
	}
	return true, nil
}

func toJSON(v any) []byte   { b, _ := json.Marshal(v); return b }
func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }

func problem(tp, title, detail string) map[string]any {
	return map[string]any{
		"type":   tp,
		"title":  title,
		"detail": detail,
	}
}

// ValidateAndBindDecision ensures the decision is executable, matches action_key,
// is not expired, and that the binding hash for inputs+facts+policy_version matches.
func ValidateAndBindDecision(ctx context.Context, pool *pgxpool.Pool, tenantID, decisionID, actionKey string, inputs map[string]any) (bool, map[string]any) {
	if pool == nil {
		return true, nil
	}
	var status, storedAction, storedHash string
	var ver int
	var expiresAt *time.Time
	row := pool.QueryRow(ctx, `WITH s AS (
		SELECT set_config('app.tenant_id', $1, true)
	) SELECT status, action_key, policy_version, hash, expires_at FROM decisions WHERE id=$2`, tenantID, decisionID)
	if err := row.Scan(&status, &storedAction, &ver, &storedHash, &expiresAt); err != nil {
		return false, problem(problems.Type("invalid-decision"), "Invalid decision id", "The provided decision_id is unknown or not accessible")
	}
	if storedAction != actionKey {
		return false, problem(problems.Type("decision-mismatch"), "Decision mismatch", "The decision_id does not match this action")
	}
	if status != string(Allow) && status != string(AllowWithConditions) {
		return false, problem(problems.Type("decision-blocked"), "Decision is blocked", "The decision is not allowed for execution")
	}
	if expiresAt != nil && expiresAt.Before(time.Now()) {
		return false, problem(problems.Type("decision-expired"), "Decision expired", "The decision has expired; call eligibility again")
	}
	// Recompute facts with provided inputs and compare hash
	fa, _ := facts.ResolveFacts(ctx, pool, tenantID, actionKey, inputs)
	h := sha256.Sum256([]byte(fmt.Sprintf("%x|%x|%d", mustJSON(inputs), mustJSON(fa), ver)))
	calc := hex.EncodeToString(h[:])
	if storedHash != "" && storedHash != calc {
		return false, problem(problems.Type("decision-mismatch"), "Decision mismatch", "Inputs or facts changed; please re-run eligibility")
	}
	return true, nil
}

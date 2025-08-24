package policy

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lamdis/internal/facts"
	"lamdis/internal/orchestrator"
	"lamdis/pkg/middleware"
	"lamdis/pkg/problems"
)

// RegisterHTTP mounts preflight and execute endpoints for actions.
// POST /v1/actions/{key}/preflight  body: { inputs }
// POST /v1/actions/{key}/execute    body: { decision_id, inputs? }
func RegisterHTTP(r chi.Router, pool *pgxpool.Pool) {
	r.Post("/v1/actions/{key}/preflight", func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		tenant := middleware.TenantFrom(ctx)
		key := chi.URLParam(req, "key")
		var body struct {
			Inputs map[string]any `json:"inputs"`
			Hints  map[string]any `json:"hints"`
		}
		_ = json.NewDecoder(req.Body).Decode(&body)
		fa, _ := facts.ResolveFacts(ctx, pool, tenant.ID, key, body.Inputs)
		dec, _ := Evaluate(ctx, pool, tenant.ID, key, body.Inputs, fa)
		// If required facts missing and policy needs inputs, surface needs
		if dec.Status == NeedsInput {
			needs, _ := facts.ResolverNeeds(ctx, pool, tenant.ID, key)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "NEEDS_INPUT",
				"needs":  needs,
			})
			return
		}
		// Persist decision for ALLOW or BLOCKED/conditions
		id, _ := PersistDecision(ctx, pool, tenant.ID, dec)
		dec.ID = id
		resp := map[string]any{"status": string(dec.Status)}
		if dec.Status == Allow || dec.Status == AllowWithConditions {
			resp["decision_id"] = dec.ID
			if dec.ExpiresAt != nil {
				resp["expires_at"] = dec.ExpiresAt
			}
			if dec.Reasons != nil {
				resp["reasons"] = dec.Reasons
			}
			if dec.Needs != nil {
				resp["conditions"] = dec.Needs
			}
		} else if dec.Status == Blocked {
			// Return structured reasons and alternatives
			resp["reasons"] = dec.Reasons
			resp["alternatives"] = dec.Alternatives
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	r.Post("/v1/actions/{key}/execute", func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		tenant := middleware.TenantFrom(ctx)
		key := chi.URLParam(req, "key")
		var body struct {
			DecisionID string         `json:"decision_id"`
			Inputs     map[string]any `json:"inputs"`
		}
		_ = json.NewDecoder(req.Body).Decode(&body)
		if strings.TrimSpace(body.DecisionID) == "" {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"type":   problems.Type("preflight-required"),
				"title":  "Preflight required",
				"detail": "Call eligibility first and pass decision_id to execute",
			})
			return
		}
		// Validate decision binding against action and recomputed facts hash
		if ok, prob := ValidateAndBindDecision(ctx, pool, tenant.ID, body.DecisionID, key, body.Inputs); !ok {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(prob)
			return
		}
		res, _ := orchestrator.Execute(ctx, pool, tenant.ID, key, body.DecisionID, body.Inputs)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(res)
	})
}

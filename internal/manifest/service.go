// internal/manifest/service.go
package manifest

import (
	"context"
	"regexp"
	"strings"

	"lamdis/pkg/config"
	"lamdis/pkg/connectors"
	"lamdis/pkg/problems"
	"lamdis/pkg/tenants"
)

type OAuth struct {
	AuthorizationURL string   `json:"authorization_url"`
	TokenURL         string   `json:"token_url"`
	Scopes           []string `json:"scopes"`
}

type Action struct {
	Path                      string            `json:"path"`
	Method                    string            `json:"method"`
	Scope                     string            `json:"scope"`
	Summary                   string            `json:"summary"`
	Title                     string            `json:"title"`
	Params                    []map[string]any  `json:"params"`
	RequiresPreflight         bool              `json:"requires_preflight"`
	Flow                      map[string]any    `json:"flow"`
	Key                       string            `json:"key,omitempty"`
	DisplayName               string            `json:"display_name,omitempty"`
	PreflightEndpoint         string            `json:"preflight_endpoint,omitempty"`
	ExecuteEndpoint           string            `json:"execute_endpoint,omitempty"`
	ExecutionRequiresDecision bool              `json:"execution_requires_decision,omitempty"`
	InputsSchema              map[string]any    `json:"inputs_schema,omitempty"`
	NeedsContract             bool              `json:"needs_contract,omitempty"`
	AlternativesSupported     []string          `json:"alternatives_supported,omitempty"`
	ProblemTypes              map[string]string `json:"problem_types,omitempty"`
}

type Manifest struct {
	Version   string   `json:"version"`
	BaseURL   string   `json:"base_url"`
	OAuth     OAuth    `json:"oauth"`
	Actions   []Action `json:"actions"`
	Namespace string   `json:"namespace,omitempty"`
}

func BuildManifest(ctx context.Context, cfg config.Config, t tenants.Tenant, reg *connectors.Registry) Manifest {
	base := t.BasePublicURL
	if base == "" {
		base = cfg.DefaultBasePublicURL
	}
	iss := t.OAuthIssuer
	ns := t.Slug
	if ns == "" {
		// Fallback to tenant ID if slug missing (rare)
		ns = t.ID
	}
	// helper to derive short action name from path (last static segment)
	deriveName := func(p string) string {
		// trim query, leading/trailing slashes
		p = strings.Split(p, "?")[0]
		p = strings.Trim(p, "/")
		if p == "" {
			return "root"
		}
		segs := strings.Split(p, "/")
		// walk from end to find a segment without parameter braces
		for i := len(segs) - 1; i >= 0; i-- {
			s := segs[i]
			if strings.Contains(s, "{") || strings.Contains(s, "}") || s == "v1" { // skip param or version placeholders
				continue
			}
			// normalize: lowercase, replace non-alphanum with '-'
			re := regexp.MustCompile(`[^a-zA-Z0-9]+`)
			s = strings.ToLower(re.ReplaceAllString(s, "-"))
			s = strings.Trim(s, "-")
			if s != "" {
				return s
			}
		}
		return "action"
	}
	var actions []Action
	if reg != nil {
		if ops, err := reg.LoadOperations(ctx, t.ID); err == nil {
			for _, o := range ops {
				scope := ""
				if len(o.Scopes) > 0 {
					scope = o.Scopes[0]
				}
				short := deriveName(o.Path)
				namespace := ns
				if o.Kind != nil && *o.Kind != "" {
					// prefer connector kind for namespace to distinguish connectors; slugify for consistency
					kind := *o.Kind
					// Insert hyphen before capitals after first char (CatcherTest -> Catcher-Test)
					capRe := regexp.MustCompile(`([a-z0-9])([A-Z])`)
					kind = capRe.ReplaceAllString(kind, `$1-$2`)
					// Normalize non-alphanum to hyphen, lowercase, collapse multiple hyphens
					nonAlnum := regexp.MustCompile(`[^a-zA-Z0-9]+`)
					kind = nonAlnum.ReplaceAllString(kind, "-")
					kind = strings.ToLower(strings.Trim(kind, "-"))
					dupHyphen := regexp.MustCompile(`-+`)
					kind = dupHyphen.ReplaceAllString(kind, "-")
					if kind != "" {
						namespace = kind
					}
				}
				key := namespace + "." + short
				// Each dynamic operation supports a two-phase flow.
				actions = append(actions, Action{
					Path: o.Path, Method: o.Method, Scope: scope, Summary: o.Summary, Title: o.Summary, Params: o.Params,
					Key:               key,
					RequiresPreflight: true,
					Flow: map[string]any{
						"preflight":    map[string]any{"method": "POST", "path": "/v1/actions/{key}/preflight"},
						"execute":      map[string]any{"method": "POST", "path": "/v1/actions/{key}/execute", "binds": []string{"decision_id"}},
						"needs_input":  true,
						"alternatives": true,
						"consent":      true,
					},
					DisplayName:               o.Summary,
					PreflightEndpoint:         "/v1/actions/{key}/eligibility",
					ExecuteEndpoint:           "/v1/actions/{key}/execute",
					ExecutionRequiresDecision: true,
					InputsSchema:              map[string]any{},
					NeedsContract:             true,
					AlternativesSupported:     []string{"create_checkout_link", "open_support_case"},
					ProblemTypes: map[string]string{
						"preflight_required": problems.Type("preflight-required"),
						"policy_violation":   problems.Type("policy-violation"),
					},
				})
			}
		}
	}
	return Manifest{
		Version: "1",
		BaseURL: base,
		OAuth: OAuth{
			AuthorizationURL: iss + "/authorize",
			TokenURL:         iss + "/token",
			Scopes:           []string{"catalog:read", "order:write", "refund:write"},
		},
		Actions:   actions,
		Namespace: ns,
	}
}

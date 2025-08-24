package tenants

// Tenant represents a logical customer / account space.
type Tenant struct {
	ID                   string // uuid
	Slug                 string // short name (acme)
	Host                 string // primary host (ai.acme.com)
	OAuthIssuer          string
	JWKSURL              string
	BasePublicURL        string            // connector base URL for actions
	AuthMode             string            // platform | byoidc (default byoidc)
	DiscoveryURL         string            // optional explicit OIDC discovery URL (else issuer + "/.well-known/openid-configuration")
	AcceptedAudiences    []string          // list of acceptable aud values (if empty -> fallback to global config audience)
	AccountClaim         string            // which claim maps to account identity (default "sub")
	MachineAllowedScopes []string          // subset of scopes allowed when grant=client_credentials
	RequiredACRByAction  map[string]string // actionScope (or action id) -> required ACR value
	DPoPRequired         bool              // whether DPoP proof is required for tokens
}

// Connector kinds
type ConnectorCreds struct {
	ShopifyDomain string
	ShopifyToken  string
}

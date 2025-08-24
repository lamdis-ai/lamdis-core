// internal/connector/service.go
package connector

import (
	"github.com/go-chi/chi/v5"
)

// Service represents a commerce connector implementation (legacy interface kept
// only if any remaining code references it). Dynamic runtime no longer uses the
// deprecated business methods removed below.
// NOTE: Removed CreateCheckoutLink / RefundOrder which depended on obsolete LineItem type.
type Service interface {
	Name() string
	Operations() []OperationDescriptor
	Register(r chi.Router)
}

// OperationDescriptor is a lightweight description of an endpoint a connector exposes.
type OperationDescriptor struct {
	Method  string
	Path    string
	Summary string
	Scopes  []string
}

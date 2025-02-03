package policies

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
)

// Validator validates an NGF Policy.
//
//counterfeiter:generate . Validator
type Validator interface {
	// Validate validates an NGF Policy.
	Validate(policy Policy, globalSettings *GlobalSettings) []conditions.Condition
	// Conflicts returns true if the two Policies conflict.
	Conflicts(a, b Policy) bool
}

// CompositeValidator manages the validators for NGF Policies.
type CompositeValidator struct {
	validators     map[schema.GroupVersionKind]Validator
	mustExtractGVK kinds.MustExtractGVK
}

// ManagerConfig contains the config to register a Policy with the CompositeValidator.
type ManagerConfig struct {
	// Validator is the Validator for the Policy.
	Validator Validator
	// GVK is the GroupVersionKind of the Policy.
	GVK schema.GroupVersionKind
}

// NewManager returns a new CompositeValidator.
// Implements validation.PolicyValidator.
func NewManager(
	mustExtractGVK kinds.MustExtractGVK,
	configs ...ManagerConfig,
) *CompositeValidator {
	v := &CompositeValidator{
		validators:     make(map[schema.GroupVersionKind]Validator),
		mustExtractGVK: mustExtractGVK,
	}

	for _, cfg := range configs {
		v.validators[cfg.GVK] = cfg.Validator
	}

	return v
}

// Validate validates the policy.
func (m *CompositeValidator) Validate(policy Policy, globalSettings *GlobalSettings) []conditions.Condition {
	gvk := m.mustExtractGVK(policy)

	validator, ok := m.validators[gvk]
	if !ok {
		panic(fmt.Sprintf("no validator registered for policy %T", policy))
	}

	return validator.Validate(policy, globalSettings)
}

// Conflicts returns true if the policies conflict.
func (m *CompositeValidator) Conflicts(polA, polB Policy) bool {
	gvk := m.mustExtractGVK(polA)

	validator, ok := m.validators[gvk]
	if !ok {
		panic(fmt.Sprintf("no validator registered for policy %T", polA))
	}

	return validator.Conflicts(polA, polB)
}

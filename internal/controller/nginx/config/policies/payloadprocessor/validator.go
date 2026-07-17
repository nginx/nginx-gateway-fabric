package payloadprocessor

import (
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

// Validator validates a PayloadProcessor policy.
// Implements policies.Validator interface.
type Validator struct{}

// NewValidator returns a new Validator.
func NewValidator() *Validator { return &Validator{} }

// Validate validates the spec of a PayloadProcessor.
func (v *Validator) Validate(policy policies.Policy) []conditions.Condition {
	pp := helpers.MustCastObject[*ngfAPI.PayloadProcessor](policy)

	// TargetRef validation: allow Gateway or HTTPRoute
	supportedKinds := []gatewayv1.Kind{kinds.Gateway, kinds.HTTPRoute}
	supportedGroups := []gatewayv1.Group{gatewayv1.GroupName}

	if err := policies.ValidateTargetRef(pp.Spec.TargetRef, nil, supportedGroups, supportedKinds); err != nil {
		return []conditions.Condition{conditions.NewPolicyInvalid(err.Error())}
	}

	if len(pp.Spec.Processors) == 0 {
		return []conditions.Condition{conditions.NewPolicyInvalid("at least one processor must be specified")}
	}

	for _, processor := range pp.Spec.Processors {
		if processor.ExtProc == nil {
			return []conditions.Condition{conditions.NewPolicyInvalid("processor missing extProc configuration")}
		}
		if processor.ExtProc.BackendRef.Name == "" {
			return []conditions.Condition{conditions.NewPolicyInvalid("processor extProc.backendRef.name must be set")}
		}
		if group := processor.ExtProc.BackendRef.Group; group != nil && *group != "" && *group != "core" {
			return []conditions.Condition{conditions.NewPolicyInvalid("processor extProc.backendRef.group must be core")}
		}
		if kind := processor.ExtProc.BackendRef.Kind; kind != nil && *kind != kinds.Service {
			return []conditions.Condition{conditions.NewPolicyInvalid("processor extProc.backendRef.kind must be Service")}
		}
		if processor.ExtProc.BackendRef.Port == nil {
			return []conditions.Condition{conditions.NewPolicyInvalid("processor extProc.backendRef.port must be set")}
		}
		if port := *processor.ExtProc.BackendRef.Port; port < 1 || port > 65535 {
			return []conditions.Condition{
				conditions.NewPolicyInvalid("processor extProc.backendRef.port must be a valid TCP port"),
			}
		}
	}

	return nil
}

// ValidateGlobalSettings validates the policy with respect to global settings.
func (v *Validator) ValidateGlobalSettings(_ policies.Policy, _ *policies.GlobalSettings) []conditions.Condition {
	return nil
}

// Conflicts returns true if the two PayloadProcessors conflict.
// PayloadProcessors occupy a single processing phase, so any two policies targeting the same object
// conflict. markConflictedPolicies groups by policy GVK and target ref and sorts oldest-first, so the
// oldest policy is kept and newer conflicting policies are marked Conflicted.
func (v *Validator) Conflicts(_ policies.Policy, _ policies.Policy) bool { return true }

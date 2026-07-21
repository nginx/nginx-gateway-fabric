package payloadprocessor

import (
	"k8s.io/apimachinery/pkg/util/validation/field"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

const (
	// coreGroup is the API group for core Kubernetes resources (e.g. Service).
	coreGroup gatewayv1.Group = "core"
	// minPort is the minimum valid TCP port.
	minPort gatewayv1.PortNumber = 1
	// maxPort is the maximum valid TCP port.
	maxPort gatewayv1.PortNumber = 65535
)

// Validator validates a PayloadProcessor policy.
// Implements policies.Validator interface.
type Validator struct{}

// NewValidator returns a new Validator.
func NewValidator() *Validator { return &Validator{} }

// Validate validates the spec of a PayloadProcessor.
func (v *Validator) Validate(policy policies.Policy) []conditions.Condition {
	pp := helpers.MustCastObject[*ngfAPI.PayloadProcessor](policy)

	specPath := field.NewPath("spec")

	// TargetRef validation: allow Gateway or HTTPRoute
	targetRefPath := specPath.Child("targetRef")
	supportedKinds := []gatewayv1.Kind{kinds.Gateway, kinds.HTTPRoute}
	supportedGroups := []gatewayv1.Group{gatewayv1.GroupName}

	if err := policies.ValidateTargetRef(pp.Spec.TargetRef, targetRefPath, supportedGroups, supportedKinds); err != nil {
		return []conditions.Condition{conditions.NewPolicyInvalid(err.Error())}
	}

	if err := validateProcessors(pp.Spec.Processors, specPath.Child("processors")); err != nil {
		return []conditions.Condition{conditions.NewPolicyInvalid(err.Error())}
	}

	return nil
}

// validateProcessors validates the list of processor entries.
func validateProcessors(processors []ngfAPI.PayloadProcessorEntry, processorsPath *field.Path) error {
	var allErrs field.ErrorList

	for i, processor := range processors {
		allErrs = append(allErrs, validateProcessor(processor, processorsPath.Index(i))...)
	}

	return allErrs.ToAggregate()
}

// validateProcessor validates a single processor entry.
func validateProcessor(processor ngfAPI.PayloadProcessorEntry, processorPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	typePath := processorPath.Child("type")
	if processor.Type != ngfAPI.ProcessorTypeExtProcess {
		allErrs = append(allErrs, field.NotSupported(
			typePath,
			processor.Type,
			[]string{string(ngfAPI.ProcessorTypeExtProcess)},
		))
		return allErrs
	}

	extProcessPath := processorPath.Child("extProcess")

	allErrs = append(allErrs, validateExtProcessBackendRef(
		processor.ExtProcess.BackendRef,
		extProcessPath.Child("backendRef"),
	)...)

	return allErrs
}

// validateExtProcessBackendRef validates the backendRef of an ExtProcess processor.
func validateExtProcessBackendRef(
	backendRef gatewayv1.BackendObjectReference,
	backendRefPath *field.Path,
) field.ErrorList {
	var allErrs field.ErrorList

	if group := backendRef.Group; group != nil && *group != "" && *group != coreGroup {
		allErrs = append(allErrs, field.NotSupported(
			backendRefPath.Child("group"),
			*group,
			[]string{string(coreGroup)},
		))
	}

	if kind := backendRef.Kind; kind != nil && *kind != kinds.Service {
		allErrs = append(allErrs, field.NotSupported(
			backendRefPath.Child("kind"),
			*kind,
			[]string{string(kinds.Service)},
		))
	}

	portPath := backendRefPath.Child("port")
	if backendRef.Port == nil {
		allErrs = append(allErrs, field.Required(portPath, "port must be set"))
	} else if port := *backendRef.Port; port < minPort || port > maxPort {
		allErrs = append(allErrs, field.Invalid(
			portPath,
			port,
			"port must be a valid TCP port (1-65535)",
		))
	}

	return allErrs
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

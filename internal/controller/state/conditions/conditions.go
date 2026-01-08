package conditions

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	inference "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
)

const (
	// GatewayClassReasonGatewayClassConflict indicates there are multiple GatewayClass resources
	// that reference this controller, and we ignored the resource in question and picked the
	// GatewayClass that is referenced in the command-line argument.
	// This reason is used with GatewayClassConditionAccepted (false).
	GatewayClassReasonGatewayClassConflict v1.GatewayClassConditionReason = "GatewayClassConflict"

	// GatewayClassMessageGatewayClassConflict is a message that describes GatewayClassReasonGatewayClassConflict.
	GatewayClassMessageGatewayClassConflict = "The resource is ignored due to a conflicting GatewayClass resource"

	// ListenerReasonUnsupportedValue is used with the "Accepted" condition when a value of a field in a Listener
	// is invalid or not supported.
	ListenerReasonUnsupportedValue v1.ListenerConditionReason = "UnsupportedValue"

	// ListenerMessageFailedNginxReload is a message used with ListenerConditionProgrammed (false)
	// when nginx fails to reload.
	ListenerMessageFailedNginxReload = "The Listener is not programmed due to a failure to " +
		"reload nginx with the configuration"

	// ListenerMessageOverlappingHostnames is a message used with the "OverlappingTLSConfig" condition when the
	// condition is true due to overlapping hostnames.
	ListenerMessageOverlappingHostnames = "Listener hostname overlaps with hostname(s) of other Listener(s) " +
		"on the same port"

	// RouteReasonBackendRefUnsupportedValue is used with the "ResolvedRefs" condition when one of the
	// Route rules has a backendRef with an unsupported value.
	RouteReasonBackendRefUnsupportedValue v1.RouteConditionReason = "UnsupportedValue"

	// RouteReasonUnsupportedField is used with the "Accepted" condition when a Route contains fields that are
	// not yet supported.
	RouteReasonUnsupportedField v1.RouteConditionReason = "UnsupportedField"

	// RouteReasonInvalidGateway is used with the "Accepted" (false) condition when the Gateway the Route
	// references is invalid.
	RouteReasonInvalidGateway v1.RouteConditionReason = "InvalidGateway"

	// RouteReasonInvalidListener is used with the "Accepted" condition when the Route references an invalid listener.
	RouteReasonInvalidListener v1.RouteConditionReason = "InvalidListener"

	// RouteReasonHostnameConflict is used with the "Accepted" condition when a route has the exact same hostname
	// as another route.
	RouteReasonHostnameConflict v1.RouteConditionReason = "HostnameConflict"

	// RouteReasonMultipleRoutesOnListener is used with the "Accepted" condition when multiple
	// L4 Routes are attached to the same listener, which is not supported.
	RouteReasonMultipleRoutesOnListener v1.RouteConditionReason = "MultipleRoutesOnListener"

	// RouteReasonUnsupportedConfiguration is used when the associated Gateway does not support the Route.
	// Used with Accepted (false).
	RouteReasonUnsupportedConfiguration v1.RouteConditionReason = "UnsupportedConfiguration"

	// RouteReasonInvalidIPFamily is used when the Service associated with the Route is not configured with
	// the same IP family as the NGINX server.
	// Used with ResolvedRefs (false).
	RouteReasonInvalidIPFamily v1.RouteConditionReason = "InvalidServiceIPFamily"

	// RouteReasonInvalidFilter is used when an extension ref filter referenced by a Route cannot be resolved, or is
	// invalid. Used with ResolvedRefs (false).
	RouteReasonInvalidFilter v1.RouteConditionReason = "InvalidFilter"

	// RouteReasonInvalidInferencePool is used when a InferencePool backendRef referenced by a Route is invalid.
	RouteReasonInvalidInferencePool v1.RouteConditionReason = "InvalidInferencePool"

	// GatewayReasonUnsupportedField is used with the "Accepted" condition when a Gateway contains fields
	// that are not yet supported.
	GatewayReasonUnsupportedField v1.GatewayConditionReason = "UnsupportedField"

	// GatewayReasonUnsupportedValue is used with GatewayConditionAccepted (false) when a value of a field in a Gateway
	// is invalid or not supported.
	GatewayReasonUnsupportedValue v1.GatewayConditionReason = "UnsupportedValue"

	// GatewayMessageFailedNginxReload is a message used with GatewayConditionProgrammed (false)
	// when nginx fails to reload.
	GatewayMessageFailedNginxReload = "The Gateway is not programmed due to a failure to " +
		"reload nginx with the configuration"

	// RouteMessageFailedNginxReload is a message used with RouteReasonGatewayNotProgrammed
	// when nginx fails to reload.
	RouteMessageFailedNginxReload = GatewayMessageFailedNginxReload + ". NGINX may still be configured " +
		"for this Route. However, future updates to this resource will not be configured until the Gateway " +
		"is programmed again"

	// GatewayClassResolvedRefs condition indicates whether the controller was able to resolve the
	// parametersRef on the GatewayClass.
	GatewayClassResolvedRefs v1.GatewayClassConditionType = "ResolvedRefs"

	// GatewayClassReasonResolvedRefs is used with the "GatewayClassResolvedRefs" condition when the condition is true.
	GatewayClassReasonResolvedRefs v1.GatewayClassConditionReason = "ResolvedRefs"

	// GatewayClassReasonParamsRefNotFound is used with the "GatewayClassResolvedRefs" condition when the
	// parametersRef resource does not exist.
	GatewayClassReasonParamsRefNotFound v1.GatewayClassConditionReason = "ParametersRefNotFound"

	// GatewayClassReasonParamsRefInvalid is used with the "GatewayClassResolvedRefs" condition when the
	// parametersRef resource is invalid.
	GatewayClassReasonParamsRefInvalid v1.GatewayClassConditionReason = "ParametersRefInvalid"

	// PolicyReasonNginxProxyConfigNotSet is used with the "PolicyAccepted" condition when the
	// NginxProxy resource is missing or invalid.
	PolicyReasonNginxProxyConfigNotSet v1.PolicyConditionReason = "NginxProxyConfigNotSet"

	// PolicyMessageNginxProxyInvalid is a message used with the PolicyReasonNginxProxyConfigNotSet reason
	// when the NginxProxy resource is either invalid or not attached.
	PolicyMessageNginxProxyInvalid = "The NginxProxy configuration is either invalid or not attached to the GatewayClass"

	// PolicyMessageTelemetryNotEnabled is a message used with the PolicyReasonNginxProxyConfigNotSet reason
	// when telemetry is not enabled in the NginxProxy resource.
	PolicyMessageTelemetryNotEnabled = "Telemetry is not enabled in the NginxProxy resource"

	// PolicyReasonTargetConflict is used with the "PolicyAccepted" condition when a Route that it targets
	// has an overlapping hostname:port/path combination with another Route.
	PolicyReasonTargetConflict v1.PolicyConditionReason = "TargetConflict"

	// ClientSettingsPolicyAffected is used with the "PolicyAffected" condition when a
	// ClientSettingsPolicy is applied to a Gateway, HTTPRoute, or GRPCRoute.
	ClientSettingsPolicyAffected v1.PolicyConditionType = "ClientSettingsPolicyAffected"

	// ObservabilityPolicyAffected is used with the "PolicyAffected" condition when an
	// ObservabilityPolicy is applied to a HTTPRoute, or GRPCRoute.
	ObservabilityPolicyAffected v1.PolicyConditionType = "ObservabilityPolicyAffected"

	// PolicyAffectedReason is used with the "PolicyAffected" condition when a
	// ObservabilityPolicy or ClientSettingsPolicy is applied to Gateways or Routes.
	PolicyAffectedReason v1.PolicyConditionReason = "PolicyAffected"

	// GatewayResolvedRefs condition indicates whether the controller was able to resolve the
	// parametersRef on the Gateway.
	GatewayResolvedRefs v1.GatewayConditionType = "ResolvedRefs"

	// GatewayReasonResolvedRefs is used with the "GatewayResolvedRefs" condition when the condition is true.
	GatewayReasonResolvedRefs v1.GatewayConditionReason = "ResolvedRefs"

	// GatewayReasonParamsRefNotFound is used with the "GatewayResolvedRefs" condition when the
	// parametersRef resource does not exist.
	GatewayReasonParamsRefNotFound v1.GatewayConditionReason = "ParametersRefNotFound"

	// GatewayReasonParamsRefInvalid is used with the "GatewayResolvedRefs" condition when the
	// parametersRef resource is invalid.
	GatewayReasonParamsRefInvalid v1.GatewayConditionReason = "ParametersRefInvalid"

	// GatewayReasonSecretRefInvalid is used with the "GatewayResolvedRefs" condition when the
	// secretRef resource is invalid.
	GatewayReasonSecretRefInvalid v1.GatewayConditionReason = "SecretRefInvalid"

	// GatewayReasonSecretRefNotPermitted is used with the "GatewayResolvedRefs" condition when the
	// secretRef resource is not permitted by any ReferenceGrant.
	GatewayReasonSecretRefNotPermitted v1.GatewayConditionReason = "SecretRefNotPermitted"

	// PolicyReasonAncestorLimitReached is used with the "PolicyAccepted" condition when a policy
	// cannot be applied because the ancestor status list has reached the maximum size of 16.
	PolicyReasonAncestorLimitReached v1.PolicyConditionReason = "AncestorLimitReached"

	// PolicyMessageAncestorLimitReached is a message used with PolicyReasonAncestorLimitReached
	// when a policy cannot be applied due to the ancestor limit being reached.
	PolicyMessageAncestorLimitReached = "Policies cannot be applied because the ancestor status list " +
		"has reached the maximum size. The following policies have been ignored:"
)

// Condition defines a condition to be reported in the status of resources.
type Condition struct {
	Type    string
	Status  metav1.ConditionStatus
	Reason  string
	Message string
}

// DeduplicateConditions removes duplicate conditions based on the condition type.
// The last condition wins. The order of conditions is preserved.
func DeduplicateConditions(conds []Condition) []Condition {
	type elem struct {
		cond       Condition
		reverseIdx int
	}

	uniqueElems := make(map[string]elem)

	idx := 0
	for i := len(conds) - 1; i >= 0; i-- {
		if _, exist := uniqueElems[conds[i].Type]; exist {
			continue
		}

		uniqueElems[conds[i].Type] = elem{
			cond:       conds[i],
			reverseIdx: idx,
		}
		idx++
	}

	result := make([]Condition, len(uniqueElems))

	for _, el := range uniqueElems {
		result[len(result)-el.reverseIdx-1] = el.cond
	}

	return result
}

// ConvertConditions converts conditions to Kubernetes API conditions.
func ConvertConditions(
	conds []Condition,
	observedGeneration int64,
	transitionTime metav1.Time,
) []metav1.Condition {
	apiConds := make([]metav1.Condition, len(conds))

	for i := range conds {
		apiConds[i] = metav1.Condition{
			Type:               conds[i].Type,
			Status:             conds[i].Status,
			ObservedGeneration: observedGeneration,
			LastTransitionTime: transitionTime,
			Reason:             conds[i].Reason,
			Message:            conds[i].Message,
		}
	}

	return apiConds
}

// HasMatchingCondition checks if the given condition matches any of the existing conditions.
func HasMatchingCondition(existingConditions []Condition, cond Condition) bool {
	for _, existing := range existingConditions {
		if existing.Type == cond.Type &&
			existing.Status == cond.Status &&
			existing.Reason == cond.Reason &&
			existing.Message == cond.Message {
			return true
		}
	}
	return false
}

// NewDefaultGatewayClassConditions returns Conditions that indicate that the GatewayClass is accepted and that the
// Gateway API CRD versions are supported.
func NewDefaultGatewayClassConditions() []Condition {
	return []Condition{
		{
			Type:    string(v1.GatewayClassConditionStatusAccepted),
			Status:  metav1.ConditionTrue,
			Reason:  string(v1.GatewayClassReasonAccepted),
			Message: "The GatewayClass is accepted",
		},
		{
			Type:    string(v1.GatewayClassConditionStatusSupportedVersion),
			Status:  metav1.ConditionTrue,
			Reason:  string(v1.GatewayClassReasonSupportedVersion),
			Message: "The Gateway API CRD versions are supported",
		},
	}
}

// NewGatewayClassSupportedVersionBestEffort returns a Condition that indicates that the GatewayClass is accepted,
// but the Gateway API CRD versions are not supported. This means NGF will attempt to generate configuration,
// but it does not guarantee support.
func NewGatewayClassSupportedVersionBestEffort(recommendedVersion string) []Condition {
	return []Condition{
		{
			Type:   string(v1.GatewayClassConditionStatusSupportedVersion),
			Status: metav1.ConditionFalse,
			Reason: string(v1.GatewayClassReasonUnsupportedVersion),
			Message: fmt.Sprintf(
				"The Gateway API CRD versions are not recommended. Recommended version is %s",
				recommendedVersion,
			),
		},
	}
}

// NewGatewayClassUnsupportedVersion returns Conditions that indicate that the GatewayClass is not accepted because
// the Gateway API CRD versions are not supported. NGF will not generate configuration in this case.
func NewGatewayClassUnsupportedVersion(recommendedVersion string) []Condition {
	return []Condition{
		{
			Type:   string(v1.GatewayClassConditionStatusAccepted),
			Status: metav1.ConditionFalse,
			Reason: string(v1.GatewayClassReasonUnsupportedVersion),
			Message: fmt.Sprintf(
				"The Gateway API CRD versions are not supported. Please install version %s",
				recommendedVersion,
			),
		},
		{
			Type:   string(v1.GatewayClassConditionStatusSupportedVersion),
			Status: metav1.ConditionFalse,
			Reason: string(v1.GatewayClassReasonUnsupportedVersion),
			Message: fmt.Sprintf(
				"The Gateway API CRD versions are not supported. Please install version %s",
				recommendedVersion,
			),
		},
	}
}

// NewGatewaySecretRefNotPermitted returns Condition that indicates that the Gateway references a TLS secret that is not
// permitted by any ReferenceGrant.
func NewGatewaySecretRefNotPermitted(msg string) Condition {
	return Condition{
		Type:    string(GatewayReasonResolvedRefs),
		Status:  metav1.ConditionFalse,
		Reason:  string(GatewayReasonSecretRefNotPermitted),
		Message: msg,
	}
}

// NewGatewaySecretRefInvalid returns Condition that indicates that the Gateway references a TLS secret that is invalid.
func NewGatewaySecretRefInvalid(msg string) Condition {
	return Condition{
		Type:    string(GatewayReasonResolvedRefs),
		Status:  metav1.ConditionFalse,
		Reason:  string(GatewayReasonSecretRefInvalid),
		Message: msg,
	}
}

// NewGatewayClassConflict returns a Condition that indicates that the GatewayClass is not accepted
// due to a conflict with another GatewayClass.
func NewGatewayClassConflict() Condition {
	return Condition{
		Type:    string(v1.GatewayClassConditionStatusAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(GatewayClassReasonGatewayClassConflict),
		Message: GatewayClassMessageGatewayClassConflict,
	}
}

// NewDefaultRouteConditions returns the default conditions that must be present in the status of a Route.
func NewDefaultRouteConditions() []Condition {
	return []Condition{
		NewRouteAccepted(),
		NewRouteResolvedRefs(),
	}
}

// NewRouteNotAllowedByListeners returns a Condition that indicates that the Route is not allowed by
// any listener.
func NewRouteNotAllowedByListeners() Condition {
	return Condition{
		Type:    string(v1.RouteConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.RouteReasonNotAllowedByListeners),
		Message: "The Route is not allowed by any listener",
	}
}

// NewRouteNoMatchingListenerHostname returns a Condition that indicates that the hostname of the Listener
// does not match the hostnames of the Route.
func NewRouteNoMatchingListenerHostname() Condition {
	return Condition{
		Type:    string(v1.RouteConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.RouteReasonNoMatchingListenerHostname),
		Message: "The Listener hostname does not match the Route hostnames",
	}
}

// NewRouteAccepted returns a Condition that indicates that the Route is accepted.
func NewRouteAccepted() Condition {
	return Condition{
		Type:    string(v1.RouteConditionAccepted),
		Status:  metav1.ConditionTrue,
		Reason:  string(v1.RouteReasonAccepted),
		Message: "The Route is accepted",
	}
}

// NewRouteUnsupportedValue returns a Condition that indicates that the Route includes an unsupported value.
func NewRouteUnsupportedValue(msg string) Condition {
	return Condition{
		Type:    string(v1.RouteConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.RouteReasonUnsupportedValue),
		Message: msg,
	}
}

// NewRouteAcceptedUnsupportedField returns a Condition that indicates that the Route is accepted but
// includes an unsupported field.
func NewRouteAcceptedUnsupportedField(msg string) Condition {
	return Condition{
		Type:    string(v1.RouteConditionAccepted),
		Status:  metav1.ConditionTrue,
		Reason:  string(RouteReasonUnsupportedField),
		Message: fmt.Sprintf("The following unsupported parameters were ignored: %s", msg),
	}
}

// NewRoutePartiallyInvalid returns a Condition that indicates that the Route contains a combination
// of both valid and invalid rules.
//
// // nolint:lll
// The message must start with "Dropped Rules(s)" according to the Gateway API spec
// See https://github.com/kubernetes-sigs/gateway-api/blob/37d81593e5a965ed76582dbc1a2f56bbd57c0622/apis/v1/shared_types.go#L408-L413
func NewRoutePartiallyInvalid(msg string) Condition {
	return Condition{
		Type:    string(v1.RouteConditionPartiallyInvalid),
		Status:  metav1.ConditionTrue,
		Reason:  string(v1.RouteReasonUnsupportedValue),
		Message: "Dropped Rule(s): " + msg,
	}
}

// NewRouteInvalidListener returns a Condition that indicates that the Route is not accepted because of an
// invalid listener.
func NewRouteInvalidListener() Condition {
	return Condition{
		Type:    string(v1.RouteConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(RouteReasonInvalidListener),
		Message: "The Listener is invalid for this parent ref",
	}
}

// NewRouteHostnameConflict returns a Condition that indicates that the Route is not accepted because of a
// conflicting hostname on the same port.
func NewRouteHostnameConflict() Condition {
	return Condition{
		Type:    string(v1.RouteConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(RouteReasonHostnameConflict),
		Message: "Hostname(s) conflict with another Route of the same kind on the same port",
	}
}

// NewRouteMultipleRoutesOnListener returns a Condition that indicates that the Route is not
// accepted because of multiple.L4 Routes attached to the same listener, which is not supported.
func NewRouteMultipleRoutesOnListener() Condition {
	return Condition{
		Type:    string(v1.RouteConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(RouteReasonMultipleRoutesOnListener),
		Message: "Multiple L4 Routes are attached to the same listener, which is not supported",
	}
}

// NewRouteResolvedRefs returns a Condition that indicates that all the references on the Route are resolved.
func NewRouteResolvedRefs() Condition {
	return Condition{
		Type:    string(v1.RouteConditionResolvedRefs),
		Status:  metav1.ConditionTrue,
		Reason:  string(v1.RouteReasonResolvedRefs),
		Message: "All references are resolved",
	}
}

// NewRouteBackendRefInvalidKind returns a Condition that indicates that the Route has a backendRef with an
// invalid kind.
func NewRouteBackendRefInvalidKind(msg string) Condition {
	return Condition{
		Type:    string(v1.RouteConditionResolvedRefs),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.RouteReasonInvalidKind),
		Message: msg,
	}
}

// NewRouteBackendRefRefNotPermitted returns a Condition that indicates that the Route has a backendRef that
// is not permitted.
func NewRouteBackendRefRefNotPermitted(msg string) Condition {
	return Condition{
		Type:    string(v1.RouteConditionResolvedRefs),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.RouteReasonRefNotPermitted),
		Message: msg,
	}
}

// NewRouteBackendRefRefBackendNotFound returns a Condition that indicates that the Route has a backendRef that
// points to non-existing backend.
func NewRouteBackendRefRefBackendNotFound(msg string) Condition {
	return Condition{
		Type:    string(v1.RouteConditionResolvedRefs),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.RouteReasonBackendNotFound),
		Message: msg,
	}
}

// NewRouteBackendRefUnsupportedValue returns a Condition that indicates that the Route has a backendRef with
// an unsupported value.
func NewRouteBackendRefUnsupportedValue(msg string) Condition {
	return Condition{
		Type:    string(v1.RouteConditionResolvedRefs),
		Status:  metav1.ConditionFalse,
		Reason:  string(RouteReasonBackendRefUnsupportedValue),
		Message: msg,
	}
}

// NewRouteBackendRefInvalidInferencePool returns a Condition that indicates that the Route has a InferencePool
// backendRef that is invalid.
func NewRouteBackendRefInvalidInferencePool(msg string) Condition {
	return Condition{
		Type:    string(v1.RouteConditionResolvedRefs),
		Status:  metav1.ConditionFalse,
		Reason:  string(RouteReasonInvalidInferencePool),
		Message: msg,
	}
}

// NewRouteBackendRefUnsupportedProtocol returns a Condition that indicates that the Route has a backendRef with
// an unsupported protocol.
func NewRouteBackendRefUnsupportedProtocol(msg string) Condition {
	return Condition{
		Type:    string(v1.RouteConditionResolvedRefs),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.RouteReasonUnsupportedProtocol),
		Message: msg,
	}
}

// NewRouteInvalidGateway returns a Condition that indicates that the Route is not Accepted because the Gateway it
// references is invalid.
func NewRouteInvalidGateway() Condition {
	return Condition{
		Type:    string(v1.RouteConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(RouteReasonInvalidGateway),
		Message: "The Gateway is invalid",
	}
}

// NewRouteNoMatchingParent returns a Condition that indicates that the Route is not Accepted because
// it specifies a Port and/or SectionName that does not match any Listeners in the Gateway.
func NewRouteNoMatchingParent() Condition {
	return Condition{
		Type:    string(v1.RouteConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.RouteReasonNoMatchingParent),
		Message: "The Listener is not found for this parent ref",
	}
}

// NewRouteUnsupportedConfiguration returns a Condition that indicates that the Route is not Accepted because
// it is incompatible with the Gateway's configuration.
func NewRouteUnsupportedConfiguration(msg string) Condition {
	return Condition{
		Type:    string(v1.RouteConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(RouteReasonUnsupportedConfiguration),
		Message: msg,
	}
}

// NewRouteInvalidIPFamily returns a Condition that indicates that the Service associated with the Route
// is not configured with the same IP family as the NGINX server.
func NewRouteInvalidIPFamily(msg string) Condition {
	return Condition{
		Type:    string(v1.RouteConditionResolvedRefs),
		Status:  metav1.ConditionFalse,
		Reason:  string(RouteReasonInvalidIPFamily),
		Message: msg,
	}
}

// NewRouteResolvedRefsInvalidFilter returns a Condition that indicates that the Route has a filter that
// cannot be resolved or is invalid.
func NewRouteResolvedRefsInvalidFilter(msg string) Condition {
	return Condition{
		Type:    string(v1.RouteConditionResolvedRefs),
		Status:  metav1.ConditionFalse,
		Reason:  string(RouteReasonInvalidFilter),
		Message: msg,
	}
}

// NewDefaultListenerConditions returns the default Conditions that must be present in the status of a Listener.
// If existingConditions contains conflict-related conditions (like OverlappingTLSConfig or Conflicted),
// the NoConflicts condition is excluded to avoid conflicting condition states.
func NewDefaultListenerConditions(existingConditions []Condition) []Condition {
	defaultConds := []Condition{
		NewListenerAccepted(),
		NewListenerProgrammed(),
		NewListenerResolvedRefs(),
	}

	// Only add NoConflicts condition if there are no existing conflict-related conditions
	if !hasConflictConditions(existingConditions) {
		defaultConds = append(defaultConds, NewListenerNoConflicts())
	}

	return defaultConds
}

// hasConflictConditions checks if the Listener has any conflict-related conditions.
func hasConflictConditions(conditions []Condition) bool {
	for _, cond := range conditions {
		if cond.Type == string(v1.ListenerConditionConflicted) ||
			cond.Type == string(v1.ListenerConditionOverlappingTLSConfig) {
			return true
		}
	}
	return false
}

// NewListenerAccepted returns a Condition that indicates that the Listener is accepted.
func NewListenerAccepted() Condition {
	return Condition{
		Type:    string(v1.ListenerConditionAccepted),
		Status:  metav1.ConditionTrue,
		Reason:  string(v1.ListenerReasonAccepted),
		Message: "The Listener is accepted",
	}
}

// NewListenerProgrammed returns a Condition that indicates the Listener is programmed.
func NewListenerProgrammed() Condition {
	return Condition{
		Type:    string(v1.ListenerConditionProgrammed),
		Status:  metav1.ConditionTrue,
		Reason:  string(v1.ListenerReasonProgrammed),
		Message: "The Listener is programmed",
	}
}

// NewListenerResolvedRefs returns a Condition that indicates that all references in a Listener are resolved.
func NewListenerResolvedRefs() Condition {
	return Condition{
		Type:    string(v1.ListenerConditionResolvedRefs),
		Status:  metav1.ConditionTrue,
		Reason:  string(v1.ListenerReasonResolvedRefs),
		Message: "All references are resolved",
	}
}

// NewListenerNoConflicts returns a Condition that indicates that there are no conflicts in a Listener.
func NewListenerNoConflicts() Condition {
	return Condition{
		Type:    string(v1.ListenerConditionConflicted),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.ListenerReasonNoConflicts),
		Message: "No conflicts",
	}
}

// NewListenerNotProgrammedInvalid returns a Condition that indicates the Listener is not programmed because it is
// semantically or syntactically invalid. The provided message contains the details of why the Listener is invalid.
func NewListenerNotProgrammedInvalid(msg string) Condition {
	return Condition{
		Type:    string(v1.ListenerConditionProgrammed),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.ListenerReasonInvalid),
		Message: msg,
	}
}

// NewListenerUnsupportedValue returns Conditions that indicate that a field of a Listener has an unsupported value.
// Unsupported means that the value is not supported by the implementation or invalid.
func NewListenerUnsupportedValue(msg string) []Condition {
	return []Condition{
		{
			Type:    string(v1.ListenerConditionAccepted),
			Status:  metav1.ConditionFalse,
			Reason:  string(ListenerReasonUnsupportedValue),
			Message: msg,
		},
		NewListenerNotProgrammedInvalid(msg),
	}
}

// NewListenerInvalidCertificateRef returns Conditions that indicate that a CertificateRef of a Listener is invalid.
func NewListenerInvalidCertificateRef(msg string) []Condition {
	return []Condition{
		{
			Type:    string(v1.ListenerConditionAccepted),
			Status:  metav1.ConditionFalse,
			Reason:  string(v1.ListenerReasonInvalidCertificateRef),
			Message: msg,
		},
		{
			Type:    string(v1.ListenerReasonResolvedRefs),
			Status:  metav1.ConditionFalse,
			Reason:  string(v1.ListenerReasonInvalidCertificateRef),
			Message: msg,
		},
		NewListenerNotProgrammedInvalid(msg),
	}
}

// NewListenerInvalidRouteKinds returns Conditions that indicate that an invalid or unsupported Route kind is
// specified by the Listener.
func NewListenerInvalidRouteKinds(msg string) []Condition {
	return []Condition{
		{
			Type:    string(v1.ListenerReasonResolvedRefs),
			Status:  metav1.ConditionFalse,
			Reason:  string(v1.ListenerReasonInvalidRouteKinds),
			Message: msg,
		},
		NewListenerNotProgrammedInvalid(msg),
	}
}

// NewListenerProtocolConflict returns Conditions that indicate multiple Listeners are specified with the same
// Listener port number, but have conflicting protocol specifications.
func NewListenerProtocolConflict(msg string) []Condition {
	return []Condition{
		{
			Type:    string(v1.ListenerConditionAccepted),
			Status:  metav1.ConditionFalse,
			Reason:  string(v1.ListenerReasonProtocolConflict),
			Message: msg,
		},
		{
			Type:    string(v1.ListenerConditionConflicted),
			Status:  metav1.ConditionTrue,
			Reason:  string(v1.ListenerReasonProtocolConflict),
			Message: msg,
		},
		NewListenerNotProgrammedInvalid(msg),
	}
}

// NewListenerHostnameConflict returns Conditions that indicate multiple Listeners are specified with the same
// Listener port, but are HTTPS and TLS and have overlapping hostnames.
func NewListenerHostnameConflict(msg string) []Condition {
	return []Condition{
		{
			Type:    string(v1.ListenerConditionAccepted),
			Status:  metav1.ConditionFalse,
			Reason:  string(v1.ListenerReasonHostnameConflict),
			Message: msg,
		},
		{
			Type:    string(v1.ListenerConditionConflicted),
			Status:  metav1.ConditionTrue,
			Reason:  string(v1.ListenerReasonHostnameConflict),
			Message: msg,
		},
		NewListenerNotProgrammedInvalid(msg),
	}
}

// NewListenerUnsupportedProtocol returns Conditions that indicate that the protocol of a Listener is unsupported.
func NewListenerUnsupportedProtocol(msg string) []Condition {
	return []Condition{
		{
			Type:    string(v1.ListenerConditionAccepted),
			Status:  metav1.ConditionFalse,
			Reason:  string(v1.ListenerReasonUnsupportedProtocol),
			Message: msg,
		},
		NewListenerNotProgrammedInvalid(msg),
	}
}

// NewListenerRefNotPermitted returns Conditions that indicates that the Listener references a TLS secret that is not
// permitted by a ReferenceGrant.
func NewListenerRefNotPermitted(msg string) []Condition {
	return []Condition{
		{
			Type:    string(v1.ListenerConditionAccepted),
			Status:  metav1.ConditionFalse,
			Reason:  string(v1.ListenerReasonRefNotPermitted),
			Message: msg,
		},
		{
			Type:    string(v1.ListenerReasonResolvedRefs),
			Status:  metav1.ConditionFalse,
			Reason:  string(v1.ListenerReasonRefNotPermitted),
			Message: msg,
		},
		NewListenerNotProgrammedInvalid(msg),
	}
}

// NewListenerOverlappingTLSConfig returns a Condition that indicates overlapping TLS configuration
// between Listeners on the same port.
func NewListenerOverlappingTLSConfig(reason v1.ListenerConditionReason, msg string) Condition {
	return Condition{
		Type:    string(v1.ListenerConditionOverlappingTLSConfig),
		Status:  metav1.ConditionTrue,
		Reason:  string(reason),
		Message: msg,
	}
}

// NewGatewayClassResolvedRefs returns a Condition that indicates that the parametersRef
// on the GatewayClass is resolved.
func NewGatewayClassResolvedRefs() Condition {
	return Condition{
		Type:    string(GatewayClassResolvedRefs),
		Status:  metav1.ConditionTrue,
		Reason:  string(GatewayClassReasonResolvedRefs),
		Message: "The ParametersRef resource is resolved",
	}
}

// NewGatewayClassRefNotFound returns a Condition that indicates that the parametersRef
// on the GatewayClass could not be resolved.
func NewGatewayClassRefNotFound() Condition {
	return Condition{
		Type:    string(GatewayClassResolvedRefs),
		Status:  metav1.ConditionFalse,
		Reason:  string(GatewayClassReasonParamsRefNotFound),
		Message: "The ParametersRef resource could not be found",
	}
}

// NewGatewayClassRefInvalid returns a Condition that indicates that the parametersRef
// on the GatewayClass could not be resolved because the resource it references is invalid.
func NewGatewayClassRefInvalid(msg string) Condition {
	return Condition{
		Type:    string(GatewayClassResolvedRefs),
		Status:  metav1.ConditionFalse,
		Reason:  string(GatewayClassReasonParamsRefInvalid),
		Message: msg,
	}
}

// NewGatewayClassInvalidParameters returns a Condition that indicates that the GatewayClass has invalid parameters.
// We are allowing Accepted to still be true to prevent nullifying the entire config tree if a parametersRef
// is updated to something invalid.
func NewGatewayClassInvalidParameters(msg string) Condition {
	return Condition{
		Type:    string(v1.GatewayClassConditionStatusAccepted),
		Status:  metav1.ConditionTrue,
		Reason:  string(v1.GatewayClassReasonInvalidParameters),
		Message: fmt.Sprintf("The GatewayClass is accepted, but ParametersRef is ignored due to an error: %s", msg),
	}
}

// NewDefaultGatewayConditions returns the default Conditions that must be present in the status of a Gateway.
func NewDefaultGatewayConditions() []Condition {
	return []Condition{
		NewGatewayAccepted(),
		NewGatewayProgrammed(),
	}
}

// NewGatewayAccepted returns a Condition that indicates the Gateway is accepted.
func NewGatewayAccepted() Condition {
	return Condition{
		Type:    string(v1.GatewayConditionAccepted),
		Status:  metav1.ConditionTrue,
		Reason:  string(v1.GatewayReasonAccepted),
		Message: "The Gateway is accepted",
	}
}

// NewGatewayAcceptedListenersNotValid returns a Condition that indicates the Gateway is accepted,
// but has at least one listener that is invalid.
func NewGatewayAcceptedListenersNotValid() Condition {
	return Condition{
		Type:    string(v1.GatewayConditionAccepted),
		Status:  metav1.ConditionTrue,
		Reason:  string(v1.GatewayReasonListenersNotValid),
		Message: "The Gateway has at least one valid listener",
	}
}

// NewGatewayNotAcceptedListenersNotValid returns Conditions that indicate the Gateway is not accepted,
// because all listeners are invalid.
func NewGatewayNotAcceptedListenersNotValid() []Condition {
	msg := "The Gateway has no valid listeners"
	return []Condition{
		{
			Type:    string(v1.GatewayConditionAccepted),
			Status:  metav1.ConditionFalse,
			Reason:  string(v1.GatewayReasonListenersNotValid),
			Message: msg,
		},
		NewGatewayNotProgrammedInvalid(msg),
	}
}

// NewGatewayInvalid returns Conditions that indicate the Gateway is not accepted and programmed because it is
// semantically or syntactically invalid. The provided message contains the details of why the Gateway is invalid.
func NewGatewayInvalid(msg string) []Condition {
	return []Condition{
		{
			Type:    string(v1.GatewayConditionAccepted),
			Status:  metav1.ConditionFalse,
			Reason:  string(v1.GatewayReasonInvalid),
			Message: msg,
		},
		NewGatewayNotProgrammedInvalid(msg),
	}
}

// NewGatewayUnsupportedValue returns Conditions that indicate that a field of the Gateway has an unsupported value.
// Unsupported means that the value is not supported by the implementation under certain conditions or invalid.
func NewGatewayUnsupportedValue(msg string) []Condition {
	return []Condition{
		{
			Type:    string(v1.GatewayConditionAccepted),
			Status:  metav1.ConditionFalse,
			Reason:  string(GatewayReasonUnsupportedValue),
			Message: msg,
		},
		{
			Type:    string(v1.GatewayConditionProgrammed),
			Status:  metav1.ConditionFalse,
			Reason:  string(GatewayReasonUnsupportedValue),
			Message: msg,
		},
	}
}

// NewGatewayUnsupportedAddress returns a Condition that indicates the Gateway is not accepted because it
// contains an address type that is not supported.
func NewGatewayUnsupportedAddress(msg string) Condition {
	return Condition{
		Type:    string(v1.GatewayConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.GatewayReasonUnsupportedAddress),
		Message: msg,
	}
}

// NewGatewayUnusableAddress returns a Condition that indicates the Gateway is not programmed because it
// contains an address type that can't be used.
func NewGatewayUnusableAddress(msg string) Condition {
	return Condition{
		Type:    string(v1.GatewayConditionProgrammed),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.GatewayReasonAddressNotUsable),
		Message: msg,
	}
}

// NewGatewayAddressNotAssigned returns a Condition that indicates the Gateway is not programmed because it
// has not assigned an address for the Gateway.
func NewGatewayAddressNotAssigned(msg string) Condition {
	return Condition{
		Type:    string(v1.GatewayConditionProgrammed),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.GatewayReasonAddressNotAssigned),
		Message: msg,
	}
}

// NewGatewayProgrammed returns a Condition that indicates the Gateway is programmed.
func NewGatewayProgrammed() Condition {
	return Condition{
		Type:    string(v1.GatewayConditionProgrammed),
		Status:  metav1.ConditionTrue,
		Reason:  string(v1.GatewayReasonProgrammed),
		Message: "The Gateway is programmed",
	}
}

// NewGatewayNotProgrammedInvalid returns a Condition that indicates the Gateway is not programmed
// because it is semantically or syntactically invalid. The provided message contains the details of
// why the Gateway is invalid.
func NewGatewayNotProgrammedInvalid(msg string) Condition {
	return Condition{
		Type:    string(v1.GatewayConditionProgrammed),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.GatewayReasonInvalid),
		Message: msg,
	}
}

// NewNginxGatewayValid returns a Condition that indicates that the NginxGateway config is valid.
func NewNginxGatewayValid() Condition {
	return Condition{
		Type:    string(ngfAPI.NginxGatewayConditionValid),
		Status:  metav1.ConditionTrue,
		Reason:  string(ngfAPI.NginxGatewayReasonValid),
		Message: "The NginxGateway is valid",
	}
}

// NewNginxGatewayInvalid returns a Condition that indicates that the NginxGateway config is invalid.
func NewNginxGatewayInvalid(msg string) Condition {
	return Condition{
		Type:    string(ngfAPI.NginxGatewayConditionValid),
		Status:  metav1.ConditionFalse,
		Reason:  string(ngfAPI.NginxGatewayReasonInvalid),
		Message: msg,
	}
}

// NewGatewayResolvedRefs returns a Condition that indicates that the parametersRef
// on the Gateway is resolved.
func NewGatewayResolvedRefs() Condition {
	return Condition{
		Type:    string(GatewayResolvedRefs),
		Status:  metav1.ConditionTrue,
		Reason:  string(GatewayReasonResolvedRefs),
		Message: "The ParametersRef resource is resolved",
	}
}

// NewGatewayRefNotFound returns a Condition that indicates that the parametersRef
// on the Gateway could not be resolved.
func NewGatewayRefNotFound() Condition {
	return Condition{
		Type:    string(GatewayResolvedRefs),
		Status:  metav1.ConditionFalse,
		Reason:  string(GatewayReasonParamsRefNotFound),
		Message: "The ParametersRef resource could not be found",
	}
}

// NewGatewayRefInvalid returns a Condition that indicates that the parametersRef
// on the Gateway could not be resolved because the referenced resource is invalid.
func NewGatewayRefInvalid(msg string) Condition {
	return Condition{
		Type:    string(GatewayResolvedRefs),
		Status:  metav1.ConditionFalse,
		Reason:  string(GatewayReasonParamsRefInvalid),
		Message: msg,
	}
}

// NewGatewayInvalidParameters returns a Condition that indicates that the Gateway has invalid parameters.
// We are allowing Accepted to still be true to prevent nullifying the entire Gateway config if a parametersRef
// is updated to something invalid.
func NewGatewayInvalidParameters(msg string) Condition {
	return Condition{
		Type:    string(v1.GatewayConditionAccepted),
		Status:  metav1.ConditionTrue,
		Reason:  string(v1.GatewayReasonInvalidParameters),
		Message: fmt.Sprintf("The Gateway is accepted, but ParametersRef is ignored due to an error: %s", msg),
	}
}

// NewGatewayAcceptedUnsupportedField returns a Condition that indicates the Gateway is accepted but
// contains a field that is not supported.
func NewGatewayAcceptedUnsupportedField(msg string) Condition {
	return Condition{
		Type:    string(v1.GatewayConditionAccepted),
		Status:  metav1.ConditionTrue,
		Reason:  string(GatewayReasonUnsupportedField),
		Message: fmt.Sprintf("The Gateway is accepted but the following unsupported parameters were ignored: %s", msg),
	}
}

// NewPolicyAccepted returns a Condition that indicates that the Policy is accepted.
func NewPolicyAccepted() Condition {
	return Condition{
		Type:    string(v1.PolicyConditionAccepted),
		Status:  metav1.ConditionTrue,
		Reason:  string(v1.PolicyReasonAccepted),
		Message: "The Policy is accepted",
	}
}

// NewPolicyInvalid returns a Condition that indicates that the Policy is not accepted because it is semantically or
// syntactically invalid.
func NewPolicyInvalid(msg string) Condition {
	return Condition{
		Type:    string(v1.PolicyConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.PolicyReasonInvalid),
		Message: msg,
	}
}

// NewPolicyConflicted returns a Condition that indicates that the Policy is not accepted because it conflicts with
// another Policy and a merge is not possible.
func NewPolicyConflicted(msg string) Condition {
	return Condition{
		Type:    string(v1.PolicyConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.PolicyReasonConflicted),
		Message: msg,
	}
}

// NewPolicyTargetNotFound returns a Condition that indicates that the Policy is not accepted because the target
// resource does not exist or can not be attached to.
func NewPolicyTargetNotFound(msg string) Condition {
	return Condition{
		Type:    string(v1.PolicyConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.PolicyReasonTargetNotFound),
		Message: msg,
	}
}

// NewPolicyAncestorLimitReached returns a Condition that indicates that the Policy is not accepted because
// the ancestor status list has reached the maximum size of 16.
func NewPolicyAncestorLimitReached(policyType string, policyName string) Condition {
	return Condition{
		Type:    string(v1.PolicyConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(PolicyReasonAncestorLimitReached),
		Message: fmt.Sprintf("%s %s %s", PolicyMessageAncestorLimitReached, policyType, policyName),
	}
}

// NewPolicyNotAcceptedTargetConflict returns a Condition that indicates that the Policy is not accepted
// because the target resource has a conflict with another resource when attempting to apply this policy.
func NewPolicyNotAcceptedTargetConflict(msg string) Condition {
	return Condition{
		Type:    string(v1.PolicyConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(PolicyReasonTargetConflict),
		Message: msg,
	}
}

// NewPolicyNotAcceptedNginxProxyNotSet returns a Condition that indicates that the Policy is not accepted
// because it relies on the NginxProxy configuration which is missing or invalid.
func NewPolicyNotAcceptedNginxProxyNotSet(msg string) Condition {
	return Condition{
		Type:    string(v1.PolicyConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(PolicyReasonNginxProxyConfigNotSet),
		Message: msg,
	}
}

// NewSnippetsFilterInvalid returns a Condition that indicates that the SnippetsFilter is not accepted because it is
// syntactically or semantically invalid.
func NewSnippetsFilterInvalid(msg string) Condition {
	return Condition{
		Type:    string(ngfAPI.SnippetsFilterConditionTypeAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(ngfAPI.SnippetsFilterConditionReasonInvalid),
		Message: msg,
	}
}

// NewSnippetsFilterAccepted returns a Condition that indicates that the SnippetsFilter is accepted because it is
// valid.
func NewSnippetsFilterAccepted() Condition {
	return Condition{
		Type:    string(ngfAPI.SnippetsFilterConditionTypeAccepted),
		Status:  metav1.ConditionTrue,
		Reason:  string(ngfAPI.SnippetsFilterConditionReasonAccepted),
		Message: "The SnippetsFilter is accepted",
	}
}

// NewObservabilityPolicyAffected returns a Condition that indicates that an ObservabilityPolicy
// is applied to the resource.
func NewObservabilityPolicyAffected() Condition {
	return Condition{
		Type:    string(ObservabilityPolicyAffected),
		Status:  metav1.ConditionTrue,
		Reason:  string(PolicyAffectedReason),
		Message: "The ObservabilityPolicy is applied to the resource",
	}
}

// NewClientSettingsPolicyAffected returns a Condition that indicates that a ClientSettingsPolicy
// is applied to the resource.
func NewClientSettingsPolicyAffected() Condition {
	return Condition{
		Type:    string(ClientSettingsPolicyAffected),
		Status:  metav1.ConditionTrue,
		Reason:  string(PolicyAffectedReason),
		Message: "The ClientSettingsPolicy is applied to the resource",
	}
}

// NewBackendTLSPolicyResolvedRefs returns a Condition that indicates that all CACertificateRefs
// in the BackendTLSPolicy are resolved.
func NewBackendTLSPolicyResolvedRefs() Condition {
	return Condition{
		Type:    string(GatewayResolvedRefs),
		Status:  metav1.ConditionTrue,
		Reason:  string(GatewayResolvedRefs),
		Message: "All CACertificateRefs are resolved",
	}
}

// NewBackendTLSPolicyInvalidCACertificateRef returns a Condition that indicates that a
// CACertificateRef in the BackendTLSPolicy refers to a resource that cannot be resolved or is misconfigured.
func NewBackendTLSPolicyInvalidCACertificateRef(message string) Condition {
	return Condition{
		Type:    string(GatewayResolvedRefs),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.BackendTLSPolicyReasonInvalidCACertificateRef),
		Message: message,
	}
}

// NewBackendTLSPolicyInvalidKind returns a Condition that indicates that a CACertificateRef
// in the BackendTLSPolicy refers to an unknown or unsupported kind of resource.
func NewBackendTLSPolicyInvalidKind(message string) Condition {
	return Condition{
		Type:    string(GatewayResolvedRefs),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.BackendTLSPolicyReasonInvalidKind),
		Message: message,
	}
}

// NewBackendTLSPolicyNoValidCACertificate returns a Condition that indicates that all
// CACertificateRefs in the BackendTLSPolicy are invalid.
func NewBackendTLSPolicyNoValidCACertificate(message string) Condition {
	return Condition{
		Type:    string(v1.PolicyConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.BackendTLSPolicyReasonNoValidCACertificate),
		Message: message,
	}
}

// NewInferencePoolAccepted returns a Condition that indicates that the InferencePool is accepted by the Gateway.
func NewInferencePoolAccepted() Condition {
	return Condition{
		Type:    string(inference.InferencePoolConditionAccepted),
		Status:  metav1.ConditionTrue,
		Reason:  string(inference.InferencePoolConditionAccepted),
		Message: "The InferencePool is accepted by the Gateway.",
	}
}

// NewInferencePoolResolvedRefs returns a Condition that
// indicates that all references in the InferencePool are resolved.
func NewInferencePoolResolvedRefs() Condition {
	return Condition{
		Type:    string(inference.InferencePoolConditionResolvedRefs),
		Status:  metav1.ConditionTrue,
		Reason:  string(inference.InferencePoolConditionResolvedRefs),
		Message: "The InferencePool references a valid ExtensionRef.",
	}
}

// NewDefaultInferenceConditions returns the default Conditions
// that must be present in the status of an InferencePool.
func NewDefaultInferenceConditions() []Condition {
	return []Condition{
		NewInferencePoolAccepted(),
		NewInferencePoolResolvedRefs(),
	}
}

// NewInferencePoolInvalidHTTPRouteNotAccepted returns a Condition that indicates that the InferencePool is not
// accepted because the associated HTTPRoute is not accepted by the Gateway.
func NewInferencePoolInvalidHTTPRouteNotAccepted(msg string) Condition {
	return Condition{
		Type:    string(inference.InferencePoolConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(inference.InferencePoolReasonHTTPRouteNotAccepted),
		Message: msg,
	}
}

// NewInferencePoolInvalidExtensionref returns a Condition that indicates that the InferencePool is not
// accepted because the ExtensionRef is invalid.
func NewInferencePoolInvalidExtensionref(msg string) Condition {
	return Condition{
		Type:    string(inference.InferencePoolConditionResolvedRefs),
		Status:  metav1.ConditionFalse,
		Reason:  string(inference.InferencePoolReasonInvalidExtensionRef),
		Message: msg,
	}
}

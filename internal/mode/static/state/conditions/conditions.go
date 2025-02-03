package conditions

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/conditions"
)

const (
	// ListenerReasonUnsupportedValue is used with the "Accepted" condition when a value of a field in a Listener
	// is invalid or not supported.
	ListenerReasonUnsupportedValue v1.ListenerConditionReason = "UnsupportedValue"

	// ListenerMessageFailedNginxReload is a message used with ListenerConditionProgrammed (false)
	// when nginx fails to reload.
	ListenerMessageFailedNginxReload = "The Listener is not programmed due to a failure to " +
		"reload nginx with the configuration. Please see the nginx container logs for any possible configuration issues."

	// RouteReasonBackendRefUnsupportedValue is used with the "ResolvedRefs" condition when one of the
	// Route rules has a backendRef with an unsupported value.
	RouteReasonBackendRefUnsupportedValue v1.RouteConditionReason = "UnsupportedValue"

	// RouteReasonInvalidGateway is used with the "Accepted" (false) condition when the Gateway the Route
	// references is invalid.
	RouteReasonInvalidGateway v1.RouteConditionReason = "InvalidGateway"

	// RouteReasonInvalidListener is used with the "Accepted" condition when the Route references an invalid listener.
	RouteReasonInvalidListener v1.RouteConditionReason = "InvalidListener"

	// RouteReasonHostnameConflict is used with the "Accepted" condition when a route has the exact same hostname
	// as another route.
	RouteReasonHostnameConflict v1.RouteConditionReason = "HostnameConflict"

	// RouteReasonGatewayNotProgrammed is used when the associated Gateway is not programmed.
	// Used with Accepted (false).
	RouteReasonGatewayNotProgrammed v1.RouteConditionReason = "GatewayNotProgrammed"

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

	// GatewayReasonGatewayConflict indicates there are multiple Gateway resources to choose from,
	// and we ignored the resource in question and picked another Gateway as the winner.
	// This reason is used with GatewayConditionAccepted (false).
	GatewayReasonGatewayConflict v1.GatewayConditionReason = "GatewayConflict"

	// GatewayMessageGatewayConflict is a message that describes GatewayReasonGatewayConflict.
	GatewayMessageGatewayConflict = "The resource is ignored due to a conflicting Gateway resource"

	// GatewayReasonUnsupportedValue is used with GatewayConditionAccepted (false) when a value of a field in a Gateway
	// is invalid or not supported.
	GatewayReasonUnsupportedValue v1.GatewayConditionReason = "UnsupportedValue"

	// GatewayMessageFailedNginxReload is a message used with GatewayConditionProgrammed (false)
	// when nginx fails to reload.
	GatewayMessageFailedNginxReload = "The Gateway is not programmed due to a failure to " +
		"reload nginx with the configuration. Please see the nginx container logs for any possible configuration issues"

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

	// PolicyReasonNginxProxyConfigNotSet is used with the "PolicyAccepted" condition when the
	// NginxProxy resource is missing or invalid.
	PolicyReasonNginxProxyConfigNotSet v1alpha2.PolicyConditionReason = "NginxProxyConfigNotSet"

	// PolicyMessageNginxProxyInvalid is a message used with the PolicyReasonNginxProxyConfigNotSet reason
	// when the NginxProxy resource is either invalid or not attached.
	PolicyMessageNginxProxyInvalid = "The NginxProxy configuration is either invalid or not attached to the GatewayClass"

	// PolicyMessageTelemetryNotEnabled is a message used with the PolicyReasonNginxProxyConfigNotSet reason
	// when telemetry is not enabled in the NginxProxy resource.
	PolicyMessageTelemetryNotEnabled = "Telemetry is not enabled in the NginxProxy resource"

	// PolicyReasonTargetConflict is used with the "PolicyAccepted" condition when a Route that it targets
	// has an overlapping hostname:port/path combination with another Route.
	PolicyReasonTargetConflict v1alpha2.PolicyConditionReason = "TargetConflict"

	// GatewayIgnoredReason is used with v1.RouteConditionAccepted when the route references a Gateway that is ignored
	// by NGF.
	GatewayIgnoredReason v1.RouteConditionReason = "GatewayIgnored"
)

// NewRouteNotAcceptedGatewayIgnored returns a Condition that indicates that the Route is not accepted by the Gateway
// because the Gateway is ignored by NGF.
func NewRouteNotAcceptedGatewayIgnored() conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.RouteConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(GatewayIgnoredReason),
		Message: "The Gateway is ignored by the controller",
	}
}

// NewDefaultRouteConditions returns the default conditions that must be present in the status of a Route.
func NewDefaultRouteConditions() []conditions.Condition {
	return []conditions.Condition{
		NewRouteAccepted(),
		NewRouteResolvedRefs(),
	}
}

// NewRouteNotAllowedByListeners returns a Condition that indicates that the Route is not allowed by
// any listener.
func NewRouteNotAllowedByListeners() conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.RouteConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.RouteReasonNotAllowedByListeners),
		Message: "Route is not allowed by any listener",
	}
}

// NewRouteNoMatchingListenerHostname returns a Condition that indicates that the hostname of the listener
// does not match the hostnames of the Route.
func NewRouteNoMatchingListenerHostname() conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.RouteConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.RouteReasonNoMatchingListenerHostname),
		Message: "Listener hostname does not match the Route hostnames",
	}
}

// NewRouteAccepted returns a Condition that indicates that the Route is accepted.
func NewRouteAccepted() conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.RouteConditionAccepted),
		Status:  metav1.ConditionTrue,
		Reason:  string(v1.RouteReasonAccepted),
		Message: "The route is accepted",
	}
}

// NewRouteUnsupportedValue returns a Condition that indicates that the Route includes an unsupported value.
func NewRouteUnsupportedValue(msg string) conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.RouteConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.RouteReasonUnsupportedValue),
		Message: msg,
	}
}

// NewRoutePartiallyInvalid returns a Condition that indicates that the Route contains a combination
// of both valid and invalid rules.
//
// // nolint:lll
// The message must start with "Dropped Rules(s)" according to the Gateway API spec
// See https://github.com/kubernetes-sigs/gateway-api/blob/37d81593e5a965ed76582dbc1a2f56bbd57c0622/apis/v1/shared_types.go#L408-L413
func NewRoutePartiallyInvalid(msg string) conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.RouteConditionPartiallyInvalid),
		Status:  metav1.ConditionTrue,
		Reason:  string(v1.RouteReasonUnsupportedValue),
		Message: "Dropped Rule(s): " + msg,
	}
}

// NewRouteInvalidListener returns a Condition that indicates that the Route is not accepted because of an
// invalid listener.
func NewRouteInvalidListener() conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.RouteConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(RouteReasonInvalidListener),
		Message: "Listener is invalid for this parent ref",
	}
}

// NewRouteHostnameConflict returns a Condition that indicates that the Route is not accepted because of a
// conflicting hostname on the same port.
func NewRouteHostnameConflict() conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.RouteConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(RouteReasonHostnameConflict),
		Message: "Hostname(s) conflict with another route of the same kind on the same port",
	}
}

// NewRouteResolvedRefs returns a Condition that indicates that all the references on the Route are resolved.
func NewRouteResolvedRefs() conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.RouteConditionResolvedRefs),
		Status:  metav1.ConditionTrue,
		Reason:  string(v1.RouteReasonResolvedRefs),
		Message: "All references are resolved",
	}
}

// NewRouteBackendRefInvalidKind returns a Condition that indicates that the Route has a backendRef with an
// invalid kind.
func NewRouteBackendRefInvalidKind(msg string) conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.RouteConditionResolvedRefs),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.RouteReasonInvalidKind),
		Message: msg,
	}
}

// NewRouteBackendRefRefNotPermitted returns a Condition that indicates that the Route has a backendRef that
// is not permitted.
func NewRouteBackendRefRefNotPermitted(msg string) conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.RouteConditionResolvedRefs),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.RouteReasonRefNotPermitted),
		Message: msg,
	}
}

// NewRouteBackendRefRefBackendNotFound returns a Condition that indicates that the Route has a backendRef that
// points to non-existing backend.
func NewRouteBackendRefRefBackendNotFound(msg string) conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.RouteConditionResolvedRefs),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.RouteReasonBackendNotFound),
		Message: msg,
	}
}

// NewRouteBackendRefUnsupportedValue returns a Condition that indicates that the Route has a backendRef with
// an unsupported value.
func NewRouteBackendRefUnsupportedValue(msg string) conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.RouteConditionResolvedRefs),
		Status:  metav1.ConditionFalse,
		Reason:  string(RouteReasonBackendRefUnsupportedValue),
		Message: msg,
	}
}

// NewRouteInvalidGateway returns a Condition that indicates that the Route is not Accepted because the Gateway it
// references is invalid.
func NewRouteInvalidGateway() conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.RouteConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(RouteReasonInvalidGateway),
		Message: "Gateway is invalid",
	}
}

// NewRouteNoMatchingParent returns a Condition that indicates that the Route is not Accepted because
// it specifies a Port and/or SectionName that does not match any Listeners in the Gateway.
func NewRouteNoMatchingParent() conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.RouteConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.RouteReasonNoMatchingParent),
		Message: "Listener is not found for this parent ref",
	}
}

// NewRouteUnsupportedConfiguration returns a Condition that indicates that the Route is not Accepted because
// it is incompatible with the Gateway's configuration.
func NewRouteUnsupportedConfiguration(msg string) conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.RouteConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(RouteReasonUnsupportedConfiguration),
		Message: msg,
	}
}

// NewRouteGatewayNotProgrammed returns a Condition that indicates that the Gateway it references is not programmed,
// which does not guarantee that the Route has been configured.
func NewRouteGatewayNotProgrammed(msg string) conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.RouteConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(RouteReasonGatewayNotProgrammed),
		Message: msg,
	}
}

// NewRouteInvalidIPFamily returns a Condition that indicates that the Service associated with the Route
// is not configured with the same IP family as the NGINX server.
func NewRouteInvalidIPFamily(msg string) conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.RouteConditionResolvedRefs),
		Status:  metav1.ConditionFalse,
		Reason:  string(RouteReasonInvalidIPFamily),
		Message: msg,
	}
}

// NewRouteResolvedRefsInvalidFilter returns a Condition that indicates that the Route has a filter that
// cannot be resolved or is invalid.
func NewRouteResolvedRefsInvalidFilter(msg string) conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.RouteConditionResolvedRefs),
		Status:  metav1.ConditionFalse,
		Reason:  string(RouteReasonInvalidFilter),
		Message: msg,
	}
}

// NewDefaultListenerConditions returns the default Conditions that must be present in the status of a Listener.
func NewDefaultListenerConditions() []conditions.Condition {
	return []conditions.Condition{
		NewListenerAccepted(),
		NewListenerProgrammed(),
		NewListenerResolvedRefs(),
		NewListenerNoConflicts(),
	}
}

// NewListenerAccepted returns a Condition that indicates that the Listener is accepted.
func NewListenerAccepted() conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.ListenerConditionAccepted),
		Status:  metav1.ConditionTrue,
		Reason:  string(v1.ListenerReasonAccepted),
		Message: "Listener is accepted",
	}
}

// NewListenerProgrammed returns a Condition that indicates the Listener is programmed.
func NewListenerProgrammed() conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.ListenerConditionProgrammed),
		Status:  metav1.ConditionTrue,
		Reason:  string(v1.ListenerReasonProgrammed),
		Message: "Listener is programmed",
	}
}

// NewListenerResolvedRefs returns a Condition that indicates that all references in a Listener are resolved.
func NewListenerResolvedRefs() conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.ListenerConditionResolvedRefs),
		Status:  metav1.ConditionTrue,
		Reason:  string(v1.ListenerReasonResolvedRefs),
		Message: "All references are resolved",
	}
}

// NewListenerNoConflicts returns a Condition that indicates that there are no conflicts in a Listener.
func NewListenerNoConflicts() conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.ListenerConditionConflicted),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.ListenerReasonNoConflicts),
		Message: "No conflicts",
	}
}

// NewListenerNotProgrammedInvalid returns a Condition that indicates the Listener is not programmed because it is
// semantically or syntactically invalid. The provided message contains the details of why the Listener is invalid.
func NewListenerNotProgrammedInvalid(msg string) conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.ListenerConditionProgrammed),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.ListenerReasonInvalid),
		Message: msg,
	}
}

// NewListenerUnsupportedValue returns Conditions that indicate that a field of a Listener has an unsupported value.
// Unsupported means that the value is not supported by the implementation or invalid.
func NewListenerUnsupportedValue(msg string) []conditions.Condition {
	return []conditions.Condition{
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
func NewListenerInvalidCertificateRef(msg string) []conditions.Condition {
	return []conditions.Condition{
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
func NewListenerInvalidRouteKinds(msg string) []conditions.Condition {
	return []conditions.Condition{
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
func NewListenerProtocolConflict(msg string) []conditions.Condition {
	return []conditions.Condition{
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
func NewListenerHostnameConflict(msg string) []conditions.Condition {
	return []conditions.Condition{
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
func NewListenerUnsupportedProtocol(msg string) []conditions.Condition {
	return []conditions.Condition{
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
func NewListenerRefNotPermitted(msg string) []conditions.Condition {
	return []conditions.Condition{
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

// NewGatewayClassResolvedRefs returns a Condition that indicates that the parametersRef
// on the GatewayClass is resolved.
func NewGatewayClassResolvedRefs() conditions.Condition {
	return conditions.Condition{
		Type:    string(GatewayClassResolvedRefs),
		Status:  metav1.ConditionTrue,
		Reason:  string(GatewayClassReasonResolvedRefs),
		Message: "parametersRef resource is resolved",
	}
}

// NewGatewayClassRefNotFound returns a Condition that indicates that the parametersRef
// on the GatewayClass could not be resolved.
func NewGatewayClassRefNotFound() conditions.Condition {
	return conditions.Condition{
		Type:    string(GatewayClassResolvedRefs),
		Status:  metav1.ConditionFalse,
		Reason:  string(GatewayClassReasonParamsRefNotFound),
		Message: "parametersRef resource could not be found",
	}
}

// NewGatewayClassInvalidParameters returns a Condition that indicates that the GatewayClass has invalid parameters.
// We are allowing Accepted to still be true to prevent nullifying the entire config tree if a parametersRef
// is updated to something invalid.
func NewGatewayClassInvalidParameters(msg string) conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.GatewayClassConditionStatusAccepted),
		Status:  metav1.ConditionTrue,
		Reason:  string(v1.GatewayClassReasonInvalidParameters),
		Message: fmt.Sprintf("GatewayClass is accepted, but parametersRef is ignored due to an error: %s", msg),
	}
}

// NewDefaultGatewayConditions returns the default Conditions that must be present in the status of a Gateway.
func NewDefaultGatewayConditions() []conditions.Condition {
	return []conditions.Condition{
		NewGatewayAccepted(),
		NewGatewayProgrammed(),
	}
}

// NewGatewayAccepted returns a Condition that indicates the Gateway is accepted.
func NewGatewayAccepted() conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.GatewayConditionAccepted),
		Status:  metav1.ConditionTrue,
		Reason:  string(v1.GatewayReasonAccepted),
		Message: "Gateway is accepted",
	}
}

// NewGatewayConflict returns Conditions that indicate the Gateway has a conflict with another Gateway.
func NewGatewayConflict() []conditions.Condition {
	return []conditions.Condition{
		{
			Type:    string(v1.GatewayConditionAccepted),
			Status:  metav1.ConditionFalse,
			Reason:  string(GatewayReasonGatewayConflict),
			Message: GatewayMessageGatewayConflict,
		},
		NewGatewayConflictNotProgrammed(),
	}
}

// NewGatewayAcceptedListenersNotValid returns a Condition that indicates the Gateway is accepted,
// but has at least one listener that is invalid.
func NewGatewayAcceptedListenersNotValid() conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.GatewayConditionAccepted),
		Status:  metav1.ConditionTrue,
		Reason:  string(v1.GatewayReasonListenersNotValid),
		Message: "Gateway has at least one valid listener",
	}
}

// NewGatewayNotAcceptedListenersNotValid returns Conditions that indicate the Gateway is not accepted,
// because all listeners are invalid.
func NewGatewayNotAcceptedListenersNotValid() []conditions.Condition {
	msg := "Gateway has no valid listeners"
	return []conditions.Condition{
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
func NewGatewayInvalid(msg string) []conditions.Condition {
	return []conditions.Condition{
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
// Unsupported means that the value is not supported by the implementation or invalid.
func NewGatewayUnsupportedValue(msg string) []conditions.Condition {
	return []conditions.Condition{
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

// NewGatewayProgrammed returns a Condition that indicates the Gateway is programmed.
func NewGatewayProgrammed() conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.GatewayConditionProgrammed),
		Status:  metav1.ConditionTrue,
		Reason:  string(v1.GatewayReasonProgrammed),
		Message: "Gateway is programmed",
	}
}

// NewGatewayNotProgrammedInvalid returns a Condition that indicates the Gateway is not programmed
// because it is semantically or syntactically invalid. The provided message contains the details of
// why the Gateway is invalid.
func NewGatewayNotProgrammedInvalid(msg string) conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.GatewayConditionProgrammed),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1.GatewayReasonInvalid),
		Message: msg,
	}
}

// NewGatewayConflictNotProgrammed returns a custom Programmed Condition that indicates the Gateway has a
// conflict with another Gateway.
func NewGatewayConflictNotProgrammed() conditions.Condition {
	return conditions.Condition{
		Type:    string(v1.GatewayConditionProgrammed),
		Status:  metav1.ConditionFalse,
		Reason:  string(GatewayReasonGatewayConflict),
		Message: GatewayMessageGatewayConflict,
	}
}

// NewNginxGatewayValid returns a Condition that indicates that the NginxGateway config is valid.
func NewNginxGatewayValid() conditions.Condition {
	return conditions.Condition{
		Type:    string(ngfAPI.NginxGatewayConditionValid),
		Status:  metav1.ConditionTrue,
		Reason:  string(ngfAPI.NginxGatewayReasonValid),
		Message: "NginxGateway is valid",
	}
}

// NewNginxGatewayInvalid returns a Condition that indicates that the NginxGateway config is invalid.
func NewNginxGatewayInvalid(msg string) conditions.Condition {
	return conditions.Condition{
		Type:    string(ngfAPI.NginxGatewayConditionValid),
		Status:  metav1.ConditionFalse,
		Reason:  string(ngfAPI.NginxGatewayReasonInvalid),
		Message: msg,
	}
}

// NewPolicyAccepted returns a Condition that indicates that the Policy is accepted.
func NewPolicyAccepted() conditions.Condition {
	return conditions.Condition{
		Type:    string(v1alpha2.PolicyConditionAccepted),
		Status:  metav1.ConditionTrue,
		Reason:  string(v1alpha2.PolicyReasonAccepted),
		Message: "Policy is accepted",
	}
}

// NewPolicyInvalid returns a Condition that indicates that the Policy is not accepted because it is semantically or
// syntactically invalid.
func NewPolicyInvalid(msg string) conditions.Condition {
	return conditions.Condition{
		Type:    string(v1alpha2.PolicyConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1alpha2.PolicyReasonInvalid),
		Message: msg,
	}
}

// NewPolicyConflicted returns a Condition that indicates that the Policy is not accepted because it conflicts with
// another Policy and a merge is not possible.
func NewPolicyConflicted(msg string) conditions.Condition {
	return conditions.Condition{
		Type:    string(v1alpha2.PolicyConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1alpha2.PolicyReasonConflicted),
		Message: msg,
	}
}

// NewPolicyTargetNotFound returns a Condition that indicates that the Policy is not accepted because the target
// resource does not exist or can not be attached to.
func NewPolicyTargetNotFound(msg string) conditions.Condition {
	return conditions.Condition{
		Type:    string(v1alpha2.PolicyConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(v1alpha2.PolicyReasonTargetNotFound),
		Message: msg,
	}
}

// NewPolicyNotAcceptedTargetConflict returns a Condition that indicates that the Policy is not accepted
// because the target resource has a conflict with another resource when attempting to apply this policy.
func NewPolicyNotAcceptedTargetConflict(msg string) conditions.Condition {
	return conditions.Condition{
		Type:    string(v1alpha2.PolicyConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(PolicyReasonTargetConflict),
		Message: msg,
	}
}

// NewPolicyNotAcceptedNginxProxyNotSet returns a Condition that indicates that the Policy is not accepted
// because it relies on the NginxProxy configuration which is missing or invalid.
func NewPolicyNotAcceptedNginxProxyNotSet(msg string) conditions.Condition {
	return conditions.Condition{
		Type:    string(v1alpha2.PolicyConditionAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(PolicyReasonNginxProxyConfigNotSet),
		Message: msg,
	}
}

// NewSnippetsFilterInvalid returns a Condition that indicates that the SnippetsFilter is not accepted because it is
// syntactically or semantically invalid.
func NewSnippetsFilterInvalid(msg string) conditions.Condition {
	return conditions.Condition{
		Type:    string(ngfAPI.SnippetsFilterConditionTypeAccepted),
		Status:  metav1.ConditionFalse,
		Reason:  string(ngfAPI.SnippetsFilterConditionReasonInvalid),
		Message: msg,
	}
}

// NewSnippetsFilterAccepted returns a Condition that indicates that the SnippetsFilter is accepted because it is
// valid.
func NewSnippetsFilterAccepted() conditions.Condition {
	return conditions.Condition{
		Type:    string(ngfAPI.SnippetsFilterConditionTypeAccepted),
		Status:  metav1.ConditionTrue,
		Reason:  string(ngfAPI.SnippetsFilterConditionReasonAccepted),
		Message: "SnippetsFilter is accepted",
	}
}

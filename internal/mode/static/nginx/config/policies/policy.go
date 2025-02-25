package policies

import (
	"slices"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
)

//go:generate go tool counterfeiter -generate

// Policy is an extension of client.Object. It adds methods that are common among all NGF Policies.
//
//counterfeiter:generate . Policy
type Policy interface {
	GetTargetRefs() []v1alpha2.LocalPolicyTargetReference
	GetPolicyStatus() v1alpha2.PolicyStatus
	SetPolicyStatus(status v1alpha2.PolicyStatus)
	client.Object
}

// GlobalSettings contains global settings from the current state of the graph that may be
// needed for policy validation or generation if certain policies rely on those global settings.
type GlobalSettings struct {
	// NginxProxyValid is whether the NginxProxy resource is valid.
	NginxProxyValid bool
	// TelemetryEnabled is whether telemetry is enabled in the NginxProxy resource.
	TelemetryEnabled bool
}

// ValidateTargetRef validates a policy's targetRef for the proper group and kind.
func ValidateTargetRef(
	ref v1alpha2.LocalPolicyTargetReference,
	basePath *field.Path,
	groups []gatewayv1.Group,
	supportedKinds []gatewayv1.Kind,
) error {
	if !slices.Contains(groups, ref.Group) {
		path := basePath.Child("group")

		return field.NotSupported(
			path,
			ref.Group,
			groups,
		)
	}

	if !slices.Contains(supportedKinds, ref.Kind) {
		path := basePath.Child("kind")

		return field.NotSupported(
			path,
			ref.Kind,
			supportedKinds,
		)
	}

	return nil
}

// We generate a mock of ObjectKind so that we can create fake policies and set their GVKs.
//counterfeiter:generate k8s.io/apimachinery/pkg/runtime/schema.ObjectKind

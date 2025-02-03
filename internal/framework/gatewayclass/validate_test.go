package gatewayclass_test

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/gatewayclass"
)

func TestValidateCRDVersions(t *testing.T) {
	t.Parallel()
	createCRDMetadata := func(version string) *metav1.PartialObjectMetadata {
		return &metav1.PartialObjectMetadata{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					gatewayclass.BundleVersionAnnotation: version,
				},
			},
		}
	}

	// Adding patch version to SupportedVersion to try and avoid having to update these tests with every release.
	fields := strings.Split(gatewayclass.SupportedVersion, ".")
	fields[2] = "99"

	validVersionWithPatch := createCRDMetadata(strings.Join(fields, "."))
	bestEffortVersion := createCRDMetadata("v1.99.99")
	unsupportedVersion := createCRDMetadata("v99.0.0")

	tests := []struct {
		crds     map[types.NamespacedName]*metav1.PartialObjectMetadata
		name     string
		expConds []conditions.Condition
		valid    bool
	}{
		{
			name: "valid; all supported versions",
			crds: map[types.NamespacedName]*metav1.PartialObjectMetadata{
				{Name: "gatewayclasses.gateway.networking.k8s.io"}:  validVersionWithPatch,
				{Name: "gateways.gateway.networking.k8s.io"}:        validVersionWithPatch,
				{Name: "httproutes.gateway.networking.k8s.io"}:      validVersionWithPatch,
				{Name: "referencegrants.gateway.networking.k8s.io"}: validVersionWithPatch,
				{Name: "some.other.crd"}:                            unsupportedVersion, /* should ignore */
			},
			valid:    true,
			expConds: nil,
		},
		{
			name: "valid; only one Gateway API CRD exists but it's a supported version",
			crds: map[types.NamespacedName]*metav1.PartialObjectMetadata{
				{Name: "gatewayclasses.gateway.networking.k8s.io"}: validVersionWithPatch,
				{Name: "some.other.crd"}:                           unsupportedVersion, /* should ignore */
			},
			valid:    true,
			expConds: nil,
		},
		{
			name: "valid; all best effort (supported major version)",
			crds: map[types.NamespacedName]*metav1.PartialObjectMetadata{
				{Name: "gatewayclasses.gateway.networking.k8s.io"}:  bestEffortVersion,
				{Name: "gateways.gateway.networking.k8s.io"}:        bestEffortVersion,
				{Name: "httproutes.gateway.networking.k8s.io"}:      bestEffortVersion,
				{Name: "referencegrants.gateway.networking.k8s.io"}: bestEffortVersion,
			},
			valid:    true,
			expConds: conditions.NewGatewayClassSupportedVersionBestEffort(gatewayclass.SupportedVersion),
		},
		{
			name: "valid; mix of supported and best effort versions",
			crds: map[types.NamespacedName]*metav1.PartialObjectMetadata{
				{Name: "gatewayclasses.gateway.networking.k8s.io"}:  validVersionWithPatch,
				{Name: "gateways.gateway.networking.k8s.io"}:        bestEffortVersion,
				{Name: "httproutes.gateway.networking.k8s.io"}:      validVersionWithPatch,
				{Name: "referencegrants.gateway.networking.k8s.io"}: validVersionWithPatch,
			},
			valid:    true,
			expConds: conditions.NewGatewayClassSupportedVersionBestEffort(gatewayclass.SupportedVersion),
		},
		{
			name: "invalid; all unsupported versions",
			crds: map[types.NamespacedName]*metav1.PartialObjectMetadata{
				{Name: "gatewayclasses.gateway.networking.k8s.io"}:  unsupportedVersion,
				{Name: "gateways.gateway.networking.k8s.io"}:        unsupportedVersion,
				{Name: "httproutes.gateway.networking.k8s.io"}:      unsupportedVersion,
				{Name: "referencegrants.gateway.networking.k8s.io"}: unsupportedVersion,
			},
			valid:    false,
			expConds: conditions.NewGatewayClassUnsupportedVersion(gatewayclass.SupportedVersion),
		},
		{
			name: "invalid; mix unsupported and best effort versions",
			crds: map[types.NamespacedName]*metav1.PartialObjectMetadata{
				{Name: "gatewayclasses.gateway.networking.k8s.io"}:  unsupportedVersion,
				{Name: "gateways.gateway.networking.k8s.io"}:        bestEffortVersion,
				{Name: "httproutes.gateway.networking.k8s.io"}:      unsupportedVersion,
				{Name: "referencegrants.gateway.networking.k8s.io"}: bestEffortVersion,
			},
			valid:    false,
			expConds: conditions.NewGatewayClassUnsupportedVersion(gatewayclass.SupportedVersion),
		},
		{
			name: "invalid; bad version string",
			crds: map[types.NamespacedName]*metav1.PartialObjectMetadata{
				{Name: "gatewayclasses.gateway.networking.k8s.io"}: createCRDMetadata("v"),
			},
			valid:    false,
			expConds: conditions.NewGatewayClassUnsupportedVersion(gatewayclass.SupportedVersion),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			conds, valid := gatewayclass.ValidateCRDVersions(test.crds)
			g.Expect(valid).To(Equal(test.valid))
			g.Expect(conds).To(Equal(test.expConds))
		})
	}
}

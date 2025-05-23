package graph

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
)

func TestProcessGatewayClasses(t *testing.T) {
	t.Parallel()
	gcName := "test-gc"
	ctlrName := "test-ctlr"
	winner := &v1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: gcName,
		},
		Spec: v1.GatewayClassSpec{
			ControllerName: v1.GatewayController(ctlrName),
		},
	}
	ignored := &v1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-gc-ignored",
		},
		Spec: v1.GatewayClassSpec{
			ControllerName: v1.GatewayController(ctlrName),
		},
	}

	tests := []struct {
		expected processedGatewayClasses
		gcs      map[types.NamespacedName]*v1.GatewayClass
		name     string
		exists   bool
	}{
		{
			gcs:      nil,
			expected: processedGatewayClasses{},
			name:     "no gatewayclasses",
		},
		{
			gcs: map[types.NamespacedName]*v1.GatewayClass{
				{Name: gcName}: winner,
			},
			expected: processedGatewayClasses{
				Winner: winner,
			},
			exists: true,
			name:   "one valid gatewayclass",
		},
		{
			gcs: map[types.NamespacedName]*v1.GatewayClass{
				{Name: gcName}: {
					ObjectMeta: metav1.ObjectMeta{
						Name: gcName,
					},
					Spec: v1.GatewayClassSpec{
						ControllerName: v1.GatewayController("not ours"),
					},
				},
			},
			expected: processedGatewayClasses{},
			exists:   true,
			name:     "one valid gatewayclass, but references wrong controller",
		},
		{
			gcs: map[types.NamespacedName]*v1.GatewayClass{
				{Name: ignored.Name}: ignored,
			},
			expected: processedGatewayClasses{
				Ignored: map[types.NamespacedName]*v1.GatewayClass{
					client.ObjectKeyFromObject(ignored): ignored,
				},
			},
			name: "one non-referenced gatewayclass with our controller",
		},
		{
			gcs: map[types.NamespacedName]*v1.GatewayClass{
				{Name: "completely ignored"}: {
					Spec: v1.GatewayClassSpec{
						ControllerName: v1.GatewayController("not ours"),
					},
				},
			},
			expected: processedGatewayClasses{},
			name:     "one non-referenced gatewayclass without our controller",
		},
		{
			gcs: map[types.NamespacedName]*v1.GatewayClass{
				{Name: gcName}:       winner,
				{Name: ignored.Name}: ignored,
			},
			expected: processedGatewayClasses{
				Winner: winner,
				Ignored: map[types.NamespacedName]*v1.GatewayClass{
					client.ObjectKeyFromObject(ignored): ignored,
				},
			},
			exists: true,
			name:   "one valid gateway class and non-referenced gatewayclass",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			result, exists := processGatewayClasses(test.gcs, gcName, ctlrName)
			g.Expect(helpers.Diff(test.expected, result)).To(BeEmpty())
			g.Expect(exists).To(Equal(test.exists))
		})
	}
}

func TestBuildGatewayClass(t *testing.T) {
	t.Parallel()
	validGC := &v1.GatewayClass{}
	npNsName := types.NamespacedName{Namespace: "test", Name: "nginx-proxy"}

	np := &ngfAPIv1alpha2.NginxProxy{
		TypeMeta: metav1.TypeMeta{
			Kind: kinds.NginxProxy,
		},
		Spec: ngfAPIv1alpha2.NginxProxySpec{
			Telemetry: &ngfAPIv1alpha2.Telemetry{
				ServiceName: helpers.GetPointer("my-svc"),
			},
		},
	}

	gcWithParams := &v1.GatewayClass{
		Spec: v1.GatewayClassSpec{
			ParametersRef: &v1.ParametersReference{
				Kind:      v1.Kind(kinds.NginxProxy),
				Namespace: helpers.GetPointer(v1.Namespace(npNsName.Namespace)),
				Name:      npNsName.Name,
			},
		},
	}

	gcWithParamsNoNamespace := gcWithParams.DeepCopy()
	gcWithParamsNoNamespace.Spec.ParametersRef.Namespace = nil

	gcWithInvalidKind := &v1.GatewayClass{
		Spec: v1.GatewayClassSpec{
			ParametersRef: &v1.ParametersReference{
				Kind:      v1.Kind("Invalid"),
				Namespace: helpers.GetPointer(v1.Namespace("test")),
			},
		},
	}

	validCRDs := map[types.NamespacedName]*metav1.PartialObjectMetadata{
		{Name: "gateways.gateway.networking.k8s.io"}: {
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					BundleVersionAnnotation: SupportedVersion,
				},
			},
		},
	}

	invalidCRDs := map[types.NamespacedName]*metav1.PartialObjectMetadata{
		{Name: "gateways.gateway.networking.k8s.io"}: {
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					BundleVersionAnnotation: "v99.0.0",
				},
			},
		},
	}

	tests := []struct {
		gc          *v1.GatewayClass
		nps         map[types.NamespacedName]*NginxProxy
		crdMetadata map[types.NamespacedName]*metav1.PartialObjectMetadata
		expected    *GatewayClass
		name        string
	}{
		{
			gc:          validGC,
			crdMetadata: validCRDs,
			expected: &GatewayClass{
				Source: validGC,
				Valid:  true,
			},
			name: "valid gatewayclass",
		},
		{
			gc:       nil,
			expected: nil,
			name:     "no gatewayclass",
		},
		{
			gc: gcWithParams,
			nps: map[types.NamespacedName]*NginxProxy{
				npNsName: {
					Source: np,
					Valid:  true,
				},
			},
			expected: &GatewayClass{
				Source:     gcWithParams,
				Valid:      true,
				Conditions: []conditions.Condition{conditions.NewGatewayClassResolvedRefs()},
				NginxProxy: &NginxProxy{
					Valid:  true,
					Source: np,
				},
			},
			name: "valid gatewayclass with paramsRef",
		},
		{
			gc: gcWithParamsNoNamespace,
			expected: &GatewayClass{
				Source: gcWithParamsNoNamespace,
				Valid:  true,
				Conditions: []conditions.Condition{
					conditions.NewGatewayClassRefInvalid(
						"spec.parametersRef.namespace: Required value: ParametersRef must specify Namespace",
					),
					conditions.NewGatewayClassInvalidParameters(
						"spec.parametersRef.namespace: Required value: ParametersRef must specify Namespace",
					),
				},
			},
			name: "valid gatewayclass with paramsRef missing namespace",
		},
		{
			gc: gcWithInvalidKind,
			expected: &GatewayClass{
				Source: gcWithInvalidKind,
				Valid:  true,
				Conditions: []conditions.Condition{
					conditions.NewGatewayClassRefInvalid(
						"spec.parametersRef.kind: Unsupported value: \"Invalid\": supported values: \"NginxProxy\"",
					),
					conditions.NewGatewayClassInvalidParameters(
						"spec.parametersRef.kind: Unsupported value: \"Invalid\": supported values: \"NginxProxy\"",
					),
				},
			},
			name: "valid gatewayclass with unsupported paramsRef Kind",
		},
		{
			gc: gcWithParams,
			expected: &GatewayClass{
				Source: gcWithParams,
				Valid:  true,
				Conditions: []conditions.Condition{
					conditions.NewGatewayClassRefNotFound(),
					conditions.NewGatewayClassInvalidParameters(
						"spec.parametersRef.name: Not found: \"nginx-proxy\"",
					),
				},
			},
			name: "valid gatewayclass with paramsRef resource that doesn't exist",
		},
		{
			gc: gcWithParams,
			nps: map[types.NamespacedName]*NginxProxy{
				npNsName: {
					Valid: false,
					ErrMsgs: field.ErrorList{
						field.Invalid(
							field.NewPath("spec", "telemetry", "serviceName"),
							"my-svc",
							"error",
						),
						field.Invalid(
							field.NewPath("spec", "telemetry", "exporter", "endpoint"),
							"my-endpoint",
							"error",
						),
					},
				},
			},
			expected: &GatewayClass{
				Source: gcWithParams,
				Valid:  true,
				Conditions: []conditions.Condition{
					conditions.NewGatewayClassRefInvalid(
						"[spec.telemetry.serviceName: Invalid value: \"my-svc\": error" +
							", spec.telemetry.exporter.endpoint: Invalid value: \"my-endpoint\": error]",
					),
					conditions.NewGatewayClassInvalidParameters(
						"[spec.telemetry.serviceName: Invalid value: \"my-svc\": error" +
							", spec.telemetry.exporter.endpoint: Invalid value: \"my-endpoint\": error]",
					),
				},
				NginxProxy: &NginxProxy{
					Valid: false,
					ErrMsgs: field.ErrorList{
						field.Invalid(
							field.NewPath("spec", "telemetry", "serviceName"),
							"my-svc",
							"error",
						),
						field.Invalid(
							field.NewPath("spec", "telemetry", "exporter", "endpoint"),
							"my-endpoint",
							"error",
						),
					},
				},
			},
			name: "valid gatewayclass with invalid paramsRef resource",
		},
		{
			gc:          validGC,
			crdMetadata: invalidCRDs,
			expected: &GatewayClass{
				Source:     validGC,
				Valid:      false,
				Conditions: conditions.NewGatewayClassUnsupportedVersion(SupportedVersion),
			},
			name: "invalid gatewayclass; unsupported version",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := buildGatewayClass(test.gc, test.nps, test.crdMetadata)
			g.Expect(helpers.Diff(test.expected, result)).To(BeEmpty())
		})
	}
}

func TestValidateCRDVersions(t *testing.T) {
	t.Parallel()
	createCRDMetadata := func(version string) *metav1.PartialObjectMetadata {
		return &metav1.PartialObjectMetadata{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					BundleVersionAnnotation: version,
				},
			},
		}
	}

	// Adding patch version to SupportedVersion to try and avoid having to update these tests with every release.
	fields := strings.Split(SupportedVersion, ".")
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
			expConds: conditions.NewGatewayClassSupportedVersionBestEffort(SupportedVersion),
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
			expConds: conditions.NewGatewayClassSupportedVersionBestEffort(SupportedVersion),
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
			expConds: conditions.NewGatewayClassUnsupportedVersion(SupportedVersion),
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
			expConds: conditions.NewGatewayClassUnsupportedVersion(SupportedVersion),
		},
		{
			name: "invalid; bad version string",
			crds: map[types.NamespacedName]*metav1.PartialObjectMetadata{
				{Name: "gatewayclasses.gateway.networking.k8s.io"}: createCRDMetadata("v"),
			},
			valid:    false,
			expConds: conditions.NewGatewayClassUnsupportedVersion(SupportedVersion),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			conds, valid := validateCRDVersions(test.crds)
			g.Expect(valid).To(Equal(test.valid))
			g.Expect(conds).To(Equal(test.expConds))
		})
	}
}

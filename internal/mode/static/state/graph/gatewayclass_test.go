package graph

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginxinc/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginxinc/nginx-gateway-fabric/internal/framework/conditions"
	"github.com/nginxinc/nginx-gateway-fabric/internal/framework/gatewayclass"
	"github.com/nginxinc/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginxinc/nginx-gateway-fabric/internal/framework/kinds"
	staticConds "github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/state/conditions"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/state/validation"
	"github.com/nginxinc/nginx-gateway-fabric/internal/mode/static/state/validation/validationfakes"
)

func TestProcessGatewayClasses(t *testing.T) {
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
			g := NewWithT(t)
			result, exists := processGatewayClasses(test.gcs, gcName, ctlrName)
			g.Expect(helpers.Diff(test.expected, result)).To(BeEmpty())
			g.Expect(exists).To(Equal(test.exists))
		})
	}
}

func TestBuildGatewayClass(t *testing.T) {
	validGC := &v1.GatewayClass{}

	gcWithParams := &v1.GatewayClass{
		Spec: v1.GatewayClassSpec{
			ParametersRef: &v1.ParametersReference{
				Kind:      v1.Kind(kinds.NginxProxy),
				Namespace: helpers.GetPointer(v1.Namespace("test")),
				Name:      "nginx-proxy",
			},
		},
	}

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
					gatewayclass.BundleVersionAnnotation: gatewayclass.SupportedVersion,
				},
			},
		},
	}

	invalidCRDs := map[types.NamespacedName]*metav1.PartialObjectMetadata{
		{Name: "gateways.gateway.networking.k8s.io"}: {
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					gatewayclass.BundleVersionAnnotation: "v99.0.0",
				},
			},
		},
	}

	createValidNPValidator := func() *validationfakes.FakeGenericValidator {
		v := &validationfakes.FakeGenericValidator{}
		v.ValidateServiceNameReturns(nil)
		v.ValidateEndpointReturns(nil)

		return v
	}

	createInvalidNPValidator := func() *validationfakes.FakeGenericValidator {
		v := &validationfakes.FakeGenericValidator{}
		v.ValidateServiceNameReturns(errors.New("error"))
		v.ValidateEndpointReturns(errors.New("error"))

		return v
	}

	tests := []struct {
		gc          *v1.GatewayClass
		np          *ngfAPI.NginxProxy
		validator   validation.GenericValidator
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
			np: &ngfAPI.NginxProxy{
				TypeMeta: metav1.TypeMeta{
					Kind: kinds.NginxProxy,
				},
				Spec: ngfAPI.NginxProxySpec{
					Telemetry: &ngfAPI.Telemetry{
						ServiceName: helpers.GetPointer("my-svc"),
					},
				},
			},
			validator: createValidNPValidator(),
			expected: &GatewayClass{
				Source:     gcWithParams,
				Valid:      true,
				Conditions: []conditions.Condition{staticConds.NewGatewayClassResolvedRefs()},
			},
			name: "valid gatewayclass with paramsRef",
		},
		{
			gc: gcWithInvalidKind,
			np: &ngfAPI.NginxProxy{
				TypeMeta: metav1.TypeMeta{
					Kind: kinds.NginxProxy,
				},
			},
			expected: &GatewayClass{
				Source: gcWithInvalidKind,
				Valid:  true,
				Conditions: []conditions.Condition{
					staticConds.NewGatewayClassInvalidParameters(
						"spec.parametersRef.kind: Unsupported value: \"Invalid\": supported values: \"NginxProxy\"",
					),
				},
			},
			name: "invalid gatewayclass with unsupported paramsRef Kind",
		},
		{
			gc: gcWithParams,
			expected: &GatewayClass{
				Source: gcWithParams,
				Valid:  true,
				Conditions: []conditions.Condition{
					staticConds.NewGatewayClassRefNotFound(),
					staticConds.NewGatewayClassInvalidParameters(
						"spec.parametersRef.name: Not found: \"nginx-proxy\"",
					),
				},
			},
			name: "invalid gatewayclass with paramsRef resource that doesn't exist",
		},
		{
			gc: gcWithParams,
			np: &ngfAPI.NginxProxy{
				TypeMeta: metav1.TypeMeta{
					Kind: kinds.NginxProxy,
				},
				Spec: ngfAPI.NginxProxySpec{
					Telemetry: &ngfAPI.Telemetry{
						ServiceName: helpers.GetPointer("my-svc"),
						Exporter: &ngfAPI.TelemetryExporter{
							Endpoint: "my-endpoint",
						},
					},
				},
			},
			validator: createInvalidNPValidator(),
			expected: &GatewayClass{
				Source: gcWithParams,
				Valid:  true,
				Conditions: []conditions.Condition{
					staticConds.NewGatewayClassInvalidParameters(
						"[spec.telemetry.serviceName: Invalid value: \"my-svc\": error" +
							", spec.telemetry.exporter.endpoint: Invalid value: \"my-endpoint\": error]",
					),
				},
			},
			name: "invalid gatewayclass with invalid paramsRef resource",
		},
		{
			gc:          validGC,
			crdMetadata: invalidCRDs,
			expected: &GatewayClass{
				Source:     validGC,
				Valid:      false,
				Conditions: conditions.NewGatewayClassUnsupportedVersion(gatewayclass.SupportedVersion),
			},
			name: "invalid gatewayclass; unsupported version",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)

			result := buildGatewayClass(test.gc, test.np, test.crdMetadata, test.validator)
			g.Expect(helpers.Diff(test.expected, result)).To(BeEmpty())
		})
	}
}

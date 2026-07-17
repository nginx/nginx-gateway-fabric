package cel

import (
	"testing"

	controllerruntime "sigs.k8s.io/controller-runtime"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

// validPayloadProcessorSpec returns a valid PayloadProcessorSpec that individual test cases
// mutate along a single dimension. TargetRef.Name is set per-case before creation.
func validPayloadProcessorSpec() ngfAPIv1alpha1.PayloadProcessorSpec {
	return ngfAPIv1alpha1.PayloadProcessorSpec{
		TargetRef: gatewayv1.LocalPolicyTargetReference{
			Kind:  gatewayKind,
			Group: gatewayGroup,
		},
		Processors: []ngfAPIv1alpha1.PayloadProcessorEntry{
			{
				ExtProc: &ngfAPIv1alpha1.ExtProcConfig{
					BackendRef: gatewayv1.BackendObjectReference{
						Name: "ext-svc",
						Port: helpers.GetPointer[gatewayv1.PortNumber](9000),
					},
				},
			},
		},
	}
}

func createPayloadProcessor(spec ngfAPIv1alpha1.PayloadProcessorSpec) *ngfAPIv1alpha1.PayloadProcessor {
	spec.TargetRef.Name = gatewayv1.ObjectName(uniqueResourceName(testTargetRefName))
	return &ngfAPIv1alpha1.PayloadProcessor{
		ObjectMeta: controllerruntime.ObjectMeta{
			Name:      uniqueResourceName(testResourceName),
			Namespace: defaultNamespace,
		},
		Spec: spec,
	}
}

func TestPayloadProcessorTargetRefKind(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		name       string
		targetKind gatewayv1.Kind
		wantErrors []string
	}{
		{
			name:       "Validate TargetRef of kind Gateway is allowed",
			targetKind: gatewayKind,
		},
		{
			name:       "Validate TargetRef of kind HTTPRoute is allowed",
			targetKind: httpRouteKind,
		},
		{
			name:       "Validate TargetRef of kind GRPCRoute is not allowed",
			targetKind: grpcRouteKind,
			wantErrors: []string{expectedTargetRefKindGatewayOrHTTPRouteError},
		},
		{
			name:       "Validate TargetRef of kind TCPRoute is not allowed",
			targetKind: tcpRouteKind,
			wantErrors: []string{expectedTargetRefKindGatewayOrHTTPRouteError},
		},
		{
			name:       "Validate invalid TargetRef Kind is not allowed",
			targetKind: invalidKind,
			wantErrors: []string{expectedTargetRefKindGatewayOrHTTPRouteError},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec := validPayloadProcessorSpec()
			spec.TargetRef.Kind = tt.targetKind
			validateCrd(t, tt.wantErrors, createPayloadProcessor(spec), k8sClient)
		})
	}
}

func TestPayloadProcessorTargetRefGroup(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		name        string
		targetGroup gatewayv1.Group
		wantErrors  []string
	}{
		{
			name:        "Validate gateway.networking.k8s.io TargetRef Group is allowed",
			targetGroup: gatewayGroup,
		},
		{
			name:        "Validate invalid.networking.k8s.io TargetRef Group is not allowed",
			targetGroup: invalidGroup,
			wantErrors:  []string{expectedTargetRefGroupError},
		},
		{
			name:        "Validate discovery.k8s.io/v1 TargetRef Group is not allowed",
			targetGroup: discoveryGroup,
			wantErrors:  []string{expectedTargetRefGroupError},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec := validPayloadProcessorSpec()
			spec.TargetRef.Group = tt.targetGroup
			validateCrd(t, tt.wantErrors, createPayloadProcessor(spec), k8sClient)
		})
	}
}

func TestPayloadProcessorProcessorExtProc(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		processor  ngfAPIv1alpha1.PayloadProcessorEntry
		name       string
		wantErrors []string
	}{
		{
			name: "Validate processor with extProc set is allowed",
			processor: ngfAPIv1alpha1.PayloadProcessorEntry{
				ExtProc: &ngfAPIv1alpha1.ExtProcConfig{
					BackendRef: gatewayv1.BackendObjectReference{
						Name: "ext-svc",
						Port: helpers.GetPointer[gatewayv1.PortNumber](9000),
					},
				},
			},
		},
		{
			name:       "Validate processor with extProc unset is not allowed",
			processor:  ngfAPIv1alpha1.PayloadProcessorEntry{},
			wantErrors: []string{expectedProcessorExtProcRequiredError},
		},
		{
			name: "Validate processor with only timeout set is not allowed",
			processor: ngfAPIv1alpha1.PayloadProcessorEntry{
				Timeout: helpers.GetPointer[ngfAPIv1alpha1.Duration]("5s"),
			},
			wantErrors: []string{expectedProcessorExtProcRequiredError},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec := validPayloadProcessorSpec()
			spec.Processors = []ngfAPIv1alpha1.PayloadProcessorEntry{tt.processor}
			validateCrd(t, tt.wantErrors, createPayloadProcessor(spec), k8sClient)
		})
	}
}

func TestPayloadProcessorProcessorsMaxItems(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	extProc := ngfAPIv1alpha1.PayloadProcessorEntry{
		ExtProc: &ngfAPIv1alpha1.ExtProcConfig{
			BackendRef: gatewayv1.BackendObjectReference{
				Name: "ext-svc",
				Port: helpers.GetPointer[gatewayv1.PortNumber](9000),
			},
		},
	}

	tests := []struct {
		name       string
		processors []ngfAPIv1alpha1.PayloadProcessorEntry
		wantErrors []string
	}{
		{
			name:       "Validate empty processors list is not allowed",
			processors: []ngfAPIv1alpha1.PayloadProcessorEntry{},
			wantErrors: []string{"should have at least 1 items"},
		},
		{
			name:       "Validate single processor is allowed",
			processors: []ngfAPIv1alpha1.PayloadProcessorEntry{extProc},
		},
		{
			name:       "Validate two processors is not allowed",
			processors: []ngfAPIv1alpha1.PayloadProcessorEntry{extProc, extProc},
			wantErrors: []string{"must have at most 1 item"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec := validPayloadProcessorSpec()
			spec.Processors = tt.processors
			validateCrd(t, tt.wantErrors, createPayloadProcessor(spec), k8sClient)
		})
	}
}

func TestPayloadProcessorBackendRefName(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		name           string
		backendRefName gatewayv1.ObjectName
		wantErrors     []string
	}{
		{
			name:           "Validate non-empty backendRef name is allowed",
			backendRefName: "ext-svc",
		},
		{
			name:           "Validate empty backendRef name is not allowed",
			backendRefName: "",
			wantErrors:     []string{expectedBackendRefNameEmptyError},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec := validPayloadProcessorSpec()
			spec.Processors[0].ExtProc.BackendRef.Name = tt.backendRefName
			validateCrd(t, tt.wantErrors, createPayloadProcessor(spec), k8sClient)
		})
	}
}

func TestPayloadProcessorBackendRefKind(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		backendRefKind *gatewayv1.Kind
		name           string
		wantErrors     []string
	}{
		{
			name:           "Validate unset kind is allowed",
			backendRefKind: nil,
		},
		{
			name:           "Validate Service kind is allowed",
			backendRefKind: helpers.GetPointer[gatewayv1.Kind](serviceKind),
		},
		{
			name:           "Validate Secret kind is not allowed",
			backendRefKind: helpers.GetPointer[gatewayv1.Kind]("Secret"),
			wantErrors:     []string{"backendRef.kind must be Service"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec := validPayloadProcessorSpec()
			spec.Processors[0].ExtProc.BackendRef.Kind = tt.backendRefKind
			validateCrd(t, tt.wantErrors, createPayloadProcessor(spec), k8sClient)
		})
	}
}

func TestPayloadProcessorBackendRefGroup(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		backendRefGroup *gatewayv1.Group
		name            string
		wantErrors      []string
	}{
		{
			name:            "Validate unset group is allowed",
			backendRefGroup: nil,
		},
		{
			name:            "Validate empty group is allowed",
			backendRefGroup: helpers.GetPointer[gatewayv1.Group](""),
		},
		{
			name:            "Validate core group is allowed",
			backendRefGroup: helpers.GetPointer[gatewayv1.Group]("core"),
		},
		{
			name:            "Validate non-core group is not allowed",
			backendRefGroup: helpers.GetPointer[gatewayv1.Group](invalidGroup),
			wantErrors:      []string{"backendRef.group must be core"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec := validPayloadProcessorSpec()
			spec.Processors[0].ExtProc.BackendRef.Group = tt.backendRefGroup
			validateCrd(t, tt.wantErrors, createPayloadProcessor(spec), k8sClient)
		})
	}
}

func TestPayloadProcessorPort(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		port       *gatewayv1.PortNumber
		name       string
		wantErrors []string
	}{
		{
			name: "Validate port 1 is allowed",
			port: helpers.GetPointer[gatewayv1.PortNumber](1),
		},
		{
			name: "Validate port 80 is allowed",
			port: helpers.GetPointer[gatewayv1.PortNumber](80),
		},
		{
			name: "Validate port 65535 is allowed",
			port: helpers.GetPointer[gatewayv1.PortNumber](65535),
		},
		{
			name:       "Validate unset port is not allowed",
			port:       nil,
			wantErrors: []string{"backendRef.port must be set"},
		},
		{
			name:       "Validate port 0 is not allowed",
			port:       helpers.GetPointer[gatewayv1.PortNumber](0),
			wantErrors: []string{"port in body should be greater than or equal to 1"},
		},
		{
			name:       "Validate port 65536 is not allowed",
			port:       helpers.GetPointer[gatewayv1.PortNumber](65536),
			wantErrors: []string{"port in body should be less than or equal to 65535"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec := validPayloadProcessorSpec()
			spec.Processors[0].ExtProc.BackendRef.Port = tt.port
			validateCrd(t, tt.wantErrors, createPayloadProcessor(spec), k8sClient)
		})
	}
}

func TestPayloadProcessorTimeout(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		timeout    *ngfAPIv1alpha1.Duration
		name       string
		wantErrors []string
	}{
		{
			name:    "Validate unset timeout is allowed",
			timeout: nil,
		},
		{
			name:    "Validate timeout 5s is allowed",
			timeout: helpers.GetPointer[ngfAPIv1alpha1.Duration]("5s"),
		},
		{
			name:    "Validate timeout 120s is allowed",
			timeout: helpers.GetPointer[ngfAPIv1alpha1.Duration]("120s"),
		},
		{
			name:    "Validate timeout 50ms is allowed",
			timeout: helpers.GetPointer[ngfAPIv1alpha1.Duration]("50ms"),
		},
		{
			name:    "Validate timeout 1h is allowed",
			timeout: helpers.GetPointer[ngfAPIv1alpha1.Duration]("1h"),
		},
		{
			name:       "Validate malformed timeout is not allowed",
			timeout:    helpers.GetPointer[ngfAPIv1alpha1.Duration]("abc"),
			wantErrors: []string{"timeout in body should match"},
		},
		{
			name:       "Validate timeout with invalid suffix is not allowed",
			timeout:    helpers.GetPointer[ngfAPIv1alpha1.Duration]("5x"),
			wantErrors: []string{"timeout in body should match"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec := validPayloadProcessorSpec()
			spec.Processors[0].Timeout = tt.timeout
			validateCrd(t, tt.wantErrors, createPayloadProcessor(spec), k8sClient)
		})
	}
}

package graph

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

func TestValidateHTTPListener(t *testing.T) {
	t.Parallel()
	protectedPorts := ProtectedPorts{9113: "MetricsPort"}

	tests := []struct {
		l        v1.Listener
		name     string
		expected []conditions.Condition
	}{
		{
			l: v1.Listener{
				Port: 80,
			},
			expected: nil,
			name:     "valid",
		},
		{
			l: v1.Listener{
				Port: 0,
			},
			expected: conditions.NewListenerUnsupportedValue(`port: Invalid value: 0: port must be between 1-65535`),
			name:     "invalid port",
		},
		{
			l: v1.Listener{
				Port: 80,
				TLS: &v1.ListenerTLSConfig{
					Mode: helpers.GetPointer(v1.TLSModeTerminate),
				},
				Name: "http-listener",
			},
			expected: conditions.NewListenerUnsupportedValue(`tls: Forbidden: tls is not supported for HTTP listener`),
			name:     "invalid HTTP listener with TLS",
		},
		{
			l: v1.Listener{
				Port: 9113,
			},
			expected: conditions.NewListenerUnsupportedValue(
				`port: Invalid value: 9113: port is already in use as MetricsPort`,
			),
			name: "invalid protected port",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			v := createHTTPListenerValidator(protectedPorts)

			result, attachable := v(test.l)

			g.Expect(result).To(Equal(test.expected))
			g.Expect(attachable).To(BeTrue())
		})
	}
}

func TestValidateHTTPSListener(t *testing.T) {
	t.Parallel()
	secretNs := "secret-ns"

	validSecretRef := v1.SecretObjectReference{
		Kind:      (*v1.Kind)(helpers.GetPointer("Secret")),
		Name:      "secret",
		Namespace: (*v1.Namespace)(helpers.GetPointer(secretNs)),
	}

	invalidSecretRefGroup := v1.SecretObjectReference{
		Group:     (*v1.Group)(helpers.GetPointer("some-group")),
		Kind:      (*v1.Kind)(helpers.GetPointer("Secret")),
		Name:      "secret",
		Namespace: (*v1.Namespace)(helpers.GetPointer(secretNs)),
	}

	invalidSecretRefKind := v1.SecretObjectReference{
		Kind:      (*v1.Kind)(helpers.GetPointer("ConfigMap")),
		Name:      "secret",
		Namespace: (*v1.Namespace)(helpers.GetPointer(secretNs)),
	}

	protectedPorts := ProtectedPorts{9113: "MetricsPort"}

	tests := []struct {
		l        v1.Listener
		name     string
		expected []conditions.Condition
	}{
		{
			l: v1.Listener{
				Port: 443,
				TLS: &v1.ListenerTLSConfig{
					Mode:            helpers.GetPointer(v1.TLSModeTerminate),
					CertificateRefs: []v1.SecretObjectReference{validSecretRef},
				},
			},
			expected: nil,
			name:     "valid",
		},
		{
			l: v1.Listener{
				Port: 0,
				TLS: &v1.ListenerTLSConfig{
					Mode:            helpers.GetPointer(v1.TLSModeTerminate),
					CertificateRefs: []v1.SecretObjectReference{validSecretRef},
				},
			},
			expected: conditions.NewListenerUnsupportedValue(`port: Invalid value: 0: port must be between 1-65535`),
			name:     "invalid port",
		},
		{
			l: v1.Listener{
				Port: 9113,
				TLS: &v1.ListenerTLSConfig{
					Mode:            helpers.GetPointer(v1.TLSModeTerminate),
					CertificateRefs: []v1.SecretObjectReference{validSecretRef},
				},
			},
			expected: conditions.NewListenerUnsupportedValue(
				`port: Invalid value: 9113: port is already in use as MetricsPort`,
			),
			name: "invalid protected port",
		},
		{
			l: v1.Listener{
				Port: 443,
				TLS: &v1.ListenerTLSConfig{
					Mode:            helpers.GetPointer(v1.TLSModeTerminate),
					CertificateRefs: []v1.SecretObjectReference{validSecretRef},
					Options:         map[v1.AnnotationKey]v1.AnnotationValue{"key": "val"},
				},
			},
			expected: conditions.NewListenerUnsupportedValue("tls.options: Forbidden: options are not supported"),
			name:     "invalid options",
		},
		{
			l: v1.Listener{
				Port: 443,
				TLS: &v1.ListenerTLSConfig{
					Mode:            helpers.GetPointer(v1.TLSModePassthrough),
					CertificateRefs: []v1.SecretObjectReference{validSecretRef},
				},
			},
			expected: conditions.NewListenerUnsupportedValue(
				`tls.mode: Unsupported value: "Passthrough": supported values: "Terminate"`,
			),
			name: "invalid tls mode",
		},
		{
			l: v1.Listener{
				Port: 443,
				TLS:  nil,
			},
			expected: conditions.NewListenerUnsupportedValue(
				`TLS: Required value: tls must be defined for HTTPS listener`,
			),
			name: "nil tls",
		},
		{
			l: v1.Listener{
				Port: 443,
				TLS: &v1.ListenerTLSConfig{
					Mode:            helpers.GetPointer(v1.TLSModeTerminate),
					CertificateRefs: []v1.SecretObjectReference{invalidSecretRefGroup},
				},
			},
			expected: conditions.NewListenerInvalidCertificateRef(
				`tls.certificateRefs[0].group: Unsupported value: "some-group": supported values: ""`,
			),
			name: "invalid cert ref group",
		},
		{
			l: v1.Listener{
				Port: 443,
				TLS: &v1.ListenerTLSConfig{
					Mode:            helpers.GetPointer(v1.TLSModeTerminate),
					CertificateRefs: []v1.SecretObjectReference{},
				},
			},
			expected: conditions.NewListenerInvalidCertificateRef(
				`tls.certificateRefs: Required value: certificateRefs must be defined for TLS mode terminate`,
			),
			name: "zero cert refs",
		},
		{
			l: v1.Listener{
				Port: 443,
				TLS: &v1.ListenerTLSConfig{
					Mode:            helpers.GetPointer(v1.TLSModeTerminate),
					CertificateRefs: []v1.SecretObjectReference{invalidSecretRefKind},
				},
			},
			expected: conditions.NewListenerInvalidCertificateRef(
				`tls.certificateRefs[0].kind: Unsupported value: "ConfigMap": supported values: "Secret"`,
			),
			name: "invalid cert ref kind",
		},
		{
			l: v1.Listener{
				Port: 443,
				TLS: &v1.ListenerTLSConfig{
					Mode:            helpers.GetPointer(v1.TLSModeTerminate),
					CertificateRefs: []v1.SecretObjectReference{validSecretRef, validSecretRef},
				},
			},
			expected: conditions.NewListenerUnsupportedValue(
				"tls.certificateRefs: Too many: 2: must have at most 1 item",
			),
			name: "too many cert refs",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			v := createHTTPSListenerValidator(protectedPorts)

			result, attachable := v(test.l)
			g.Expect(result).To(Equal(test.expected))
			g.Expect(attachable).To(BeTrue())
		})
	}
}

func TestValidateListenerHostname(t *testing.T) {
	t.Parallel()
	tests := []struct {
		hostname  *v1.Hostname
		name      string
		expectErr bool
	}{
		{
			hostname:  nil,
			expectErr: false,
			name:      "nil hostname",
		},
		{
			hostname:  (*v1.Hostname)(helpers.GetPointer("")),
			expectErr: false,
			name:      "empty hostname",
		},
		{
			hostname:  (*v1.Hostname)(helpers.GetPointer("foo.example.com")),
			expectErr: false,
			name:      "valid hostname",
		},
		{
			hostname:  (*v1.Hostname)(helpers.GetPointer("*.example.com")),
			expectErr: false,
			name:      "wildcard hostname",
		},
		{
			hostname:  (*v1.Hostname)(helpers.GetPointer("example$com")),
			expectErr: true,
			name:      "invalid hostname",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			conds, attachable := validateListenerHostname(v1.Listener{Hostname: test.hostname})

			if test.expectErr {
				g.Expect(conds).ToNot(BeEmpty())
				g.Expect(attachable).To(BeFalse())
			} else {
				g.Expect(conds).To(BeEmpty())
				g.Expect(attachable).To(BeTrue())
			}
		})
	}
}

func TestGetAndValidateListenerSupportedKinds(t *testing.T) {
	t.Parallel()
	HTTPRouteGroupKind := v1.RouteGroupKind{
		Kind:  kinds.HTTPRoute,
		Group: helpers.GetPointer[v1.Group](v1.GroupName),
	}
	GRPCRouteGroupKind := v1.RouteGroupKind{
		Kind:  kinds.GRPCRoute,
		Group: helpers.GetPointer[v1.Group](v1.GroupName),
	}
	TCPRouteGroupKind := []v1.RouteGroupKind{
		{
			Kind:  "TCPRoute",
			Group: helpers.GetPointer[v1.Group](v1.GroupName),
		},
	}
	TLSRouteGroupKind := v1.RouteGroupKind{
		Kind:  kinds.TLSRoute,
		Group: helpers.GetPointer[v1.Group](v1.GroupName),
	}
	tests := []struct {
		protocol  v1.ProtocolType
		name      string
		kind      []v1.RouteGroupKind
		expected  []v1.RouteGroupKind
		expectErr bool
	}{
		{
			protocol:  v1.TCPProtocolType,
			expectErr: false,
			name:      "valid TCP protocol",
			expected: []v1.RouteGroupKind{
				{
					Kind:  kinds.TCPRoute,
					Group: helpers.GetPointer[v1.Group](v1.GroupName),
				},
			},
		},
		{
			protocol:  v1.UDPProtocolType,
			expectErr: false,
			name:      "valid UDP protocol",
			expected: []v1.RouteGroupKind{
				{
					Kind:  kinds.UDPRoute,
					Group: helpers.GetPointer[v1.Group](v1.GroupName),
				},
			},
		},
		{
			protocol: v1.HTTPProtocolType,
			kind: []v1.RouteGroupKind{
				{
					Kind:  kinds.HTTPRoute,
					Group: helpers.GetPointer[v1.Group]("bad-group"),
				},
			},
			expectErr: true,
			name:      "invalid group",
			expected:  []v1.RouteGroupKind{},
		},
		{
			protocol:  v1.HTTPProtocolType,
			kind:      TCPRouteGroupKind,
			expectErr: true,
			name:      "invalid kind",
			expected:  []v1.RouteGroupKind{},
		},
		{
			protocol:  v1.HTTPProtocolType,
			kind:      []v1.RouteGroupKind{HTTPRouteGroupKind},
			expectErr: false,
			name:      "valid HTTP",
			expected:  []v1.RouteGroupKind{HTTPRouteGroupKind},
		},
		{
			protocol:  v1.HTTPSProtocolType,
			kind:      []v1.RouteGroupKind{HTTPRouteGroupKind},
			expectErr: false,
			name:      "valid HTTPS",
			expected:  []v1.RouteGroupKind{HTTPRouteGroupKind},
		},
		{
			protocol:  v1.HTTPSProtocolType,
			expectErr: false,
			name:      "valid HTTPS no kind specified",
			expected: []v1.RouteGroupKind{
				HTTPRouteGroupKind, GRPCRouteGroupKind,
			},
		},

		{
			protocol: v1.HTTPProtocolType,
			kind: []v1.RouteGroupKind{
				HTTPRouteGroupKind,
				{
					Kind:  "bad-kind",
					Group: helpers.GetPointer[v1.Group](v1.GroupName),
				},
				TLSRouteGroupKind,
			},
			expectErr: true,
			name:      "valid and invalid kinds",
			expected:  []v1.RouteGroupKind{HTTPRouteGroupKind},
		},
		{
			protocol: v1.HTTPProtocolType,
			kind: []v1.RouteGroupKind{
				HTTPRouteGroupKind,
				GRPCRouteGroupKind,
				GRPCRouteGroupKind,
			},
			expectErr: false,
			name:      "handle duplicate kinds",
			expected: []v1.RouteGroupKind{
				HTTPRouteGroupKind,
				GRPCRouteGroupKind,
			},
		},
		{
			protocol: v1.TLSProtocolType,
			kind: []v1.RouteGroupKind{
				HTTPRouteGroupKind,
				{
					Kind:  "bad-kind",
					Group: helpers.GetPointer[v1.Group](v1.GroupName),
				},
				TLSRouteGroupKind,
				GRPCRouteGroupKind,
			},
			expectErr: true,
			name:      "valid and invalid kinds for TLS protocol",
			expected:  []v1.RouteGroupKind{TLSRouteGroupKind},
		},
		{
			protocol: v1.TLSProtocolType,
			kind: []v1.RouteGroupKind{
				HTTPRouteGroupKind,
				{
					Kind:  "bad-kind",
					Group: helpers.GetPointer[v1.Group](v1.GroupName),
				},
				GRPCRouteGroupKind,
			},
			expectErr: true,
			name:      "invalid kinds for TLS protocol",
			expected:  []v1.RouteGroupKind{},
		},
		{
			protocol: v1.TLSProtocolType,
			kind: []v1.RouteGroupKind{
				TLSRouteGroupKind,
			},
			name:     "valid kinds for TLS protocol",
			expected: []v1.RouteGroupKind{TLSRouteGroupKind},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			listener := v1.Listener{
				Protocol: test.protocol,
			}

			if test.kind != nil {
				listener.AllowedRoutes = &v1.AllowedRoutes{
					Kinds: test.kind,
				}
			}

			conds, kinds := getAndValidateListenerSupportedKinds(listener)
			g.Expect(helpers.Diff(test.expected, kinds)).To(BeEmpty())
			if test.expectErr {
				g.Expect(conds).ToNot(BeEmpty())
			} else {
				g.Expect(conds).To(BeEmpty())
			}
		})
	}
}

func TestValidateListenerLabelSelector(t *testing.T) {
	t.Parallel()
	tests := []struct {
		selector  *metav1.LabelSelector
		from      v1.FromNamespaces
		name      string
		expectErr bool
	}{
		{
			from:      v1.NamespacesFromSelector,
			selector:  &metav1.LabelSelector{},
			expectErr: false,
			name:      "valid spec",
		},
		{
			from:      v1.NamespacesFromSelector,
			selector:  nil,
			expectErr: true,
			name:      "invalid spec",
		},
		{
			from:      v1.NamespacesFromAll,
			selector:  nil,
			expectErr: false,
			name:      "ignored from type",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			// create iteration variable inside the loop to fix implicit memory aliasing
			from := test.from

			listener := v1.Listener{
				AllowedRoutes: &v1.AllowedRoutes{
					Namespaces: &v1.RouteNamespaces{
						From:     &from,
						Selector: test.selector,
					},
				},
			}

			conds, attachable := validateListenerLabelSelector(listener)
			if test.expectErr {
				g.Expect(conds).ToNot(BeEmpty())
				g.Expect(attachable).To(BeFalse())
			} else {
				g.Expect(conds).To(BeEmpty())
				g.Expect(attachable).To(BeTrue())
			}
		})
	}
}

func TestValidateListenerPort(t *testing.T) {
	t.Parallel()
	validPorts := []v1.PortNumber{1, 80, 443, 1000, 50000, 65535}
	invalidPorts := []v1.PortNumber{-1, 0, 65536, 80000, 9113}
	protectedPorts := ProtectedPorts{9113: "MetricsPort"}

	for _, p := range validPorts {
		t.Run(fmt.Sprintf("valid port %d", p), func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(validateListenerPort(p, protectedPorts)).To(Succeed())
		})
	}

	for _, p := range invalidPorts {
		t.Run(fmt.Sprintf("invalid port %d", p), func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(validateListenerPort(p, protectedPorts)).ToNot(Succeed())
		})
	}
}

func TestListenerNamesHaveOverlap(t *testing.T) {
	t.Parallel()
	tests := []struct {
		hostname1    *v1.Hostname
		hostname2    *v1.Hostname
		msg          string
		expectResult bool
	}{
		{
			hostname1:    (*v1.Hostname)(helpers.GetPointer("*.example.com")),
			hostname2:    (*v1.Hostname)(helpers.GetPointer("*.example.com")),
			expectResult: true,
			msg:          "same hostnames with wildcard",
		},
		{
			hostname1:    nil,
			hostname2:    nil,
			expectResult: true,
			msg:          "two nil hostnames",
		},
		{
			hostname1:    (*v1.Hostname)(helpers.GetPointer("cafe.example.com")),
			hostname2:    (*v1.Hostname)(helpers.GetPointer("app.example.com")),
			expectResult: false,
			msg:          "two different hostnames no wildcard",
		},
		{
			hostname1:    (*v1.Hostname)(helpers.GetPointer("cafe.example.com")),
			hostname2:    nil,
			expectResult: true,
			msg:          "hostname1 is nil",
		},
		{
			hostname1:    nil,
			hostname2:    (*v1.Hostname)(helpers.GetPointer("cafe.example.com")),
			expectResult: true,
			msg:          "hostname2 is nil",
		},
		{
			hostname1:    (*v1.Hostname)(helpers.GetPointer("*.example.com")),
			hostname2:    (*v1.Hostname)(helpers.GetPointer("*.example.org")),
			expectResult: false,
			msg:          "wildcard hostnames that do not overlap",
		},
		{
			hostname1:    (*v1.Hostname)(helpers.GetPointer("*.example.com")),
			hostname2:    (*v1.Hostname)(helpers.GetPointer("cafe.example.com")),
			expectResult: true,
			msg:          "one wildcard hostname and one hostname that overlap",
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(haveOverlap(test.hostname1, test.hostname2)).To(Equal(test.expectResult))
		})
	}
}

func TestValidateTLSFieldOnTLSListener(t *testing.T) {
	t.Parallel()
	tests := []struct {
		listener     v1.Listener
		msg          string
		expectedCond []conditions.Condition
		expectValid  bool
	}{
		{
			listener: v1.Listener{},
			expectedCond: conditions.NewListenerUnsupportedValue(
				"TLS: Required value: tls must be defined for TLS listener",
			),
			expectValid: false,
			msg:         "TLS listener without tls field",
		},
		{
			listener: v1.Listener{TLS: nil},
			expectedCond: conditions.NewListenerUnsupportedValue(
				"TLS: Required value: tls must be defined for TLS listener",
			),
			expectValid: false,
			msg:         "TLS listener with TLS field nil",
		},
		{
			listener: v1.Listener{TLS: &v1.ListenerTLSConfig{Mode: helpers.GetPointer(v1.TLSModeTerminate)}},
			expectedCond: conditions.NewListenerUnsupportedValue(
				"TLS.Mode: Required value: Mode must be passthrough for TLS listener",
			),
			expectValid: false,
			msg:         "TLS listener with TLS mode terminate",
		},
		{
			listener:    v1.Listener{TLS: &v1.ListenerTLSConfig{Mode: helpers.GetPointer(v1.TLSModePassthrough)}},
			expectValid: true,
			msg:         "TLS listener with TLS mode passthrough",
		},
	}
	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			cond, valid := validateTLSFieldOnTLSListener(test.listener)

			g.Expect(cond).To(BeEquivalentTo(test.expectedCond))
			g.Expect(valid).To(BeEquivalentTo(test.expectValid))
		})
	}
}

func TestOverlappingTLSConfigCondition(t *testing.T) {
	t.Parallel()

	protectedPorts := ProtectedPorts{9113: "MetricsPort"}

	tests := []struct {
		gateway           *v1.Gateway
		name              string
		conditionReason   v1.ListenerConditionReason
		expectedCondition bool
	}{
		{
			name: "overlapping hostnames on same port",
			gateway: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gateway",
					Namespace: "test-ns",
				},
				Spec: v1.GatewaySpec{
					Listeners: []v1.Listener{
						{
							Name:     "listener1",
							Port:     443,
							Protocol: v1.HTTPSProtocolType,
							Hostname: helpers.GetPointer[v1.Hostname]("*.example.com"),
							TLS: &v1.ListenerTLSConfig{
								Mode: helpers.GetPointer(v1.TLSModeTerminate),
								CertificateRefs: []v1.SecretObjectReference{
									{Name: "secret1"},
								},
							},
						},
						{
							Name:     "listener2",
							Port:     443,
							Protocol: v1.HTTPSProtocolType,
							Hostname: helpers.GetPointer[v1.Hostname]("app.example.com"),
							TLS: &v1.ListenerTLSConfig{
								Mode: helpers.GetPointer(v1.TLSModeTerminate),
								CertificateRefs: []v1.SecretObjectReference{
									{Name: "secret2"},
								},
							},
						},
					},
				},
			},
			expectedCondition: true,
			conditionReason:   v1.ListenerReasonOverlappingHostnames,
		},
		{
			name: "no overlap - different ports",
			gateway: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gateway",
					Namespace: "test-ns",
				},
				Spec: v1.GatewaySpec{
					Listeners: []v1.Listener{
						{
							Name:     "listener1",
							Port:     443,
							Protocol: v1.HTTPSProtocolType,
							Hostname: helpers.GetPointer[v1.Hostname]("*.example.com"),
							TLS: &v1.ListenerTLSConfig{
								Mode: helpers.GetPointer(v1.TLSModeTerminate),
								CertificateRefs: []v1.SecretObjectReference{
									{Name: "secret1"},
								},
							},
						},
						{
							Name:     "listener2",
							Port:     8443,
							Protocol: v1.HTTPSProtocolType,
							Hostname: helpers.GetPointer[v1.Hostname]("app.example.com"),
							TLS: &v1.ListenerTLSConfig{
								Mode: helpers.GetPointer(v1.TLSModeTerminate),
								CertificateRefs: []v1.SecretObjectReference{
									{Name: "secret2"},
								},
							},
						},
					},
				},
			},
			expectedCondition: false,
		},
		{
			name: "no overlap - different hostnames same port",
			gateway: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gateway",
					Namespace: "test-ns",
				},
				Spec: v1.GatewaySpec{
					Listeners: []v1.Listener{
						{
							Name:     "listener1",
							Port:     443,
							Protocol: v1.HTTPSProtocolType,
							Hostname: helpers.GetPointer[v1.Hostname]("app.example.com"),
							TLS: &v1.ListenerTLSConfig{
								Mode: helpers.GetPointer(v1.TLSModeTerminate),
								CertificateRefs: []v1.SecretObjectReference{
									{Name: "secret1"},
								},
							},
						},
						{
							Name:     "listener2",
							Port:     443,
							Protocol: v1.HTTPSProtocolType,
							Hostname: helpers.GetPointer[v1.Hostname]("cafe.example.org"),
							TLS: &v1.ListenerTLSConfig{
								Mode: helpers.GetPointer(v1.TLSModeTerminate),
								CertificateRefs: []v1.SecretObjectReference{
									{Name: "secret2"},
								},
							},
						},
					},
				},
			},
			expectedCondition: false,
		},
		{
			name: "overlap between HTTPS and TLS listeners",
			gateway: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gateway",
					Namespace: "test-ns",
				},
				Spec: v1.GatewaySpec{
					Listeners: []v1.Listener{
						{
							Name:     "listener1",
							Port:     443,
							Protocol: v1.HTTPSProtocolType,
							Hostname: helpers.GetPointer[v1.Hostname]("*.example.com"),
							TLS: &v1.ListenerTLSConfig{
								Mode: helpers.GetPointer(v1.TLSModeTerminate),
								CertificateRefs: []v1.SecretObjectReference{
									{Name: "secret1"},
								},
							},
						},
						{
							Name:     "listener2",
							Port:     443,
							Protocol: v1.TLSProtocolType,
							Hostname: helpers.GetPointer[v1.Hostname]("app.example.com"),
							TLS: &v1.ListenerTLSConfig{
								Mode: helpers.GetPointer(v1.TLSModePassthrough),
							},
						},
					},
				},
			},
			expectedCondition: true,
			conditionReason:   v1.ListenerReasonOverlappingHostnames,
		},
		{
			name: "overlap with nil hostnames",
			gateway: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gateway",
					Namespace: "test-ns",
				},
				Spec: v1.GatewaySpec{
					Listeners: []v1.Listener{
						{
							Name:     "listener1",
							Port:     443,
							Protocol: v1.HTTPSProtocolType,
							Hostname: nil, // nil hostname matches everything
							TLS: &v1.ListenerTLSConfig{
								Mode: helpers.GetPointer(v1.TLSModeTerminate),
								CertificateRefs: []v1.SecretObjectReference{
									{Name: "secret1"},
								},
							},
						},
						{
							Name:     "listener2",
							Port:     443,
							Protocol: v1.HTTPSProtocolType,
							Hostname: helpers.GetPointer[v1.Hostname]("app.example.com"),
							TLS: &v1.ListenerTLSConfig{
								Mode: helpers.GetPointer(v1.TLSModeTerminate),
								CertificateRefs: []v1.SecretObjectReference{
									{Name: "secret2"},
								},
							},
						},
					},
				},
			},
			expectedCondition: true,
			conditionReason:   v1.ListenerReasonOverlappingHostnames,
		},
		{
			name: "no overlap - HTTP listener excluded",
			gateway: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gateway",
					Namespace: "test-ns",
				},
				Spec: v1.GatewaySpec{
					Listeners: []v1.Listener{
						{
							Name:     "listener1",
							Port:     80,
							Protocol: v1.HTTPProtocolType,
							Hostname: helpers.GetPointer[v1.Hostname]("*.example.com"),
						},
						{
							Name:     "listener2",
							Port:     80,
							Protocol: v1.HTTPProtocolType,
							Hostname: helpers.GetPointer[v1.Hostname]("app.example.com"),
						},
					},
				},
			},
			expectedCondition: false,
		},
		{
			name: "no overlap - HTTP and HTTPS listeners with same hostname and port",
			gateway: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gateway",
					Namespace: "test-ns",
				},
				Spec: v1.GatewaySpec{
					Listeners: []v1.Listener{
						{
							Name:     "listener1",
							Port:     80,
							Protocol: v1.HTTPProtocolType,
							Hostname: helpers.GetPointer[v1.Hostname]("app.example.com"),
						},
						{
							Name:     "listener2",
							Port:     80,
							Protocol: v1.HTTPSProtocolType,
							Hostname: helpers.GetPointer[v1.Hostname]("app.example.com"),
							TLS: &v1.ListenerTLSConfig{
								Mode: helpers.GetPointer(v1.TLSModeTerminate),
								CertificateRefs: []v1.SecretObjectReference{
									{Name: "secret1"},
								},
							},
						},
					},
				},
			},
			expectedCondition: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			// Create mock resolvers
			secretResolver := newSecretResolver(nil)
			refGrantResolver := newReferenceGrantResolver(nil)

			// Build listeners
			listeners := buildListeners(test.gateway, secretResolver, refGrantResolver, protectedPorts)

			if test.expectedCondition {
				// Check that the expected listeners have the OverlappingTLSConfig condition
				listenersWithCondition := 0
				for _, listener := range listeners {
					found := false
					for _, cond := range listener.Conditions {
						if cond.Type == string(v1.ListenerConditionOverlappingTLSConfig) {
							g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
							g.Expect(cond.Reason).To(Equal(string(test.conditionReason)))
							found = true
							break
						}
					}
					if found {
						listenersWithCondition++
					}
				}
				// At least 2 listeners should have the condition when there's overlap
				g.Expect(listenersWithCondition).To(
					BeNumerically(">=", 2),
					"at least 2 listeners should have OverlappingTLSConfig condition",
				)
			} else {
				// No listener should have the OverlappingTLSConfig condition
				for i, listener := range listeners {
					for _, cond := range listener.Conditions {
						g.Expect(cond.Type).ToNot(Equal(string(v1.ListenerConditionOverlappingTLSConfig)),
							"listener %d should not have OverlappingTLSConfig condition", i)
					}
				}
			}
		})
	}
}

package graph

import (
	"errors"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/resolver"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/resolver/resolverfakes"
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
					Options:         map[v1.AnnotationKey]v1.AnnotationValue{"unsupported-key": "val"},
				},
			},
			expected: conditions.NewListenerUnsupportedValue(
				`tls.options[unsupported-key]: Unsupported value: "unsupported-key": ` +
					`supported values: "nginx.org/ssl-protocols", "nginx.org/ssl-ciphers", "nginx.org/ssl-prefer-server-ciphers"`,
			),
			name: "unsupported options",
		},
		{
			l: v1.Listener{
				Port: 443,
				TLS: &v1.ListenerTLSConfig{
					Mode:            helpers.GetPointer(v1.TLSModeTerminate),
					CertificateRefs: []v1.SecretObjectReference{validSecretRef},
					Options: map[v1.AnnotationKey]v1.AnnotationValue{
						"nginx.org/ssl-protocols":             "TLSv1.2 TLSv1.3",
						"nginx.org/ssl-ciphers":               "HIGH:!aNULL:!MD5",
						"nginx.org/ssl-prefer-server-ciphers": "on",
					},
				},
			},
			expected: nil,
			name:     "valid supported options",
		},
		{
			l: v1.Listener{
				Port: 443,
				TLS: &v1.ListenerTLSConfig{
					Mode:            helpers.GetPointer(v1.TLSModeTerminate),
					CertificateRefs: []v1.SecretObjectReference{validSecretRef},
					Options: map[v1.AnnotationKey]v1.AnnotationValue{
						"nginx.org/ssl-protocols": "unknown",
					},
				},
			},
			expected: conditions.NewListenerUnsupportedValue(
				`tls.options[nginx.org/ssl-protocols]: Unsupported value: "unknown": ` +
					`supported values: "SSLv2", "SSLv3", "TLSv1", "TLSv1.1", "TLSv1.2", "TLSv1.3"`,
			),
			name: "invalid nginx.org/ssl-protocols value",
		},
		{
			l: v1.Listener{
				Port: 443,
				TLS: &v1.ListenerTLSConfig{
					Mode:            helpers.GetPointer(v1.TLSModeTerminate),
					CertificateRefs: []v1.SecretObjectReference{validSecretRef},
					Options: map[v1.AnnotationKey]v1.AnnotationValue{
						"nginx.org/ssl-protocols": "",
					},
				},
			},
			expected: conditions.NewListenerUnsupportedValue(
				`tls.options[nginx.org/ssl-protocols]: Unsupported value: "": ` +
					`supported values: "SSLv2", "SSLv3", "TLSv1", "TLSv1.1", "TLSv1.2", "TLSv1.3"`,
			),
			name: "invalid nginx.org/ssl-protocols empty value",
		},
		{
			l: v1.Listener{
				Port: 443,
				TLS: &v1.ListenerTLSConfig{
					Mode:            helpers.GetPointer(v1.TLSModeTerminate),
					CertificateRefs: []v1.SecretObjectReference{validSecretRef},
					Options: map[v1.AnnotationKey]v1.AnnotationValue{
						"nginx.org/ssl-ciphers": "unknown!",
					},
				},
			},
			expected: conditions.NewListenerUnsupportedValue(
				`tls.options[nginx.org/ssl-ciphers]: Invalid value: "unknown!": invalid ssl ciphers`,
			),
			name: "invalid nginx.org/ssl-ciphers value",
		},
		{
			l: v1.Listener{
				Port: 443,
				TLS: &v1.ListenerTLSConfig{
					Mode:            helpers.GetPointer(v1.TLSModeTerminate),
					CertificateRefs: []v1.SecretObjectReference{validSecretRef},
					Options: map[v1.AnnotationKey]v1.AnnotationValue{
						"nginx.org/ssl-ciphers": "",
					},
				},
			},
			expected: conditions.NewListenerUnsupportedValue(
				`tls.options[nginx.org/ssl-ciphers]: Invalid value: "": invalid ssl ciphers`,
			),
			name: "invalid nginx.org/ssl-ciphers empty value",
		},
		{
			l: v1.Listener{
				Port: 443,
				TLS: &v1.ListenerTLSConfig{
					Mode:            helpers.GetPointer(v1.TLSModeTerminate),
					CertificateRefs: []v1.SecretObjectReference{validSecretRef},
					Options: map[v1.AnnotationKey]v1.AnnotationValue{
						"nginx.org/ssl-prefer-server-ciphers": "unknown",
					},
				},
			},
			expected: conditions.NewListenerUnsupportedValue(
				`tls.options[nginx.org/ssl-prefer-server-ciphers]: Unsupported value: "unknown": supported values: "on", "off"`,
			),
			name: "invalid nginx.org/ssl-prefer-server-ciphers value",
		},
		{
			l: v1.Listener{
				Port: 443,
				TLS: &v1.ListenerTLSConfig{
					Mode:            helpers.GetPointer(v1.TLSModeTerminate),
					CertificateRefs: []v1.SecretObjectReference{validSecretRef},
					Options: map[v1.AnnotationKey]v1.AnnotationValue{
						"nginx.org/ssl-prefer-server-ciphers": "",
					},
				},
			},
			expected: conditions.NewListenerUnsupportedValue(
				`tls.options[nginx.org/ssl-prefer-server-ciphers]: Unsupported value: "": supported values: "on", "off"`,
			),
			name: "invalid nginx.org/ssl-prefer-server-ciphers empty value",
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
			expected: conditions.NewListenerInvalidCertificateRefNotAccepted(
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
			expected: conditions.NewListenerInvalidCertificateRefNotAccepted(
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
			expected: conditions.NewListenerInvalidCertificateRefNotAccepted(
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
			expected: nil,
			name:     "multiple cert refs",
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
		tls       *v1.ListenerTLSConfig
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
			tls: &v1.ListenerTLSConfig{
				Mode: helpers.GetPointer(v1.TLSModePassthrough),
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
			tls: &v1.ListenerTLSConfig{
				Mode: helpers.GetPointer(v1.TLSModePassthrough),
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
			tls: &v1.ListenerTLSConfig{
				Mode: helpers.GetPointer(v1.TLSModeTerminate),
			},
			expectErr: true,
			name:      "invalid mode for TLS protocol",
			expected:  []v1.RouteGroupKind{},
		},
		{
			protocol: v1.TLSProtocolType,
			kind: []v1.RouteGroupKind{
				TLSRouteGroupKind,
			},
			tls: &v1.ListenerTLSConfig{
				Mode: helpers.GetPointer(v1.TLSModePassthrough),
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

			if test.tls != nil {
				listener.TLS = test.tls
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
		{
			hostname1:    (*v1.Hostname)(helpers.GetPointer("example.com")),
			hostname2:    (*v1.Hostname)(helpers.GetPointer("*.example.com")),
			expectResult: false,
			msg:          "one wildcard hostname and one hostname that does not overlap",
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
		{
			name: "no overlap between two TLS listeners",
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
								Mode: helpers.GetPointer(v1.TLSModePassthrough),
							},
						},
						{
							Name:     "listener2",
							Port:     443,
							Protocol: v1.TLSProtocolType,
							Hostname: helpers.GetPointer[v1.Hostname]("example.com"),
							TLS: &v1.ListenerTLSConfig{
								Mode: helpers.GetPointer(v1.TLSModePassthrough),
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
			refGrantResolver := newReferenceGrantResolver(nil)

			// Build listeners
			listeners := buildListeners(
				&Gateway{Source: test.gateway},
				&resolverfakes.FakeResolver{},
				refGrantResolver,
				protectedPorts,
			)

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

func TestMatchesWildcard(t *testing.T) {
	t.Parallel()
	tests := []struct {
		hostname    string
		wildcard    string
		expectMatch bool
	}{
		// Basic wildcard matching
		{
			hostname:    "www.example.com",
			wildcard:    "*.example.com",
			expectMatch: true,
		},
		// Apex domain should NOT match wildcard
		{
			hostname:    "example.com",
			wildcard:    "*.example.com",
			expectMatch: false,
		},
		// Multi-level subdomains should match
		{
			hostname:    "foo.bar.example.com",
			wildcard:    "*.example.com",
			expectMatch: true,
		},
		// Different domains should NOT match
		{
			hostname:    "www.other.com",
			wildcard:    "*.example.com",
			expectMatch: false,
		},
		// Nested wildcard matching
		{
			hostname:    "api.prod.example.com",
			wildcard:    "*.prod.example.com",
			expectMatch: true,
		},
		// Both wildcards - should match if they overlap
		{
			hostname:    "*.api.example.com",
			wildcard:    "*.example.com",
			expectMatch: true,
		},
		{
			hostname:    "*.example.com",
			wildcard:    "*.example.com",
			expectMatch: true,
		},
		// reverse order
		{
			hostname:    "*.example.com",
			wildcard:    "www.example.com",
			expectMatch: true,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("hostname: %s, wildcard: %s", test.hostname, test.wildcard), func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(matchesWildcard(test.hostname, test.wildcard)).To(Equal(test.expectMatch))
		})
	}
}

func TestCreateFrontendTLSCaCertReferenceResolver(t *testing.T) {
	t.Parallel()
	gwNamespace := "default"
	listenerName := v1.SectionName("https")
	listenerPort := v1.PortNumber(443)
	caCertName := "ca-cert"

	tests := []struct {
		listener               *Listener
		gateway                *Gateway
		name                   string
		expectedValidationMode v1.FrontendValidationModeType
		expectedCACertRefNames []string
		expectedGatewayCondLen int
		expectedCACertRefs     bool
		expectedListenerValid  bool
	}{
		{
			name: "Gateway with no TLS spec",
			listener: &Listener{
				Source: v1.Listener{
					Name: listenerName,
					Port: listenerPort,
					TLS: &v1.ListenerTLSConfig{
						Mode: helpers.GetPointer(v1.TLSModeTerminate),
					},
				},
				Valid:      true,
				Conditions: []conditions.Condition{},
			},
			gateway: &Gateway{
				Source: &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-gw",
						Namespace: gwNamespace,
					},
					Spec: v1.GatewaySpec{},
				},
				Conditions: []conditions.Condition{},
			},
			expectedCACertRefs:     false,
			expectedValidationMode: "",
			expectedGatewayCondLen: 0,
			expectedListenerValid:  true,
		},
		{
			name: "Gateway with TLS and no Frontend",
			listener: &Listener{
				Source: v1.Listener{
					Name: listenerName,
					Port: listenerPort,
					TLS: &v1.ListenerTLSConfig{
						Mode: helpers.GetPointer(v1.TLSModeTerminate),
					},
				},
				Valid:      true,
				Conditions: []conditions.Condition{},
			},
			gateway: &Gateway{
				Source: &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-gw",
						Namespace: gwNamespace,
					},
					Spec: v1.GatewaySpec{
						TLS: &v1.GatewayTLSConfig{},
					},
				},
				Conditions: []conditions.Condition{},
			},
			expectedCACertRefs:     false,
			expectedValidationMode: "",
			expectedGatewayCondLen: 0,
			expectedListenerValid:  true,
		},
		{
			name: "Listener with no TLS",
			listener: &Listener{
				Source: v1.Listener{
					Name: listenerName,
					Port: listenerPort,
				},
				Valid:      true,
				Conditions: []conditions.Condition{},
			},
			gateway: &Gateway{
				Source: &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-gw",
						Namespace: gwNamespace,
					},
					Spec: v1.GatewaySpec{
						TLS: &v1.GatewayTLSConfig{
							Frontend: &v1.FrontendTLSConfig{
								Default: v1.TLSConfig{
									Validation: &v1.FrontendTLSValidation{
										Mode: v1.AllowValidOnly,
										CACertificateRefs: []v1.ObjectReference{
											{
												Name: v1.ObjectName(caCertName),
												Kind: v1.Kind("Secret"),
											},
										},
									},
								},
							},
						},
					},
				},
				Conditions: []conditions.Condition{},
			},
			expectedCACertRefs:     false,
			expectedValidationMode: "",
			expectedGatewayCondLen: 0,
			expectedListenerValid:  true,
		},
		{
			name: "Listener TLS mode Passthrough",
			listener: &Listener{
				Source: v1.Listener{
					Name: listenerName,
					Port: listenerPort,
					TLS: &v1.ListenerTLSConfig{
						Mode: helpers.GetPointer(v1.TLSModePassthrough),
					},
				},
				Valid:      true,
				Conditions: []conditions.Condition{},
			},
			gateway: &Gateway{
				Source: &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-gw",
						Namespace: gwNamespace,
					},
					Spec: v1.GatewaySpec{
						TLS: &v1.GatewayTLSConfig{
							Frontend: &v1.FrontendTLSConfig{
								Default: v1.TLSConfig{
									Validation: &v1.FrontendTLSValidation{
										Mode: v1.AllowValidOnly,
										CACertificateRefs: []v1.ObjectReference{
											{
												Name: v1.ObjectName(caCertName),
												Kind: v1.Kind("Secret"),
											},
										},
									},
								},
							},
						},
					},
				},
				Conditions: []conditions.Condition{},
			},
			expectedCACertRefs:     false,
			expectedValidationMode: "",
			expectedGatewayCondLen: 0,
			expectedListenerValid:  true,
		},
		{
			name: "Per-port configuration where port matches",
			listener: &Listener{
				Source: v1.Listener{
					Name: listenerName,
					Port: listenerPort,
					TLS: &v1.ListenerTLSConfig{
						Mode: helpers.GetPointer(v1.TLSModeTerminate),
					},
				},
				Valid:      true,
				Conditions: []conditions.Condition{},
			},
			gateway: &Gateway{
				Source: &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-gw",
						Namespace: gwNamespace,
					},
					Spec: v1.GatewaySpec{
						TLS: &v1.GatewayTLSConfig{
							Frontend: &v1.FrontendTLSConfig{
								PerPort: []v1.TLSPortConfig{
									{
										Port: listenerPort,
										TLS: v1.TLSConfig{
											Validation: &v1.FrontendTLSValidation{
												Mode: v1.AllowValidOnly,
												CACertificateRefs: []v1.ObjectReference{
													{
														Name: v1.ObjectName(caCertName),
														Kind: v1.Kind("Secret"),
													},
												},
											},
										},
									},
								},
								Default: v1.TLSConfig{
									Validation: &v1.FrontendTLSValidation{
										Mode: v1.AllowInsecureFallback,
									},
								},
							},
						},
					},
				},
				Conditions: []conditions.Condition{},
			},
			expectedCACertRefs:     true,
			expectedValidationMode: v1.AllowValidOnly,
			expectedGatewayCondLen: 0,
			expectedListenerValid:  true,
		},
		{
			name: "Uses Default when Per-port port doesn't match",
			listener: &Listener{
				Source: v1.Listener{
					Name: listenerName,
					Port: v1.PortNumber(444), // Different port
					TLS: &v1.ListenerTLSConfig{
						Mode: helpers.GetPointer(v1.TLSModeTerminate),
					},
				},
				Valid:      true,
				Conditions: []conditions.Condition{},
			},
			gateway: &Gateway{
				Source: &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-gw",
						Namespace: gwNamespace,
					},
					Spec: v1.GatewaySpec{
						TLS: &v1.GatewayTLSConfig{
							Frontend: &v1.FrontendTLSConfig{
								PerPort: []v1.TLSPortConfig{
									{
										Port: listenerPort,
										TLS: v1.TLSConfig{
											Validation: &v1.FrontendTLSValidation{
												Mode: v1.AllowValidOnly,
												CACertificateRefs: []v1.ObjectReference{
													{
														Name: v1.ObjectName(caCertName),
														Kind: v1.Kind("Secret"),
													},
												},
											},
										},
									},
								},
								Default: v1.TLSConfig{
									Validation: &v1.FrontendTLSValidation{
										Mode: v1.AllowInsecureFallback,
										CACertificateRefs: []v1.ObjectReference{
											{
												Name: v1.ObjectName("default-ca-cert"),
												Kind: v1.Kind("Secret"),
											},
										},
									},
								},
							},
						},
					},
				},
				Conditions: []conditions.Condition{},
			},
			expectedCACertRefs:     true,
			expectedValidationMode: v1.AllowInsecureFallback,
			expectedGatewayCondLen: 1,
			expectedListenerValid:  true,
		},
		{
			name: "Uses Default when no Per-port config",
			listener: &Listener{
				Source: v1.Listener{
					Name: listenerName,
					Port: listenerPort,
					TLS: &v1.ListenerTLSConfig{
						Mode: helpers.GetPointer(v1.TLSModeTerminate),
					},
				},
				Valid:      true,
				Conditions: []conditions.Condition{},
			},
			gateway: &Gateway{
				Source: &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-gw",
						Namespace: gwNamespace,
					},
					Spec: v1.GatewaySpec{
						TLS: &v1.GatewayTLSConfig{
							Frontend: &v1.FrontendTLSConfig{
								Default: v1.TLSConfig{
									Validation: &v1.FrontendTLSValidation{
										Mode: v1.AllowValidOnly,
										CACertificateRefs: []v1.ObjectReference{
											{
												Name: v1.ObjectName(caCertName),
												Kind: v1.Kind("Secret"),
											},
										},
									},
								},
							},
						},
					},
				},
				Conditions: []conditions.Condition{},
			},
			expectedCACertRefs:     true,
			expectedValidationMode: v1.AllowValidOnly,
			expectedGatewayCondLen: 0,
			expectedListenerValid:  true,
		},
		{
			name: "Adds gateway condition when validation mode is AllowInsecureFallback",
			listener: &Listener{
				Source: v1.Listener{
					Name: listenerName,
					Port: listenerPort,
					TLS: &v1.ListenerTLSConfig{
						Mode: helpers.GetPointer(v1.TLSModeTerminate),
					},
				},
				Valid:      true,
				Conditions: []conditions.Condition{},
			},
			gateway: &Gateway{
				Source: &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-gw",
						Namespace: gwNamespace,
					},
					Spec: v1.GatewaySpec{
						TLS: &v1.GatewayTLSConfig{
							Frontend: &v1.FrontendTLSConfig{
								Default: v1.TLSConfig{
									Validation: &v1.FrontendTLSValidation{
										Mode: v1.AllowInsecureFallback,
										CACertificateRefs: []v1.ObjectReference{
											{
												Name: v1.ObjectName(caCertName),
												Kind: v1.Kind("Secret"),
											},
										},
									},
								},
							},
						},
					},
				},
				Conditions: []conditions.Condition{},
			},
			expectedCACertRefs:     true,
			expectedValidationMode: v1.AllowInsecureFallback,
			expectedGatewayCondLen: 1,
			expectedListenerValid:  true,
		},
		{
			name: "Uses Default when no CA cert refs in Per-port",
			listener: &Listener{
				Source: v1.Listener{
					Name: listenerName,
					Port: listenerPort,
					TLS: &v1.ListenerTLSConfig{
						Mode: helpers.GetPointer(v1.TLSModeTerminate),
					},
				},
				Valid:      true,
				Conditions: []conditions.Condition{},
			},
			gateway: &Gateway{
				Source: &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-gw",
						Namespace: gwNamespace,
					},
					Spec: v1.GatewaySpec{
						TLS: &v1.GatewayTLSConfig{
							Frontend: &v1.FrontendTLSConfig{
								PerPort: []v1.TLSPortConfig{
									{
										Port: listenerPort,
										TLS: v1.TLSConfig{
											Validation: &v1.FrontendTLSValidation{
												Mode: v1.AllowValidOnly,
											},
										},
									},
								},
								Default: v1.TLSConfig{
									Validation: &v1.FrontendTLSValidation{
										Mode: v1.AllowInsecureFallback,
										CACertificateRefs: []v1.ObjectReference{
											{
												Name: v1.ObjectName("default-ca-cert"),
												Kind: v1.Kind("Secret"),
											},
										},
									},
								},
							},
						},
					},
				},
				Conditions: []conditions.Condition{},
			},
			expectedCACertRefs:     true,
			expectedValidationMode: v1.AllowInsecureFallback,
			expectedGatewayCondLen: 1,
			expectedListenerValid:  true,
		},
		{
			name: "Uses Default when Per-port validation is nil",
			listener: &Listener{
				Source: v1.Listener{
					Name: listenerName,
					Port: listenerPort,
					TLS: &v1.ListenerTLSConfig{
						Mode: helpers.GetPointer(v1.TLSModeTerminate),
					},
				},
				Valid:      true,
				Conditions: []conditions.Condition{},
			},
			gateway: &Gateway{
				Source: &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-gw",
						Namespace: gwNamespace,
					},
					Spec: v1.GatewaySpec{
						TLS: &v1.GatewayTLSConfig{
							Frontend: &v1.FrontendTLSConfig{
								PerPort: []v1.TLSPortConfig{
									{
										Port: listenerPort,
										TLS:  v1.TLSConfig{},
									},
								},
								Default: v1.TLSConfig{
									Validation: &v1.FrontendTLSValidation{
										Mode: v1.AllowInsecureFallback,
										CACertificateRefs: []v1.ObjectReference{
											{
												Name: v1.ObjectName("default-ca-cert"),
												Kind: v1.Kind("Secret"),
											},
										},
									},
								},
							},
						},
					},
				},
				Conditions: []conditions.Condition{},
			},
			expectedCACertRefs:     true,
			expectedValidationMode: v1.AllowInsecureFallback,
			expectedGatewayCondLen: 1,
			expectedListenerValid:  true,
		},
		{
			name: "Default and Per-port config: 443 uses Default",
			listener: &Listener{
				Source: v1.Listener{
					Name: v1.SectionName("https-443"),
					Port: v1.PortNumber(443),
					TLS: &v1.ListenerTLSConfig{
						Mode: helpers.GetPointer(v1.TLSModeTerminate),
					},
				},
				Valid:      true,
				Conditions: []conditions.Condition{},
			},
			gateway: &Gateway{
				Source: &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-gw",
						Namespace: gwNamespace,
					},
					Spec: v1.GatewaySpec{
						TLS: &v1.GatewayTLSConfig{
							Frontend: &v1.FrontendTLSConfig{
								Default: v1.TLSConfig{
									Validation: &v1.FrontendTLSValidation{
										Mode: v1.AllowValidOnly,
										CACertificateRefs: []v1.ObjectReference{
											{
												Name: v1.ObjectName("default-ca-cert"),
												Kind: v1.Kind("Secret"),
											},
										},
									},
								},
								PerPort: []v1.TLSPortConfig{
									{
										Port: v1.PortNumber(8443),
										TLS: v1.TLSConfig{
											Validation: &v1.FrontendTLSValidation{
												Mode: v1.AllowInsecureFallback,
												CACertificateRefs: []v1.ObjectReference{
													{
														Name: v1.ObjectName("8443-secret-ca"),
														Kind: v1.Kind("Secret"),
													},
													{
														Name: v1.ObjectName("8443-configmap-ca"),
														Kind: v1.Kind("ConfigMap"),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Conditions: []conditions.Condition{},
			},
			expectedCACertRefs:     true,
			expectedCACertRefNames: []string{"default-ca-cert"},
			expectedValidationMode: v1.AllowValidOnly,
			expectedGatewayCondLen: 0,
			expectedListenerValid:  true,
		},
		{
			name: "Uses Default and Per-port config",
			listener: &Listener{
				Source: v1.Listener{
					Name: v1.SectionName("https-8443"),
					Port: v1.PortNumber(8443),
					TLS: &v1.ListenerTLSConfig{
						Mode: helpers.GetPointer(v1.TLSModeTerminate),
					},
				},
				Valid:      true,
				Conditions: []conditions.Condition{},
			},
			gateway: &Gateway{
				Source: &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-gw",
						Namespace: gwNamespace,
					},
					Spec: v1.GatewaySpec{
						TLS: &v1.GatewayTLSConfig{
							Frontend: &v1.FrontendTLSConfig{
								Default: v1.TLSConfig{
									Validation: &v1.FrontendTLSValidation{
										Mode: v1.AllowValidOnly,
										CACertificateRefs: []v1.ObjectReference{
											{
												Name: v1.ObjectName("default-ca-cert"),
												Kind: v1.Kind("Secret"),
											},
										},
									},
								},
								PerPort: []v1.TLSPortConfig{
									{
										Port: v1.PortNumber(8443),
										TLS: v1.TLSConfig{
											Validation: &v1.FrontendTLSValidation{
												Mode: v1.AllowInsecureFallback,
												CACertificateRefs: []v1.ObjectReference{
													{
														Name: v1.ObjectName("8443-secret-ca"),
														Kind: v1.Kind("Secret"),
													},
													{
														Name: v1.ObjectName("8443-configmap-ca"),
														Kind: v1.Kind("ConfigMap"),
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Conditions: []conditions.Condition{},
			},
			expectedCACertRefs:     true,
			expectedCACertRefNames: []string{"8443-secret-ca", "8443-configmap-ca"},
			expectedValidationMode: v1.AllowInsecureFallback,
			expectedGatewayCondLen: 1,
			expectedListenerValid:  true,
		},
		{
			name: "Listener TLS mode is nil (no mode specified - should default to Terminate behavior)",
			listener: &Listener{
				Source: v1.Listener{
					Name: listenerName,
					Port: listenerPort,
					TLS:  &v1.ListenerTLSConfig{}, // No Mode specified.
				},
				Valid:      true,
				Conditions: []conditions.Condition{},
			},
			gateway: &Gateway{
				Source: &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-gw",
						Namespace: gwNamespace,
					},
					Spec: v1.GatewaySpec{
						TLS: &v1.GatewayTLSConfig{
							Frontend: &v1.FrontendTLSConfig{
								Default: v1.TLSConfig{
									Validation: &v1.FrontendTLSValidation{
										Mode: v1.AllowValidOnly,
										CACertificateRefs: []v1.ObjectReference{
											{
												Name: v1.ObjectName(caCertName),
												Kind: v1.Kind("Secret"),
											},
										},
									},
								},
							},
						},
					},
				},
				Conditions: []conditions.Condition{},
			},
			expectedCACertRefs:     true,
			expectedValidationMode: v1.AllowValidOnly,
			expectedGatewayCondLen: 0,
			expectedListenerValid:  true,
		},
		{
			name: "Default validation is nil and Per-port validation is nil",
			listener: &Listener{
				Source: v1.Listener{
					Name: listenerName,
					Port: listenerPort,
					TLS: &v1.ListenerTLSConfig{
						Mode: helpers.GetPointer(v1.TLSModeTerminate),
					},
				},
				Valid:      true,
				Conditions: []conditions.Condition{},
			},
			gateway: &Gateway{
				Source: &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-gw",
						Namespace: gwNamespace,
					},
					Spec: v1.GatewaySpec{
						TLS: &v1.GatewayTLSConfig{
							Frontend: &v1.FrontendTLSConfig{
								Default: v1.TLSConfig{},       // No Validation specified
								PerPort: []v1.TLSPortConfig{}, // No Per-port configs
							},
						},
					},
				},
				Conditions: []conditions.Condition{},
			},
			expectedCACertRefs:     false,
			expectedValidationMode: "",
			expectedGatewayCondLen: 0,
			expectedListenerValid:  true,
		},
		{
			name: "Multiple Per-port configs",
			listener: &Listener{
				Source: v1.Listener{
					Name: listenerName,
					Port: v1.PortNumber(8443),
					TLS: &v1.ListenerTLSConfig{
						Mode: helpers.GetPointer(v1.TLSModeTerminate),
					},
				},
				Valid:      true,
				Conditions: []conditions.Condition{},
			},
			gateway: &Gateway{
				Source: &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-gw",
						Namespace: gwNamespace,
					},
					Spec: v1.GatewaySpec{
						TLS: &v1.GatewayTLSConfig{
							Frontend: &v1.FrontendTLSConfig{
								PerPort: []v1.TLSPortConfig{
									{
										Port: v1.PortNumber(443),
										TLS: v1.TLSConfig{
											Validation: &v1.FrontendTLSValidation{
												Mode: v1.AllowValidOnly,
											},
										},
									},
									{
										Port: v1.PortNumber(8443),
										TLS: v1.TLSConfig{
											Validation: &v1.FrontendTLSValidation{
												Mode: v1.AllowInsecureFallback,
												CACertificateRefs: []v1.ObjectReference{
													{
														Name: v1.ObjectName("8443-ca-cert"),
														Kind: v1.Kind("Secret"),
													},
												},
											},
										},
									},
								},
								Default: v1.TLSConfig{
									Validation: nil,
								},
							},
						},
					},
				},
				Conditions: []conditions.Condition{},
			},
			expectedCACertRefs:     true,
			expectedValidationMode: v1.AllowInsecureFallback,
			expectedGatewayCondLen: 1,
			expectedListenerValid:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			fakeResolver := &resolverfakes.FakeResolver{}
			refGrantResolver := &referenceGrantResolver{}

			resolverFunc := createFrontendTLSCaCertReferenceResolver(fakeResolver, refGrantResolver)
			resolverFunc(test.listener, test.gateway)

			if test.expectedCACertRefs {
				g.Expect(test.listener.CACertificateRefs).ToNot(BeEmpty())
			} else {
				g.Expect(test.listener.CACertificateRefs).To(BeNil())
			}

			if len(test.expectedCACertRefNames) > 0 {
				g.Expect(test.listener.CACertificateRefs).To(HaveLen(len(test.expectedCACertRefNames)))
				actualRefNames := make([]string, 0, len(test.listener.CACertificateRefs))
				for _, ref := range test.listener.CACertificateRefs {
					actualRefNames = append(actualRefNames, string(ref.Name))
				}
				g.Expect(actualRefNames).To(Equal(test.expectedCACertRefNames))
			}

			g.Expect(test.listener.ValidationMode).To(Equal(test.expectedValidationMode))
			g.Expect(test.gateway.Conditions).To(HaveLen(test.expectedGatewayCondLen))
			g.Expect(test.listener.Valid).To(Equal(test.expectedListenerValid))
		})
	}
}

func TestCreateFrontendTLSCaCertReferenceResolverConditions(t *testing.T) {
	t.Parallel()

	namespace := func(ns string) *v1.Namespace {
		n := v1.Namespace(ns)
		return &n
	}

	secretRef := func(name string, ns *v1.Namespace) v1.ObjectReference {
		return v1.ObjectReference{
			Name:      v1.ObjectName(name),
			Kind:      v1.Kind(kinds.Secret),
			Namespace: ns,
		}
	}

	secretRefPtr := func(name string, ns *v1.Namespace) *v1.ObjectReference {
		ref := secretRef(name, ns)
		return &ref
	}

	configMapRef := func(name string, ns *v1.Namespace) v1.ObjectReference {
		return v1.ObjectReference{
			Name:      v1.ObjectName(name),
			Kind:      v1.Kind(kinds.ConfigMap),
			Namespace: ns,
		}
	}

	invalidKindRef := func(name string) v1.ObjectReference {
		return v1.ObjectReference{
			Name: v1.ObjectName(name),
			Kind: v1.Kind("Service"),
		}
	}

	tests := []struct {
		resolveErrByNN          map[string]error
		name                    string
		frontendTLS             v1.FrontendTLSConfig
		expectedCACertRefs      []*v1.ObjectReference
		expectedListenerReasons []string
		expectedGatewayReasons  []string
		listenerPort            v1.PortNumber
		expectedListenerValid   bool
	}{
		{
			name:         "Default Secret valid",
			listenerPort: v1.PortNumber(443),
			frontendTLS: v1.FrontendTLSConfig{
				Default: v1.TLSConfig{
					Validation: &v1.FrontendTLSValidation{
						Mode:              v1.AllowValidOnly,
						CACertificateRefs: []v1.ObjectReference{secretRef("default-secret", nil)},
					},
				},
			},
			expectedCACertRefs:    []*v1.ObjectReference{secretRefPtr("default-secret", namespace("default"))},
			expectedListenerValid: true,
		},
		{
			name:         "Per-port Secret valid",
			listenerPort: v1.PortNumber(443),
			frontendTLS: v1.FrontendTLSConfig{
				PerPort: []v1.TLSPortConfig{
					{
						Port: v1.PortNumber(443),
						TLS: v1.TLSConfig{
							Validation: &v1.FrontendTLSValidation{
								Mode:              v1.AllowValidOnly,
								CACertificateRefs: []v1.ObjectReference{secretRef("per-port-secret", nil)},
							},
						},
					},
				},
			},
			expectedListenerValid: true,
		},
		{
			name:         "Default Secret with AllowInsecureFallback",
			listenerPort: v1.PortNumber(443),
			frontendTLS: v1.FrontendTLSConfig{
				Default: v1.TLSConfig{
					Validation: &v1.FrontendTLSValidation{
						Mode:              v1.AllowInsecureFallback,
						CACertificateRefs: []v1.ObjectReference{secretRef("default-secret", nil)},
					},
				},
			},
			expectedGatewayReasons: []string{string(v1.GatewayReasonConfigurationChanged)},
			expectedListenerValid:  true,
		},
		{
			name:         "Per-port ConfigMap with AllowInsecureFallback",
			listenerPort: v1.PortNumber(8443),
			frontendTLS: v1.FrontendTLSConfig{
				Default: v1.TLSConfig{
					Validation: &v1.FrontendTLSValidation{
						Mode:              v1.AllowValidOnly,
						CACertificateRefs: []v1.ObjectReference{secretRef("default-secret", nil)},
					},
				},
				PerPort: []v1.TLSPortConfig{
					{
						Port: v1.PortNumber(8443),
						TLS: v1.TLSConfig{
							Validation: &v1.FrontendTLSValidation{
								Mode:              v1.AllowInsecureFallback,
								CACertificateRefs: []v1.ObjectReference{configMapRef("per-port-cm", nil)},
							},
						},
					},
				},
			},
			expectedGatewayReasons: []string{string(v1.GatewayReasonConfigurationChanged)},
			expectedListenerValid:  true,
		},
		{
			name:         "Default invalid kind",
			listenerPort: v1.PortNumber(443),
			frontendTLS: v1.FrontendTLSConfig{
				Default: v1.TLSConfig{
					Validation: &v1.FrontendTLSValidation{
						Mode:              v1.AllowValidOnly,
						CACertificateRefs: []v1.ObjectReference{invalidKindRef("bad-kind")},
					},
				},
			},
			expectedListenerReasons: []string{
				string(v1.ListenerReasonInvalidCACertificateKind),
				string(v1.ListenerReasonNoValidCACertificate),
			},
			expectedListenerValid: false,
		},
		{
			name:         "Per-port invalid kind",
			listenerPort: v1.PortNumber(443),
			frontendTLS: v1.FrontendTLSConfig{
				PerPort: []v1.TLSPortConfig{
					{
						Port: v1.PortNumber(443),
						TLS: v1.TLSConfig{
							Validation: &v1.FrontendTLSValidation{
								Mode:              v1.AllowValidOnly,
								CACertificateRefs: []v1.ObjectReference{invalidKindRef("bad-kind")},
							},
						},
					},
				},
			},
			expectedListenerReasons: []string{
				string(v1.ListenerReasonInvalidCACertificateKind),
				string(v1.ListenerReasonNoValidCACertificate),
			},
			expectedListenerValid: false,
		},
		{
			name:         "Default Secret without ca.crt key",
			listenerPort: v1.PortNumber(443),
			frontendTLS: v1.FrontendTLSConfig{
				Default: v1.TLSConfig{
					Validation: &v1.FrontendTLSValidation{
						Mode:              v1.AllowValidOnly,
						CACertificateRefs: []v1.ObjectReference{secretRef("default-secret-no-ca", nil)},
					},
				},
			},
			resolveErrByNN: map[string]error{"default/default-secret-no-ca": errors.New("missing key ca.crt")},
			expectedListenerReasons: []string{
				string(v1.ListenerReasonInvalidCACertificateRef),
				string(v1.ListenerReasonNoValidCACertificate),
			},
			expectedListenerValid: false,
		},
		{
			name:         "Per-port Secret without ca.crt key",
			listenerPort: v1.PortNumber(443),
			frontendTLS: v1.FrontendTLSConfig{
				PerPort: []v1.TLSPortConfig{
					{
						Port: v1.PortNumber(443),
						TLS: v1.TLSConfig{
							Validation: &v1.FrontendTLSValidation{
								Mode:              v1.AllowValidOnly,
								CACertificateRefs: []v1.ObjectReference{secretRef("per-port-secret-no-ca", nil)},
							},
						},
					},
				},
			},
			resolveErrByNN: map[string]error{"default/per-port-secret-no-ca": errors.New("missing key ca.crt")},
			expectedListenerReasons: []string{
				string(v1.ListenerReasonInvalidCACertificateRef),
				string(v1.ListenerReasonNoValidCACertificate),
			},
			expectedListenerValid: false,
		},
		{
			name:         "Default Secret without ca.crt key with Per-port valid resource",
			listenerPort: v1.PortNumber(8443),
			frontendTLS: v1.FrontendTLSConfig{
				Default: v1.TLSConfig{
					Validation: &v1.FrontendTLSValidation{
						Mode:              v1.AllowValidOnly,
						CACertificateRefs: []v1.ObjectReference{secretRef("default-secret-no-ca", nil)},
					},
				},
				PerPort: []v1.TLSPortConfig{
					{
						Port: v1.PortNumber(8443),
						TLS: v1.TLSConfig{
							Validation: &v1.FrontendTLSValidation{
								Mode:              v1.AllowValidOnly,
								CACertificateRefs: []v1.ObjectReference{secretRef("per-port-valid-secret", nil)},
							},
						},
					},
				},
			},
			resolveErrByNN:          map[string]error{"default/default-secret-no-ca": errors.New("missing key ca.crt")},
			expectedListenerReasons: nil,
			expectedListenerValid:   true,
		},
		{
			name:         "Per-port Secret without ca.crt key with Default valid resource",
			listenerPort: v1.PortNumber(8443),
			frontendTLS: v1.FrontendTLSConfig{
				Default: v1.TLSConfig{
					Validation: &v1.FrontendTLSValidation{
						Mode:              v1.AllowValidOnly,
						CACertificateRefs: []v1.ObjectReference{secretRef("default-valid-secret", nil)},
					},
				},
				PerPort: []v1.TLSPortConfig{
					{
						Port: v1.PortNumber(8443),
						TLS: v1.TLSConfig{
							Validation: &v1.FrontendTLSValidation{
								Mode:              v1.AllowValidOnly,
								CACertificateRefs: []v1.ObjectReference{secretRef("per-port-secret-no-ca", nil)},
							},
						},
					},
				},
			},
			resolveErrByNN: map[string]error{"default/per-port-secret-no-ca": errors.New("missing key ca.crt")},
			expectedListenerReasons: []string{
				string(v1.ListenerReasonInvalidCACertificateRef),
				string(v1.ListenerReasonNoValidCACertificate),
			},
			expectedListenerValid: false,
		},
		{
			name:         "Default ConfigMap without ca.crt key",
			listenerPort: v1.PortNumber(443),
			frontendTLS: v1.FrontendTLSConfig{
				Default: v1.TLSConfig{
					Validation: &v1.FrontendTLSValidation{
						Mode:              v1.AllowValidOnly,
						CACertificateRefs: []v1.ObjectReference{configMapRef("default-cm-no-ca", nil)},
					},
				},
			},
			resolveErrByNN: map[string]error{"default/default-cm-no-ca": errors.New("missing key ca.crt")},
			expectedListenerReasons: []string{
				string(v1.ListenerReasonInvalidCACertificateRef),
				string(v1.ListenerReasonNoValidCACertificate),
			},
			expectedListenerValid: false,
		},
		{
			name:         "Per-port ConfigMap without ca.crt key",
			listenerPort: v1.PortNumber(443),
			frontendTLS: v1.FrontendTLSConfig{
				PerPort: []v1.TLSPortConfig{
					{
						Port: v1.PortNumber(443),
						TLS: v1.TLSConfig{
							Validation: &v1.FrontendTLSValidation{
								Mode:              v1.AllowValidOnly,
								CACertificateRefs: []v1.ObjectReference{configMapRef("per-port-cm-no-ca", nil)},
							},
						},
					},
				},
			},
			resolveErrByNN: map[string]error{"default/per-port-cm-no-ca": errors.New("missing key ca.crt")},
			expectedListenerReasons: []string{
				string(v1.ListenerReasonInvalidCACertificateRef),
				string(v1.ListenerReasonNoValidCACertificate),
			},
			expectedListenerValid: false,
		},
		{
			name:         "Default ConfigMap without ca.crt key with Per-port valid resource",
			listenerPort: v1.PortNumber(8443),
			frontendTLS: v1.FrontendTLSConfig{
				Default: v1.TLSConfig{
					Validation: &v1.FrontendTLSValidation{
						Mode:              v1.AllowValidOnly,
						CACertificateRefs: []v1.ObjectReference{configMapRef("default-cm-no-ca", nil)},
					},
				},
				PerPort: []v1.TLSPortConfig{
					{
						Port: v1.PortNumber(8443),
						TLS: v1.TLSConfig{
							Validation: &v1.FrontendTLSValidation{
								Mode:              v1.AllowValidOnly,
								CACertificateRefs: []v1.ObjectReference{secretRef("per-port-valid-secret", nil)},
							},
						},
					},
				},
			},
			resolveErrByNN:          map[string]error{"default/default-cm-no-ca": errors.New("missing key ca.crt")},
			expectedListenerReasons: nil,
			expectedListenerValid:   true,
		},
		{
			name:         "Per-port ConfigMap without ca.crt key with Default valid resource",
			listenerPort: v1.PortNumber(8443),
			frontendTLS: v1.FrontendTLSConfig{
				Default: v1.TLSConfig{
					Validation: &v1.FrontendTLSValidation{
						Mode:              v1.AllowValidOnly,
						CACertificateRefs: []v1.ObjectReference{secretRef("default-valid-secret", nil)},
					},
				},
				PerPort: []v1.TLSPortConfig{
					{
						Port: v1.PortNumber(8443),
						TLS: v1.TLSConfig{
							Validation: &v1.FrontendTLSValidation{
								Mode:              v1.AllowValidOnly,
								CACertificateRefs: []v1.ObjectReference{configMapRef("per-port-cm-no-ca", nil)},
							},
						},
					},
				},
			},
			resolveErrByNN: map[string]error{"default/per-port-cm-no-ca": errors.New("missing key ca.crt")},
			expectedListenerReasons: []string{
				string(v1.ListenerReasonInvalidCACertificateRef),
				string(v1.ListenerReasonNoValidCACertificate),
			},
			expectedListenerValid: false,
		},
		{
			name:         "Default with valid secret. Per-port with missing Secret and valid ConfigMap",
			listenerPort: v1.PortNumber(8443),
			frontendTLS: v1.FrontendTLSConfig{
				Default: v1.TLSConfig{
					Validation: &v1.FrontendTLSValidation{
						Mode:              v1.AllowValidOnly,
						CACertificateRefs: []v1.ObjectReference{secretRef("default-secret", nil)},
					},
				},
				PerPort: []v1.TLSPortConfig{
					{
						Port: v1.PortNumber(8443),
						TLS: v1.TLSConfig{
							Validation: &v1.FrontendTLSValidation{
								Mode: v1.AllowValidOnly,
								CACertificateRefs: []v1.ObjectReference{
									secretRef("missing-secret", nil),
									configMapRef("good-cm", nil),
								},
							},
						},
					},
				},
			},
			resolveErrByNN: map[string]error{"default/missing-secret": errors.New("not found")},
			expectedListenerReasons: []string{
				string(v1.ListenerReasonInvalidCACertificateRef),
			},
			expectedListenerValid: true,
		},
		{
			name:         "Per-port precedence: valid Per-port refs ignore Default missing Secret",
			listenerPort: v1.PortNumber(8443),
			frontendTLS: v1.FrontendTLSConfig{
				Default: v1.TLSConfig{
					Validation: &v1.FrontendTLSValidation{
						Mode: v1.AllowValidOnly,
						CACertificateRefs: []v1.ObjectReference{
							secretRef("missing-secret", nil),
							configMapRef("good-cm", nil),
						},
					},
				},
				PerPort: []v1.TLSPortConfig{
					{
						Port: v1.PortNumber(8443),
						TLS: v1.TLSConfig{
							Validation: &v1.FrontendTLSValidation{
								Mode: v1.AllowValidOnly,
								CACertificateRefs: []v1.ObjectReference{
									secretRef("default-secret", nil),
								},
							},
						},
					},
				},
			},
			resolveErrByNN:          map[string]error{"default/missing-secret": errors.New("not found")},
			expectedListenerReasons: nil,
			expectedListenerValid:   true,
		},
		{
			name:         "Default cross-namespace Secret without ReferenceGrant",
			listenerPort: v1.PortNumber(443),
			frontendTLS: v1.FrontendTLSConfig{
				Default: v1.TLSConfig{
					Validation: &v1.FrontendTLSValidation{
						Mode:              v1.AllowValidOnly,
						CACertificateRefs: []v1.ObjectReference{secretRef("xns-secret", namespace("other"))},
					},
				},
			},
			expectedListenerReasons: []string{
				string(v1.ListenerReasonRefNotPermitted),
				string(v1.ListenerReasonNoValidCACertificate),
			},
			expectedGatewayReasons: []string{string(v1.GatewayReasonRefNotPermitted)},
			expectedListenerValid:  false,
		},
		{
			name:         "Per-port cross-namespace Secret without ReferenceGrant",
			listenerPort: v1.PortNumber(443),
			frontendTLS: v1.FrontendTLSConfig{
				PerPort: []v1.TLSPortConfig{
					{
						Port: v1.PortNumber(443),
						TLS: v1.TLSConfig{
							Validation: &v1.FrontendTLSValidation{
								Mode:              v1.AllowValidOnly,
								CACertificateRefs: []v1.ObjectReference{secretRef("xns-secret", namespace("other"))},
							},
						},
					},
				},
			},
			expectedListenerReasons: []string{
				string(v1.ListenerReasonRefNotPermitted),
				string(v1.ListenerReasonNoValidCACertificate),
			},
			expectedGatewayReasons: []string{string(v1.GatewayReasonRefNotPermitted)},
			expectedListenerValid:  false,
		},
		{
			name:         "Per-port precedence: valid Per-port secret ignores Default cross-namespace ConfigMap",
			listenerPort: v1.PortNumber(8443),
			frontendTLS: v1.FrontendTLSConfig{
				Default: v1.TLSConfig{
					Validation: &v1.FrontendTLSValidation{
						Mode: v1.AllowValidOnly,
						CACertificateRefs: []v1.ObjectReference{
							secretRef("local-secret", nil),
							configMapRef("xns-cm", namespace("other")),
						},
					},
				},
				PerPort: []v1.TLSPortConfig{
					{
						Port: v1.PortNumber(8443),
						TLS: v1.TLSConfig{
							Validation: &v1.FrontendTLSValidation{
								Mode: v1.AllowValidOnly,
								CACertificateRefs: []v1.ObjectReference{
									secretRef("default-secret", nil),
								},
							},
						},
					},
				},
			},
			expectedListenerReasons: nil,
			expectedListenerValid:   true,
		},
		{
			name:         "Per-port Secret in same ns + cross-namespace ConfigMap without ReferenceGrant",
			listenerPort: v1.PortNumber(8443),
			frontendTLS: v1.FrontendTLSConfig{
				Default: v1.TLSConfig{
					Validation: &v1.FrontendTLSValidation{
						Mode:              v1.AllowValidOnly,
						CACertificateRefs: []v1.ObjectReference{secretRef("default-secret", nil)},
					},
				},
				PerPort: []v1.TLSPortConfig{
					{
						Port: v1.PortNumber(8443),
						TLS: v1.TLSConfig{
							Validation: &v1.FrontendTLSValidation{
								Mode: v1.AllowValidOnly,
								CACertificateRefs: []v1.ObjectReference{
									secretRef("local-secret", nil),
									configMapRef("xns-cm", namespace("other")),
								},
							},
						},
					},
				},
			},
			expectedListenerReasons: []string{string(v1.ListenerReasonRefNotPermitted)},
			expectedGatewayReasons:  []string{string(v1.GatewayReasonRefNotPermitted)},
			expectedListenerValid:   true,
		},
		{
			name:         "Default invalid kind with per-port valid",
			listenerPort: v1.PortNumber(8443),
			frontendTLS: v1.FrontendTLSConfig{
				Default: v1.TLSConfig{
					Validation: &v1.FrontendTLSValidation{
						Mode:              v1.AllowValidOnly,
						CACertificateRefs: []v1.ObjectReference{secretRef("default-secret", nil)},
					},
				},
				PerPort: []v1.TLSPortConfig{
					{
						Port: v1.PortNumber(8443),
						TLS: v1.TLSConfig{
							Validation: &v1.FrontendTLSValidation{
								Mode:              v1.AllowValidOnly,
								CACertificateRefs: []v1.ObjectReference{invalidKindRef("bad-kind")},
							},
						},
					},
				},
			},
			expectedListenerReasons: []string{
				string(v1.ListenerReasonInvalidCACertificateKind),
				string(v1.ListenerReasonNoValidCACertificate),
			},
			expectedListenerValid: false,
		},
		{
			name:         "Per-port invalid kind with Default valid",
			listenerPort: v1.PortNumber(8443),
			frontendTLS: v1.FrontendTLSConfig{
				Default: v1.TLSConfig{
					Validation: &v1.FrontendTLSValidation{
						Mode:              v1.AllowValidOnly,
						CACertificateRefs: []v1.ObjectReference{secretRef("default-secret", nil)},
					},
				},
				PerPort: []v1.TLSPortConfig{
					{
						Port: v1.PortNumber(8443),
						TLS: v1.TLSConfig{
							Validation: &v1.FrontendTLSValidation{
								Mode:              v1.AllowValidOnly,
								CACertificateRefs: []v1.ObjectReference{invalidKindRef("bad-kind")},
							},
						},
					},
				},
			},
			expectedListenerReasons: []string{
				string(v1.ListenerReasonInvalidCACertificateKind),
				string(v1.ListenerReasonNoValidCACertificate),
			},
			expectedListenerValid: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			gw := &Gateway{
				Source: &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gw",
						Namespace: "default",
					},
					Spec: v1.GatewaySpec{
						TLS: &v1.GatewayTLSConfig{
							Frontend: &test.frontendTLS,
						},
					},
				},
			}

			listener := &Listener{
				Name: fmt.Sprintf("https-%d", test.listenerPort),
				Source: v1.Listener{
					Name: v1.SectionName(fmt.Sprintf("https-%d", test.listenerPort)),
					Port: test.listenerPort,
					TLS: &v1.ListenerTLSConfig{
						Mode: helpers.GetPointer(v1.TLSModeTerminate),
					},
				},
				Valid:      true,
				Conditions: []conditions.Condition{},
			}

			fakeResolver := &resolverfakes.FakeResolver{}
			if test.resolveErrByNN != nil {
				fakeResolver.ResolveCalls(func(
					_ resolver.ResourceType,
					nsName types.NamespacedName,
					_ ...resolver.ResolveOption,
				) error {
					if err, exists := test.resolveErrByNN[nsName.String()]; exists {
						return err
					}
					return nil
				})
			}

			resolverFunc := createFrontendTLSCaCertReferenceResolver(fakeResolver, newReferenceGrantResolver(nil))
			resolverFunc(listener, gw)

			listenerReasons := make([]string, 0, len(listener.Conditions))
			for _, cond := range listener.Conditions {
				listenerReasons = append(listenerReasons, cond.Reason)
			}

			gatewayReasons := make([]string, 0, len(gw.Conditions))
			for _, cond := range gw.Conditions {
				gatewayReasons = append(gatewayReasons, cond.Reason)
			}

			for _, expectedReason := range test.expectedListenerReasons {
				g.Expect(listenerReasons).To(ContainElement(expectedReason))
			}

			for _, expectedReason := range test.expectedGatewayReasons {
				g.Expect(gatewayReasons).To(ContainElement(expectedReason))
			}

			if len(test.expectedListenerReasons) == 0 {
				g.Expect(listener.Conditions).To(BeEmpty())
			}

			if len(test.expectedGatewayReasons) == 0 {
				g.Expect(gw.Conditions).To(BeEmpty())
			}

			g.Expect(listener.Valid).To(Equal(test.expectedListenerValid))
			if test.expectedCACertRefs != nil {
				g.Expect(listener.CACertificateRefs).To(Equal(test.expectedCACertRefs))
			}
		})
	}
}

func TestGetFrontendTLSCertReferences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		ref              v1.ObjectReference
		expectedObjRefNS *v1.Namespace
		expectedNsName   types.NamespacedName
	}{
		{
			name:             "defaults empty namespace to gateway namespace",
			ref:              v1.ObjectReference{Name: v1.ObjectName("ca-secret"), Kind: v1.Kind(kinds.Secret)},
			expectedObjRefNS: helpers.GetPointer(v1.Namespace("gateway-ns")),
			expectedNsName:   types.NamespacedName{Namespace: "gateway-ns", Name: "ca-secret"},
		},
		{
			name: "preserves explicit namespace",
			ref: v1.ObjectReference{
				Name:      v1.ObjectName("ca-secret"),
				Kind:      v1.Kind(kinds.Secret),
				Namespace: helpers.GetPointer(v1.Namespace("other-ns")),
			},
			expectedObjRefNS: helpers.GetPointer(v1.Namespace("other-ns")),
			expectedNsName:   types.NamespacedName{Namespace: "other-ns", Name: "ca-secret"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			objRef, nsName := getFrontendTLSCertReferences(test.ref, &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Namespace: "gateway-ns"},
			})

			g.Expect(objRef).NotTo(BeNil())
			g.Expect(objRef.Namespace).To(Equal(test.expectedObjRefNS))
			g.Expect(nsName).To(Equal(&test.expectedNsName))
		})
	}
}

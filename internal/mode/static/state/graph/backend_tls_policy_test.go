package graph

import (
	"testing"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
	"sigs.k8s.io/gateway-api/apis/v1alpha3"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
)

func TestProcessBackendTLSPoliciesEmpty(t *testing.T) {
	t.Parallel()
	backendTLSPolicies := map[types.NamespacedName]*v1alpha3.BackendTLSPolicy{
		{Namespace: "test", Name: "tls-policy"}: {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tls-policy",
				Namespace: "test",
			},
			Spec: v1alpha3.BackendTLSPolicySpec{
				TargetRefs: []v1alpha2.LocalPolicyTargetReferenceWithSectionName{
					{
						LocalPolicyTargetReference: v1alpha2.LocalPolicyTargetReference{
							Kind: "Service",
							Name: "service1",
						},
					},
				},
				Validation: v1alpha3.BackendTLSPolicyValidation{
					CACertificateRefs: []gatewayv1.LocalObjectReference{
						{
							Kind:  "ConfigMap",
							Name:  "configmap",
							Group: "",
						},
					},
					Hostname: "foo.test.com",
				},
			},
		},
	}

	gateway := &Gateway{
		Source: &gatewayv1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "gateway", Namespace: "test"}},
	}

	tests := []struct {
		expected           map[types.NamespacedName]*BackendTLSPolicy
		gateway            *Gateway
		backendTLSPolicies map[types.NamespacedName]*v1alpha3.BackendTLSPolicy
		name               string
	}{
		{
			name:               "no policies",
			expected:           nil,
			gateway:            gateway,
			backendTLSPolicies: nil,
		},
		{
			name:               "nil gateway",
			expected:           nil,
			backendTLSPolicies: backendTLSPolicies,
			gateway:            nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			processed := processBackendTLSPolicies(test.backendTLSPolicies, nil, nil, "test", test.gateway)

			g.Expect(processed).To(Equal(test.expected))
		})
	}
}

func TestValidateBackendTLSPolicy(t *testing.T) {
	targetRefNormalCase := []v1alpha2.LocalPolicyTargetReferenceWithSectionName{
		{
			LocalPolicyTargetReference: v1alpha2.LocalPolicyTargetReference{
				Kind: "Service",
				Name: "service1",
			},
		},
	}

	localObjectRefNormalCase := []gatewayv1.LocalObjectReference{
		{
			Kind:  "ConfigMap",
			Name:  "configmap",
			Group: "",
		},
	}

	localObjectRefInvalidName := []gatewayv1.LocalObjectReference{
		{
			Kind:  "ConfigMap",
			Name:  "invalid",
			Group: "",
		},
	}

	localObjectRefInvalidKind := []gatewayv1.LocalObjectReference{
		{
			Kind:  "Secret",
			Name:  "secret",
			Group: "",
		},
	}

	localObjectRefInvalidGroup := []gatewayv1.LocalObjectReference{
		{
			Kind:  "ConfigMap",
			Name:  "configmap",
			Group: "bhu",
		},
	}

	localObjectRefTooManyCerts := []gatewayv1.LocalObjectReference{
		{
			Kind:  "ConfigMap",
			Name:  "configmap",
			Group: "",
		},
		{
			Kind:  "ConfigMap",
			Name:  "invalid",
			Group: "",
		},
	}

	getAncestorRef := func(ctlrName, parentName string) v1alpha2.PolicyAncestorStatus {
		return v1alpha2.PolicyAncestorStatus{
			ControllerName: gatewayv1.GatewayController(ctlrName),
			AncestorRef: gatewayv1.ParentReference{
				Name:      gatewayv1.ObjectName(parentName),
				Namespace: helpers.GetPointer(gatewayv1.Namespace("test")),
				Group:     helpers.GetPointer[gatewayv1.Group](gatewayv1.GroupName),
				Kind:      helpers.GetPointer[gatewayv1.Kind](kinds.Gateway),
			},
		}
	}

	ancestors := []v1alpha2.PolicyAncestorStatus{
		getAncestorRef("not-us", "not-us"),
		getAncestorRef("not-us", "not-us"),
		getAncestorRef("not-us", "not-us"),
		getAncestorRef("not-us", "not-us"),
		getAncestorRef("not-us", "not-us"),
		getAncestorRef("not-us", "not-us"),
		getAncestorRef("not-us", "not-us"),
		getAncestorRef("not-us", "not-us"),
		getAncestorRef("not-us", "not-us"),
		getAncestorRef("not-us", "not-us"),
		getAncestorRef("not-us", "not-us"),
		getAncestorRef("not-us", "not-us"),
		getAncestorRef("not-us", "not-us"),
		getAncestorRef("not-us", "not-us"),
		getAncestorRef("not-us", "not-us"),
		getAncestorRef("not-us", "not-us"),
	}

	ancestorsWithUs := make([]v1alpha2.PolicyAncestorStatus, len(ancestors))
	copy(ancestorsWithUs, ancestors)
	ancestorsWithUs[0] = getAncestorRef("test", "gateway")

	tests := []struct {
		tlsPolicy *v1alpha3.BackendTLSPolicy
		gateway   *Gateway
		name      string
		isValid   bool
		ignored   bool
	}{
		{
			name: "normal case with ca cert refs",
			tlsPolicy: &v1alpha3.BackendTLSPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-policy",
					Namespace: "test",
				},
				Spec: v1alpha3.BackendTLSPolicySpec{
					TargetRefs: targetRefNormalCase,
					Validation: v1alpha3.BackendTLSPolicyValidation{
						CACertificateRefs: localObjectRefNormalCase,
						Hostname:          "foo.test.com",
					},
				},
			},
			isValid: true,
		},
		{
			name: "normal case with ca cert refs and 16 ancestors including us",
			tlsPolicy: &v1alpha3.BackendTLSPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-policy",
					Namespace: "test",
				},
				Spec: v1alpha3.BackendTLSPolicySpec{
					TargetRefs: targetRefNormalCase,
					Validation: v1alpha3.BackendTLSPolicyValidation{
						CACertificateRefs: localObjectRefNormalCase,
						Hostname:          "foo.test.com",
					},
				},
				Status: v1alpha2.PolicyStatus{
					Ancestors: ancestorsWithUs,
				},
			},
			isValid: true,
		},
		{
			name: "normal case with well known certs",
			tlsPolicy: &v1alpha3.BackendTLSPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-policy",
					Namespace: "test",
				},
				Spec: v1alpha3.BackendTLSPolicySpec{
					TargetRefs: targetRefNormalCase,
					Validation: v1alpha3.BackendTLSPolicyValidation{
						WellKnownCACertificates: (helpers.GetPointer(v1alpha3.WellKnownCACertificatesSystem)),
						Hostname:                "foo.test.com",
					},
				},
			},
			isValid: true,
		},
		{
			name: "no hostname invalid case",
			tlsPolicy: &v1alpha3.BackendTLSPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-policy",
					Namespace: "test",
				},
				Spec: v1alpha3.BackendTLSPolicySpec{
					TargetRefs: targetRefNormalCase,
					Validation: v1alpha3.BackendTLSPolicyValidation{
						CACertificateRefs: localObjectRefNormalCase,
						Hostname:          "",
					},
				},
			},
		},
		{
			name: "invalid ca cert ref name",
			tlsPolicy: &v1alpha3.BackendTLSPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-policy",
					Namespace: "test",
				},
				Spec: v1alpha3.BackendTLSPolicySpec{
					TargetRefs: targetRefNormalCase,
					Validation: v1alpha3.BackendTLSPolicyValidation{
						CACertificateRefs: localObjectRefInvalidName,
						Hostname:          "foo.test.com",
					},
				},
			},
		},
		{
			name: "invalid ca cert ref kind",
			tlsPolicy: &v1alpha3.BackendTLSPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-policy",
					Namespace: "test",
				},
				Spec: v1alpha3.BackendTLSPolicySpec{
					TargetRefs: targetRefNormalCase,
					Validation: v1alpha3.BackendTLSPolicyValidation{
						CACertificateRefs: localObjectRefInvalidKind,
						Hostname:          "foo.test.com",
					},
				},
			},
		},
		{
			name: "invalid ca cert ref group",
			tlsPolicy: &v1alpha3.BackendTLSPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-policy",
					Namespace: "test",
				},
				Spec: v1alpha3.BackendTLSPolicySpec{
					TargetRefs: targetRefNormalCase,
					Validation: v1alpha3.BackendTLSPolicyValidation{
						CACertificateRefs: localObjectRefInvalidGroup,
						Hostname:          "foo.test.com",
					},
				},
			},
		},
		{
			name: "invalid case with well known certs",
			tlsPolicy: &v1alpha3.BackendTLSPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-policy",
					Namespace: "test",
				},
				Spec: v1alpha3.BackendTLSPolicySpec{
					TargetRefs: targetRefNormalCase,
					Validation: v1alpha3.BackendTLSPolicyValidation{
						WellKnownCACertificates: (helpers.GetPointer(v1alpha3.WellKnownCACertificatesType("unknown"))),
						Hostname:                "foo.test.com",
					},
				},
			},
		},
		{
			name: "invalid case neither TLS config option chosen",
			tlsPolicy: &v1alpha3.BackendTLSPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-policy",
					Namespace: "test",
				},
				Spec: v1alpha3.BackendTLSPolicySpec{
					TargetRefs: targetRefNormalCase,
					Validation: v1alpha3.BackendTLSPolicyValidation{
						Hostname: "foo.test.com",
					},
				},
			},
		},
		{
			name: "invalid case with too many ca cert refs",
			tlsPolicy: &v1alpha3.BackendTLSPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-policy",
					Namespace: "test",
				},
				Spec: v1alpha3.BackendTLSPolicySpec{
					TargetRefs: targetRefNormalCase,
					Validation: v1alpha3.BackendTLSPolicyValidation{
						CACertificateRefs: localObjectRefTooManyCerts,
						Hostname:          "foo.test.com",
					},
				},
			},
		},
		{
			name: "invalid case with too both ca cert refs and wellknowncerts",
			tlsPolicy: &v1alpha3.BackendTLSPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-policy",
					Namespace: "test",
				},
				Spec: v1alpha3.BackendTLSPolicySpec{
					TargetRefs: targetRefNormalCase,
					Validation: v1alpha3.BackendTLSPolicyValidation{
						CACertificateRefs:       localObjectRefNormalCase,
						Hostname:                "foo.test.com",
						WellKnownCACertificates: (helpers.GetPointer(v1alpha3.WellKnownCACertificatesSystem)),
					},
				},
			},
		},
		{
			name: "invalid case with too many ancestors",
			tlsPolicy: &v1alpha3.BackendTLSPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-policy",
					Namespace: "test",
				},
				Spec: v1alpha3.BackendTLSPolicySpec{
					TargetRefs: targetRefNormalCase,
					Validation: v1alpha3.BackendTLSPolicyValidation{
						CACertificateRefs: localObjectRefNormalCase,
						Hostname:          "foo.test.com",
					},
				},
				Status: v1alpha2.PolicyStatus{
					Ancestors: ancestors,
				},
			},
			ignored: true,
		},
	}

	configMaps := map[types.NamespacedName]*v1.ConfigMap{
		{Namespace: "test", Name: "configmap"}: {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "configmap",
				Namespace: "test",
			},
			Data: map[string]string{
				"ca.crt": caBlock,
			},
		},
		{Namespace: "test", Name: "invalid"}: {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "invalid",
				Namespace: "test",
			},
			Data: map[string]string{
				"ca.crt": "invalid",
			},
		},
	}

	secretMaps := map[types.NamespacedName]*v1.Secret{
		{Namespace: "test", Name: "test-secret"}: {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret",
				Namespace: "test",
			},
			// FILL IN
		},
		{Namespace: "test", Name: "invalid-secret"}: {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "invalid-secret",
				Namespace: "test",
			},
		},
		// FILL IN
	}

	configMapResolver := newConfigMapResolver(configMaps)
	secretMapResolver := newSecretResolver(secretMaps)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)

			valid, ignored, conds := validateBackendTLSPolicy(test.tlsPolicy, configMapResolver, secretMapResolver, "test")

			g.Expect(valid).To(Equal(test.isValid))
			g.Expect(ignored).To(Equal(test.ignored))
			if !test.isValid && !test.ignored {
				g.Expect(conds).To(HaveLen(1))
			} else {
				g.Expect(conds).To(BeEmpty())
			}
		})
	}
}

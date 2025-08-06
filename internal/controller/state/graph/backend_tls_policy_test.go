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

	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
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

	gateway := map[types.NamespacedName]*Gateway{
		{Namespace: "test", Name: "gateway"}: {
			Source: &gatewayv1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "gateway", Namespace: "test"}},
		},
	}

	tests := []struct {
		expected           map[types.NamespacedName]*BackendTLSPolicy
		gateways           map[types.NamespacedName]*Gateway
		backendTLSPolicies map[types.NamespacedName]*v1alpha3.BackendTLSPolicy
		name               string
	}{
		{
			name:               "no policies",
			expected:           nil,
			gateways:           gateway,
			backendTLSPolicies: nil,
		},
		{
			name:               "nil gateway",
			expected:           nil,
			backendTLSPolicies: backendTLSPolicies,
			gateways:           nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			processed := processBackendTLSPolicies(test.backendTLSPolicies, nil, nil, "test", test.gateways)

			g.Expect(processed).To(Equal(test.expected))
		})
	}
}

func TestValidateBackendTLSPolicy(t *testing.T) {
	const testSecretName string = "test-secret"
	targetRefNormalCase := []v1alpha2.LocalPolicyTargetReferenceWithSectionName{
		{
			LocalPolicyTargetReference: v1alpha2.LocalPolicyTargetReference{
				Kind: "Service",
				Name: "service1",
			},
		},
	}

	targetRefInvalidKind := []v1alpha2.LocalPolicyTargetReferenceWithSectionName{
		{
			LocalPolicyTargetReference: v1alpha2.LocalPolicyTargetReference{
				Kind: "Invalid",
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

	localObjectRefSecretNormalCase := []gatewayv1.LocalObjectReference{
		{
			Kind:  "Secret",
			Name:  gatewayv1.ObjectName(testSecretName),
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
			Kind:  "Invalid",
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
			name: "normal case with ca cert ref secrets",
			tlsPolicy: &v1alpha3.BackendTLSPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-policy",
					Namespace: "test",
				},
				Spec: v1alpha3.BackendTLSPolicySpec{
					TargetRefs: targetRefNormalCase,
					Validation: v1alpha3.BackendTLSPolicyValidation{
						CACertificateRefs: localObjectRefSecretNormalCase,
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
					TargetRefs: targetRefInvalidKind,
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
			name: "valid case with many ancestors (ancestor limit now handled during gateway assignment)",
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
			isValid: true,
		},
	}

	configMaps := map[types.NamespacedName]*v1.ConfigMap{
		{Namespace: "test", Name: "configmap"}: {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "configmap",
				Namespace: "test",
			},
			Data: map[string]string{
				CAKey: caBlock,
			},
		},
		{Namespace: "test", Name: "invalid"}: {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "invalid",
				Namespace: "test",
			},
			Data: map[string]string{
				CAKey: "invalid",
			},
		},
	}

	secretMaps := map[types.NamespacedName]*v1.Secret{
		{Namespace: "test", Name: testSecretName}: {
			ObjectMeta: metav1.ObjectMeta{
				Name:      testSecretName,
				Namespace: "test",
			},
			Type: v1.SecretTypeTLS,
			Data: map[string][]byte{
				v1.TLSCertKey:       cert,
				v1.TLSPrivateKeyKey: key,
				CAKey:               []byte(caBlock),
			},
		},
		{Namespace: "test", Name: "invalid-secret"}: {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "invalid-secret",
				Namespace: "test",
			},
			Data: map[string][]byte{
				v1.TLSCertKey:       invalidCert,
				v1.TLSPrivateKeyKey: invalidKey,
				CAKey:               []byte("invalid-cert"),
			},
		},
	}

	configMapResolver := newConfigMapResolver(configMaps)
	secretMapResolver := newSecretResolver(secretMaps)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)

			valid, ignored, conds := validateBackendTLSPolicy(test.tlsPolicy, configMapResolver, secretMapResolver, "test")

			if !test.isValid && !test.ignored {
				g.Expect(conds).To(HaveLen(1))
			} else {
				g.Expect(conds).To(BeEmpty())
			}
			g.Expect(valid).To(Equal(test.isValid))
			g.Expect(ignored).To(Equal(test.ignored))
		})
	}
}

func TestAddGatewaysForBackendTLSPolicies(t *testing.T) {
	t.Parallel()

	btp1 := &BackendTLSPolicy{
		Source: &v1alpha3.BackendTLSPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "btp1",
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
					{
						LocalPolicyTargetReference: v1alpha2.LocalPolicyTargetReference{
							Kind: "Service",
							Name: "service2",
						},
					},
				},
			},
		},
	}
	btp1Expected := btp1

	btp1Expected.Gateways = []types.NamespacedName{
		{Namespace: "test", Name: "gateway1"},
		{Namespace: "test", Name: "gateway2"},
		{Namespace: "test", Name: "gateway3"},
	}

	btp2 := &BackendTLSPolicy{
		Source: &v1alpha3.BackendTLSPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "btp2",
				Namespace: "test",
			},
			Spec: v1alpha3.BackendTLSPolicySpec{
				TargetRefs: []v1alpha2.LocalPolicyTargetReferenceWithSectionName{
					{
						LocalPolicyTargetReference: v1alpha2.LocalPolicyTargetReference{
							Kind: "Service",
							Name: "service3",
						},
					},
					{
						LocalPolicyTargetReference: v1alpha2.LocalPolicyTargetReference{
							Kind: "Service",
							Name: "service4",
						},
					},
				},
			},
		},
	}

	btp2Expected := btp2
	btp2Expected.Gateways = []types.NamespacedName{
		{Namespace: "test", Name: "gateway4"},
	}

	btp3 := &BackendTLSPolicy{
		Source: &v1alpha3.BackendTLSPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "btp3",
				Namespace: "test",
			},
			Spec: v1alpha3.BackendTLSPolicySpec{
				TargetRefs: []v1alpha2.LocalPolicyTargetReferenceWithSectionName{
					{
						LocalPolicyTargetReference: v1alpha2.LocalPolicyTargetReference{
							Kind: "Service",
							Name: "service-does-not-exist",
						},
					},
				},
			},
		},
	}

	btp4 := &BackendTLSPolicy{
		Source: &v1alpha3.BackendTLSPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "btp4",
				Namespace: "test",
			},
			Spec: v1alpha3.BackendTLSPolicySpec{
				TargetRefs: []v1alpha2.LocalPolicyTargetReferenceWithSectionName{
					{
						LocalPolicyTargetReference: v1alpha2.LocalPolicyTargetReference{
							Kind: "Gateway",
							Name: "gateway",
						},
					},
				},
			},
		},
	}

	tests := []struct {
		backendTLSPolicies map[types.NamespacedName]*BackendTLSPolicy
		services           map[types.NamespacedName]*ReferencedService
		expected           map[types.NamespacedName]*BackendTLSPolicy
		name               string
	}{
		{
			name: "add multiple gateways to backend tls policies",
			backendTLSPolicies: map[types.NamespacedName]*BackendTLSPolicy{
				{Namespace: "test", Name: "btp1"}: btp1,
				{Namespace: "test", Name: "btp2"}: btp2,
			},
			services: map[types.NamespacedName]*ReferencedService{
				{Namespace: "test", Name: "service1"}: {
					GatewayNsNames: map[types.NamespacedName]struct{}{
						{Namespace: "test", Name: "gateway1"}: {},
					},
				},
				{Namespace: "test", Name: "service2"}: {
					GatewayNsNames: map[types.NamespacedName]struct{}{
						{Namespace: "test", Name: "gateway2"}: {},
						{Namespace: "test", Name: "gateway3"}: {},
					},
				},
				{Namespace: "test", Name: "service3"}: {
					GatewayNsNames: map[types.NamespacedName]struct{}{
						{Namespace: "test", Name: "gateway4"}: {},
					},
				},
				{Namespace: "test", Name: "service4"}: {
					GatewayNsNames: map[types.NamespacedName]struct{}{
						{Namespace: "test", Name: "gateway4"}: {},
					},
				},
			},
			expected: map[types.NamespacedName]*BackendTLSPolicy{
				{Namespace: "test", Name: "btp1"}: btp1Expected,
				{Namespace: "test", Name: "btp2"}: btp2Expected,
			},
		},
		{
			name: "backend tls policy with a service target ref that does not reference a gateway",
			backendTLSPolicies: map[types.NamespacedName]*BackendTLSPolicy{
				{Namespace: "test", Name: "btp3"}: btp3,
			},
			services: map[types.NamespacedName]*ReferencedService{
				{Namespace: "test", Name: "service1"}: {
					GatewayNsNames: map[types.NamespacedName]struct{}{},
				},
			},
			expected: map[types.NamespacedName]*BackendTLSPolicy{
				{Namespace: "test", Name: "btp3"}: btp3,
			},
		},
		{
			name: "backend tls policy that does not reference a service",
			backendTLSPolicies: map[types.NamespacedName]*BackendTLSPolicy{
				{Namespace: "test", Name: "btp4"}: btp4,
			},
			services: map[types.NamespacedName]*ReferencedService{},
			expected: map[types.NamespacedName]*BackendTLSPolicy{
				{Namespace: "test", Name: "btp4"}: btp4,
			},
		},
	}

	for _, test := range tests {
		g := NewWithT(t)
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			addGatewaysForBackendTLSPolicies(test.backendTLSPolicies, test.services, "nginx-gateway")
			g.Expect(helpers.Diff(test.backendTLSPolicies, test.expected)).To(BeEmpty())
		})
	}
}

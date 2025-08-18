package graph

import (
	"bytes"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
	"sigs.k8s.io/gateway-api/apis/v1alpha3"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
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

			processed := processBackendTLSPolicies(test.backendTLSPolicies, nil, nil, test.gateways)

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
			name: "valid case with many ancestors",
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

			valid, ignored, conds := validateBackendTLSPolicy(test.tlsPolicy, configMapResolver, secretMapResolver)

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
			addGatewaysForBackendTLSPolicies(test.backendTLSPolicies, test.services, "nginx-gateway", nil, logr.Discard())
			g.Expect(helpers.Diff(test.backendTLSPolicies, test.expected)).To(BeEmpty())
		})
	}
}

func TestAddGatewaysForBackendTLSPoliciesAncestorLimit(t *testing.T) {
	t.Parallel()

	// Create a test logger that captures log output
	var logBuf bytes.Buffer
	testLogger := logr.New(&testNGFLogSink{buffer: &logBuf})

	// Helper function to create ancestor references
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

	// Create 16 ancestors from different controllers to simulate full list
	fullAncestors := make([]v1alpha2.PolicyAncestorStatus, 16)
	for i := range 16 {
		fullAncestors[i] = getAncestorRef("other-controller", "other-gateway")
	}

	btpWithFullAncestors := &BackendTLSPolicy{
		Source: &v1alpha3.BackendTLSPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "btp-full-ancestors",
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
			},
			Status: v1alpha2.PolicyStatus{
				Ancestors: fullAncestors,
			},
		},
	}

	btpNormal := &BackendTLSPolicy{
		Source: &v1alpha3.BackendTLSPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "btp-normal",
				Namespace: "test",
			},
			Spec: v1alpha3.BackendTLSPolicySpec{
				TargetRefs: []v1alpha2.LocalPolicyTargetReferenceWithSectionName{
					{
						LocalPolicyTargetReference: v1alpha2.LocalPolicyTargetReference{
							Kind: "Service",
							Name: "service2",
						},
					},
				},
			},
			Status: v1alpha2.PolicyStatus{
				Ancestors: []v1alpha2.PolicyAncestorStatus{}, // Empty ancestors list
			},
		},
	}

	services := map[types.NamespacedName]*ReferencedService{
		{Namespace: "test", Name: "service1"}: {
			GatewayNsNames: map[types.NamespacedName]struct{}{
				{Namespace: "test", Name: "gateway1"}: {},
			},
		},
		{Namespace: "test", Name: "service2"}: {
			GatewayNsNames: map[types.NamespacedName]struct{}{
				{Namespace: "test", Name: "gateway2"}: {},
			},
		},
	}

	// Create gateways - one will receive ancestor limit condition
	gateways := map[types.NamespacedName]*Gateway{
		{Namespace: "test", Name: "gateway1"}: {
			Source: &gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Name: "gateway1", Namespace: "test"},
			},
			Conditions: []conditions.Condition{}, // Start with empty conditions
		},
		{Namespace: "test", Name: "gateway2"}: {
			Source: &gatewayv1.Gateway{
				ObjectMeta: metav1.ObjectMeta{Name: "gateway2", Namespace: "test"},
			},
			Conditions: []conditions.Condition{}, // Start with empty conditions
		},
	}

	backendTLSPolicies := map[types.NamespacedName]*BackendTLSPolicy{
		{Namespace: "test", Name: "btp-full-ancestors"}: btpWithFullAncestors,
		{Namespace: "test", Name: "btp-normal"}:         btpNormal,
	}

	g := NewWithT(t)

	// Execute the function
	addGatewaysForBackendTLSPolicies(backendTLSPolicies, services, "nginx-gateway", gateways, testLogger)

	// Verify that the policy with full ancestors doesn't get any gateways assigned
	g.Expect(btpWithFullAncestors.Gateways).To(BeEmpty(), "Policy with full ancestors should not get gateways assigned")

	// Verify that the normal policy gets its gateway assigned
	g.Expect(btpNormal.Gateways).To(HaveLen(1))
	g.Expect(btpNormal.Gateways[0]).To(Equal(types.NamespacedName{Namespace: "test", Name: "gateway2"}))

	// Verify that gateway1 received the ancestor limit condition
	gateway1 := gateways[types.NamespacedName{Namespace: "test", Name: "gateway1"}]
	g.Expect(gateway1.Conditions).To(HaveLen(1), "Gateway should have received ancestor limit condition")

	condition := gateway1.Conditions[0]
	g.Expect(condition.Type).To(Equal(string(v1alpha2.PolicyConditionAccepted)))
	g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
	g.Expect(condition.Reason).To(Equal(string(conditions.PolicyReasonAncestorLimitReached)))
	g.Expect(condition.Message).To(ContainSubstring("ancestor status list has reached the maximum size"))

	// Verify that gateway2 did not receive any conditions (normal case)
	gateway2 := gateways[types.NamespacedName{Namespace: "test", Name: "gateway2"}]
	g.Expect(gateway2.Conditions).To(BeEmpty(), "Normal gateway should not have conditions")

	// Verify logging function works - test the logging function directly
	logAncestorLimitReached(testLogger, "test/btp-full-ancestors", "BackendTLSPolicy", "test/gateway1")
	logOutput := logBuf.String()

	g.Expect(logOutput).To(ContainSubstring("Policy ancestor limit reached for test/btp-full-ancestors"))
	g.Expect(logOutput).To(ContainSubstring("test/btp-full-ancestors"))
	g.Expect(logOutput).To(ContainSubstring("policyKind=BackendTLSPolicy"))
	g.Expect(logOutput).To(ContainSubstring("ancestor=test/gateway1"))
}

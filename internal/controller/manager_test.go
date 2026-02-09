package controller

import (
	"errors"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	apiv1 "k8s.io/api/core/v1"
	discoveryV1 "k8s.io/api/discovery/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	inference "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/config"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/crd/crdfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/secrets"
	ngftypes "github.com/nginx/nginx-gateway-fabric/v2/internal/framework/types"
)

func TestPrepareFirstEventBatchPreparerArgs(t *testing.T) {
	t.Parallel()
	const gcName = "nginx"

	partialObjectMetadataList := &metav1.PartialObjectMetadataList{}
	partialObjectMetadataList.SetGroupVersionKind(
		schema.GroupVersionKind{
			Group:   apiext.GroupName,
			Version: "v1",
			Kind:    "CustomResourceDefinition",
		},
	)

	tests := []struct {
		discoveredCRDs      map[string]bool
		name                string
		expectedObjects     []client.Object
		expectedObjectLists []client.ObjectList
		cfg                 config.Config
	}{
		{
			name: "base case with BackendTLSPolicy v1",
			cfg: config.Config{
				GatewayClassName: gcName,
			},
			discoveredCRDs: map[string]bool{
				"BackendTLSPolicy": true,
			},
			expectedObjects: []client.Object{
				&gatewayv1.GatewayClass{ObjectMeta: metav1.ObjectMeta{Name: "nginx"}},
			},
			expectedObjectLists: []client.ObjectList{
				&apiv1.ServiceList{},
				&apiv1.SecretList{},
				&apiv1.NamespaceList{},
				&discoveryV1.EndpointSliceList{},
				&gatewayv1.HTTPRouteList{},
				&gatewayv1.BackendTLSPolicyList{},
				&apiv1.ConfigMapList{},
				&gatewayv1.GatewayList{},
				&gatewayv1beta1.ReferenceGrantList{},
				&ngfAPIv1alpha2.NginxProxyList{},
				&gatewayv1.GRPCRouteList{},
				partialObjectMetadataList,
				&ngfAPIv1alpha1.ClientSettingsPolicyList{},
				&ngfAPIv1alpha2.ObservabilityPolicyList{},
				&ngfAPIv1alpha1.ProxySettingsPolicyList{},
				&ngfAPIv1alpha1.UpstreamSettingsPolicyList{},
				&ngfAPIv1alpha1.AuthenticationFilterList{},
				&ngfAPIv1alpha1.RateLimitPolicyList{},
				&ngfAPIv1alpha1.WAFGatewayBindingPolicyList{},
			},
		},
		{
			name: "base case without BackendTLSPolicy v1",
			cfg: config.Config{
				GatewayClassName: gcName,
			},
			discoveredCRDs: map[string]bool{
				"BackendTLSPolicy": false,
			},
			expectedObjects: []client.Object{
				&gatewayv1.GatewayClass{ObjectMeta: metav1.ObjectMeta{Name: "nginx"}},
			},
			expectedObjectLists: []client.ObjectList{
				&apiv1.ServiceList{},
				&apiv1.SecretList{},
				&apiv1.NamespaceList{},
				&discoveryV1.EndpointSliceList{},
				&gatewayv1.HTTPRouteList{},
				&apiv1.ConfigMapList{},
				&gatewayv1beta1.ReferenceGrantList{},
				&ngfAPIv1alpha2.NginxProxyList{},
				&gatewayv1.GRPCRouteList{},
				&ngfAPIv1alpha1.ClientSettingsPolicyList{},
				&ngfAPIv1alpha2.ObservabilityPolicyList{},
				&ngfAPIv1alpha1.ProxySettingsPolicyList{},
				&ngfAPIv1alpha1.UpstreamSettingsPolicyList{},
				&ngfAPIv1alpha1.AuthenticationFilterList{},
				&ngfAPIv1alpha1.RateLimitPolicyList{},
				partialObjectMetadataList,
				&gatewayv1.GatewayList{},
				&ngfAPIv1alpha1.WAFGatewayBindingPolicyList{},
			},
		},
		{
			name: "experimental enabled with BackendTLSPolicy v1",
			cfg: config.Config{
				GatewayClassName:     gcName,
				ExperimentalFeatures: true,
			},
			discoveredCRDs: map[string]bool{
				"BackendTLSPolicy": true,
				"TLSRoute":         true,
				"TCPRoute":         true,
				"UDPRoute":         true,
			},
			expectedObjects: []client.Object{
				&gatewayv1.GatewayClass{ObjectMeta: metav1.ObjectMeta{Name: "nginx"}},
			},
			expectedObjectLists: []client.ObjectList{
				&apiv1.ServiceList{},
				&apiv1.SecretList{},
				&apiv1.NamespaceList{},
				&apiv1.ConfigMapList{},
				&discoveryV1.EndpointSliceList{},
				&gatewayv1.HTTPRouteList{},
				&gatewayv1.GatewayList{},
				&gatewayv1beta1.ReferenceGrantList{},
				&ngfAPIv1alpha2.NginxProxyList{},
				partialObjectMetadataList,
				&gatewayv1.BackendTLSPolicyList{},
				&gatewayv1alpha2.TLSRouteList{},
				&gatewayv1alpha2.TCPRouteList{},
				&gatewayv1alpha2.UDPRouteList{},
				&gatewayv1.GRPCRouteList{},
				&ngfAPIv1alpha1.ClientSettingsPolicyList{},
				&ngfAPIv1alpha2.ObservabilityPolicyList{},
				&ngfAPIv1alpha1.ProxySettingsPolicyList{},
				&ngfAPIv1alpha1.UpstreamSettingsPolicyList{},
				&ngfAPIv1alpha1.AuthenticationFilterList{},
				&ngfAPIv1alpha1.RateLimitPolicyList{},
				&ngfAPIv1alpha1.WAFGatewayBindingPolicyList{},
			},
		},
		{
			name: "inference extension enabled with BackendTLSPolicy v1",
			cfg: config.Config{
				GatewayClassName:   gcName,
				InferenceExtension: true,
			},
			discoveredCRDs: map[string]bool{
				"BackendTLSPolicy": true,
				"InferencePool":    true,
			},
			expectedObjects: []client.Object{
				&gatewayv1.GatewayClass{ObjectMeta: metav1.ObjectMeta{Name: "nginx"}},
			},
			expectedObjectLists: []client.ObjectList{
				&apiv1.ServiceList{},
				&apiv1.SecretList{},
				&apiv1.NamespaceList{},
				&discoveryV1.EndpointSliceList{},
				&gatewayv1.HTTPRouteList{},
				&gatewayv1.BackendTLSPolicyList{},
				&apiv1.ConfigMapList{},
				&gatewayv1beta1.ReferenceGrantList{},
				&ngfAPIv1alpha2.NginxProxyList{},
				&gatewayv1.GRPCRouteList{},
				&ngfAPIv1alpha1.ClientSettingsPolicyList{},
				&ngfAPIv1alpha2.ObservabilityPolicyList{},
				&ngfAPIv1alpha1.ProxySettingsPolicyList{},
				&ngfAPIv1alpha1.UpstreamSettingsPolicyList{},
				&ngfAPIv1alpha1.RateLimitPolicyList{},
				partialObjectMetadataList,
				&inference.InferencePoolList{},
				&gatewayv1.GatewayList{},
				&ngfAPIv1alpha1.AuthenticationFilterList{},
				&ngfAPIv1alpha1.WAFGatewayBindingPolicyList{},
			},
		},
		{
			name: "snippets filters enabled with BackendTLSPolicy v1",
			cfg: config.Config{
				GatewayClassName: gcName,
				SnippetsFilters:  true,
			},
			discoveredCRDs: map[string]bool{
				"BackendTLSPolicy": true,
			},
			expectedObjects: []client.Object{
				&gatewayv1.GatewayClass{ObjectMeta: metav1.ObjectMeta{Name: "nginx"}},
			},
			expectedObjectLists: []client.ObjectList{
				&apiv1.ServiceList{},
				&apiv1.SecretList{},
				&apiv1.NamespaceList{},
				&discoveryV1.EndpointSliceList{},
				&gatewayv1.HTTPRouteList{},
				&gatewayv1.BackendTLSPolicyList{},
				&apiv1.ConfigMapList{},
				&gatewayv1.GatewayList{},
				&gatewayv1beta1.ReferenceGrantList{},
				&ngfAPIv1alpha2.NginxProxyList{},
				partialObjectMetadataList,
				&gatewayv1.GRPCRouteList{},
				&ngfAPIv1alpha1.ClientSettingsPolicyList{},
				&ngfAPIv1alpha2.ObservabilityPolicyList{},
				&ngfAPIv1alpha1.SnippetsFilterList{},
				&ngfAPIv1alpha1.ProxySettingsPolicyList{},
				&ngfAPIv1alpha1.UpstreamSettingsPolicyList{},
				&ngfAPIv1alpha1.AuthenticationFilterList{},
				&ngfAPIv1alpha1.RateLimitPolicyList{},
				&ngfAPIv1alpha1.WAFGatewayBindingPolicyList{},
			},
		},
		{
			name: "snippets enabled",
			cfg: config.Config{
				GatewayClassName: gcName,
				Snippets:         true,
			},
			discoveredCRDs: map[string]bool{
				"BackendTLSPolicy": true,
			},
			expectedObjects: []client.Object{
				&gatewayv1.GatewayClass{ObjectMeta: metav1.ObjectMeta{Name: "nginx"}},
			},
			expectedObjectLists: []client.ObjectList{
				&apiv1.ServiceList{},
				&apiv1.SecretList{},
				&apiv1.NamespaceList{},
				&discoveryV1.EndpointSliceList{},
				&gatewayv1.HTTPRouteList{},
				&gatewayv1.BackendTLSPolicyList{},
				&apiv1.ConfigMapList{},
				&gatewayv1.GatewayList{},
				&gatewayv1beta1.ReferenceGrantList{},
				&ngfAPIv1alpha2.NginxProxyList{},
				partialObjectMetadataList,
				&gatewayv1.GRPCRouteList{},
				&ngfAPIv1alpha1.ClientSettingsPolicyList{},
				&ngfAPIv1alpha2.ObservabilityPolicyList{},
				&ngfAPIv1alpha1.SnippetsFilterList{},
				&ngfAPIv1alpha1.SnippetsPolicyList{},
				&ngfAPIv1alpha1.ProxySettingsPolicyList{},
				&ngfAPIv1alpha1.UpstreamSettingsPolicyList{},
				&ngfAPIv1alpha1.AuthenticationFilterList{},
				&ngfAPIv1alpha1.RateLimitPolicyList{},
				&ngfAPIv1alpha1.WAFGatewayBindingPolicyList{},
			},
		},
		{
			name: "experimental, inference, and snippets filters enabled with BackendTLSPolicy v1",
			cfg: config.Config{
				GatewayClassName:     gcName,
				ExperimentalFeatures: true,
				InferenceExtension:   true,
				SnippetsFilters:      true,
			},
			discoveredCRDs: map[string]bool{
				"BackendTLSPolicy": true,
				"TLSRoute":         true,
				"TCPRoute":         true,
				"UDPRoute":         true,
				"InferencePool":    true,
			},
			expectedObjects: []client.Object{
				&gatewayv1.GatewayClass{ObjectMeta: metav1.ObjectMeta{Name: "nginx"}},
			},
			expectedObjectLists: []client.ObjectList{
				&apiv1.ServiceList{},
				&apiv1.SecretList{},
				&apiv1.NamespaceList{},
				&apiv1.ConfigMapList{},
				&discoveryV1.EndpointSliceList{},
				&gatewayv1.HTTPRouteList{},
				&gatewayv1.GatewayList{},
				&gatewayv1beta1.ReferenceGrantList{},
				&ngfAPIv1alpha2.NginxProxyList{},
				partialObjectMetadataList,
				&inference.InferencePoolList{},
				&gatewayv1.BackendTLSPolicyList{},
				&gatewayv1alpha2.TLSRouteList{},
				&gatewayv1alpha2.TCPRouteList{},
				&gatewayv1alpha2.UDPRouteList{},
				&gatewayv1.GRPCRouteList{},
				&ngfAPIv1alpha1.ClientSettingsPolicyList{},
				&ngfAPIv1alpha2.ObservabilityPolicyList{},
				&ngfAPIv1alpha1.SnippetsFilterList{},
				&ngfAPIv1alpha1.ProxySettingsPolicyList{},
				&ngfAPIv1alpha1.UpstreamSettingsPolicyList{},
				&ngfAPIv1alpha1.AuthenticationFilterList{},
				&ngfAPIv1alpha1.RateLimitPolicyList{},
				&ngfAPIv1alpha1.WAFGatewayBindingPolicyList{},
			},
		},
		{
			name: "all features enabled",
			cfg: config.Config{
				GatewayClassName:     gcName,
				ExperimentalFeatures: true,
				InferenceExtension:   true,
				Snippets:             true,
			},
			discoveredCRDs: map[string]bool{
				"BackendTLSPolicy": true,
				"TLSRoute":         true,
				"TCPRoute":         true,
				"UDPRoute":         true,
				"InferencePool":    true,
			},
			expectedObjects: []client.Object{
				&gatewayv1.GatewayClass{ObjectMeta: metav1.ObjectMeta{Name: "nginx"}},
			},
			expectedObjectLists: []client.ObjectList{
				&apiv1.ServiceList{},
				&apiv1.SecretList{},
				&apiv1.NamespaceList{},
				&apiv1.ConfigMapList{},
				&discoveryV1.EndpointSliceList{},
				&gatewayv1.HTTPRouteList{},
				&gatewayv1.GatewayList{},
				&gatewayv1beta1.ReferenceGrantList{},
				&ngfAPIv1alpha2.NginxProxyList{},
				partialObjectMetadataList,
				&inference.InferencePoolList{},
				&gatewayv1.BackendTLSPolicyList{},
				&gatewayv1alpha2.TLSRouteList{},
				&gatewayv1alpha2.TCPRouteList{},
				&gatewayv1alpha2.UDPRouteList{},
				&gatewayv1.GRPCRouteList{},
				&ngfAPIv1alpha1.ClientSettingsPolicyList{},
				&ngfAPIv1alpha2.ObservabilityPolicyList{},
				&ngfAPIv1alpha1.SnippetsFilterList{},
				&ngfAPIv1alpha1.SnippetsPolicyList{},
				&ngfAPIv1alpha1.ProxySettingsPolicyList{},
				&ngfAPIv1alpha1.UpstreamSettingsPolicyList{},
				&ngfAPIv1alpha1.AuthenticationFilterList{},
				&ngfAPIv1alpha1.RateLimitPolicyList{},
				&ngfAPIv1alpha1.WAFGatewayBindingPolicyList{},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			objects, objectLists := prepareFirstEventBatchPreparerArgs(test.cfg, test.discoveredCRDs)

			g.Expect(objects).To(ConsistOf(test.expectedObjects))
			g.Expect(objectLists).To(ConsistOf(test.expectedObjectLists))
		})
	}
}

func TestGetMetricsOptions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		expectedOptions metricsserver.Options
		metricsConfig   config.MetricsConfig
	}{
		{
			name:            "Metrics disabled",
			metricsConfig:   config.MetricsConfig{Enabled: false},
			expectedOptions: metricsserver.Options{BindAddress: "0"},
		},
		{
			name: "Metrics enabled, not secure",
			metricsConfig: config.MetricsConfig{
				Port:    9113,
				Enabled: true,
				Secure:  false,
			},
			expectedOptions: metricsserver.Options{
				SecureServing: false,
				BindAddress:   ":9113",
			},
		},
		{
			name: "Metrics enabled, secure",
			metricsConfig: config.MetricsConfig{
				Port:    9113,
				Enabled: true,
				Secure:  true,
			},
			expectedOptions: metricsserver.Options{
				SecureServing: true,
				BindAddress:   ":9113",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			metricsServerOptions := getMetricsOptions(test.metricsConfig)

			g.Expect(metricsServerOptions).To(Equal(test.expectedOptions))
		})
	}
}

func TestCreatePlusSecretMetadata(t *testing.T) {
	t.Parallel()

	jwtSecret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ngf",
			Name:      "nplus-license",
		},
		Data: map[string][]byte{
			secrets.LicenseJWTKey: []byte("data"),
		},
	}

	jwtSecretWrongField := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ngf",
			Name:      "nplus-license",
		},
		Data: map[string][]byte{
			"wrong": []byte("data"),
		},
	}

	caSecret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ngf",
			Name:      "ca",
		},
		Data: map[string][]byte{
			secrets.CAKey: []byte("data"),
		},
	}

	caSecretWrongField := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ngf",
			Name:      "ca",
		},
		Data: map[string][]byte{
			"wrong": []byte("data"),
		},
	}

	clientSecret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ngf",
			Name:      "client",
		},
		Data: map[string][]byte{
			secrets.TLSCertKey: []byte("data"),
			secrets.TLSKeyKey:  []byte("data"),
		},
	}

	clientSecretWrongCert := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ngf",
			Name:      "client",
		},
		Data: map[string][]byte{
			"wrong":            []byte("data"),
			secrets.TLSCertKey: []byte("data"),
		},
	}

	clientSecretWrongKey := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ngf",
			Name:      "client",
		},
		Data: map[string][]byte{
			secrets.TLSCertKey: []byte("data"),
			"wrong":            []byte("data"),
		},
	}

	tests := []struct {
		expSecrets map[types.NamespacedName][]graph.PlusSecretFile
		name       string
		secrets    []runtime.Object
		cfg        config.Config
		expErr     bool
	}{
		{
			name: "plus not enabled",
			cfg: config.Config{
				Plus: false,
			},
			expSecrets: map[types.NamespacedName][]graph.PlusSecretFile{},
		},
		{
			name:    "only JWT token specified",
			secrets: []runtime.Object{jwtSecret},
			cfg: config.Config{
				Plus:             true,
				GatewayPodConfig: config.GatewayPodConfig{Namespace: jwtSecret.Namespace},
				UsageReportConfig: config.UsageReportConfig{
					SecretName: jwtSecret.Name,
				},
			},
			expSecrets: map[types.NamespacedName][]graph.PlusSecretFile{
				{Name: jwtSecret.Name, Namespace: jwtSecret.Namespace}: {
					{
						FieldName: secrets.LicenseJWTKey,
						Type:      graph.PlusReportJWTToken,
					},
				},
			},
		},
		{
			name:    "JWT and CA specified",
			secrets: []runtime.Object{jwtSecret, caSecret},
			cfg: config.Config{
				Plus:             true,
				GatewayPodConfig: config.GatewayPodConfig{Namespace: jwtSecret.Namespace},
				UsageReportConfig: config.UsageReportConfig{
					SecretName:   jwtSecret.Name,
					CASecretName: caSecret.Name,
				},
			},
			expSecrets: map[types.NamespacedName][]graph.PlusSecretFile{
				{Name: jwtSecret.Name, Namespace: jwtSecret.Namespace}: {
					{
						FieldName: secrets.LicenseJWTKey,
						Type:      graph.PlusReportJWTToken,
					},
				},
				{Name: caSecret.Name, Namespace: jwtSecret.Namespace}: {
					{
						FieldName: secrets.CAKey,
						Type:      graph.PlusReportCACertificate,
					},
				},
			},
		},
		{
			name:    "all Secrets specified",
			secrets: []runtime.Object{jwtSecret, caSecret, clientSecret},
			cfg: config.Config{
				Plus:             true,
				GatewayPodConfig: config.GatewayPodConfig{Namespace: jwtSecret.Namespace},
				UsageReportConfig: config.UsageReportConfig{
					SecretName:          jwtSecret.Name,
					CASecretName:        caSecret.Name,
					ClientSSLSecretName: clientSecret.Name,
				},
			},
			expSecrets: map[types.NamespacedName][]graph.PlusSecretFile{
				{Name: jwtSecret.Name, Namespace: jwtSecret.Namespace}: {
					{
						FieldName: secrets.LicenseJWTKey,
						Type:      graph.PlusReportJWTToken,
					},
				},
				{Name: caSecret.Name, Namespace: jwtSecret.Namespace}: {
					{
						FieldName: secrets.CAKey,
						Type:      graph.PlusReportCACertificate,
					},
				},
				{Name: clientSecret.Name, Namespace: jwtSecret.Namespace}: {
					{
						FieldName: secrets.TLSCertKey,
						Type:      graph.PlusReportClientSSLCertificate,
					},
					{
						FieldName: secrets.TLSKeyKey,
						Type:      graph.PlusReportClientSSLKey,
					},
				},
			},
		},
		{
			name: "JWT Secret doesn't exist",
			cfg: config.Config{
				Plus:             true,
				GatewayPodConfig: config.GatewayPodConfig{Namespace: jwtSecret.Namespace},
				UsageReportConfig: config.UsageReportConfig{
					SecretName: jwtSecret.Name,
				},
			},
			expSecrets: nil,
			expErr:     true,
		},
		{
			name:    "JWT Secret doesn't have correct field",
			secrets: []runtime.Object{jwtSecretWrongField},
			cfg: config.Config{
				Plus:             true,
				GatewayPodConfig: config.GatewayPodConfig{Namespace: jwtSecret.Namespace},
				UsageReportConfig: config.UsageReportConfig{
					SecretName: jwtSecret.Name,
				},
			},
			expSecrets: nil,
			expErr:     true,
		},
		{
			name:    "CA Secret doesn't exist",
			secrets: []runtime.Object{jwtSecret},
			cfg: config.Config{
				Plus:             true,
				GatewayPodConfig: config.GatewayPodConfig{Namespace: jwtSecret.Namespace},
				UsageReportConfig: config.UsageReportConfig{
					SecretName:   jwtSecret.Name,
					CASecretName: caSecret.Name,
				},
			},
			expSecrets: nil,
			expErr:     true,
		},
		{
			name:    "CA Secret doesn't have correct field",
			secrets: []runtime.Object{jwtSecretWrongField, caSecretWrongField},
			cfg: config.Config{
				Plus:             true,
				GatewayPodConfig: config.GatewayPodConfig{Namespace: jwtSecret.Namespace},
				UsageReportConfig: config.UsageReportConfig{
					SecretName:   jwtSecret.Name,
					CASecretName: caSecret.Name,
				},
			},
			expSecrets: nil,
			expErr:     true,
		},
		{
			name:    "Client Secret doesn't exist",
			secrets: []runtime.Object{jwtSecret, caSecret},
			cfg: config.Config{
				Plus:             true,
				GatewayPodConfig: config.GatewayPodConfig{Namespace: jwtSecret.Namespace},
				UsageReportConfig: config.UsageReportConfig{
					SecretName:          jwtSecret.Name,
					CASecretName:        caSecret.Name,
					ClientSSLSecretName: clientSecret.Name,
				},
			},
			expSecrets: nil,
			expErr:     true,
		},
		{
			name:    "Client Secret doesn't have correct cert",
			secrets: []runtime.Object{jwtSecret, caSecret, clientSecretWrongCert},
			cfg: config.Config{
				Plus:             true,
				GatewayPodConfig: config.GatewayPodConfig{Namespace: jwtSecret.Namespace},
				UsageReportConfig: config.UsageReportConfig{
					SecretName:          jwtSecret.Name,
					CASecretName:        caSecret.Name,
					ClientSSLSecretName: clientSecret.Name,
				},
			},
			expSecrets: nil,
			expErr:     true,
		},
		{
			name:    "Client Secret doesn't have correct key",
			secrets: []runtime.Object{jwtSecret, caSecret, clientSecretWrongKey},
			cfg: config.Config{
				Plus:             true,
				GatewayPodConfig: config.GatewayPodConfig{Namespace: jwtSecret.Namespace},
				UsageReportConfig: config.UsageReportConfig{
					SecretName:          jwtSecret.Name,
					CASecretName:        caSecret.Name,
					ClientSSLSecretName: clientSecret.Name,
				},
			},
			expSecrets: nil,
			expErr:     true,
		},
	}

	for _, test := range tests {
		fakeClient := fake.NewFakeClient(test.secrets...)

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			plusSecrets, err := createPlusSecretMetadata(test.cfg, fakeClient)
			if test.expErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}

			g.Expect(plusSecrets).To(Equal(test.expSecrets))
		})
	}
}

func TestFilterControllersByCRDExistence(t *testing.T) {
	t.Parallel()

	backendTLSPolicyGVK := schema.GroupVersionKind{
		Group:   "gateway.networking.k8s.io",
		Version: "v1",
		Kind:    "BackendTLSPolicy",
	}

	tlsRouteGVK := schema.GroupVersionKind{
		Group:   "gateway.networking.k8s.io",
		Version: "v1alpha2",
		Kind:    "TLSRoute",
	}

	tcpRouteGVK := schema.GroupVersionKind{
		Group:   "gateway.networking.k8s.io",
		Version: "v1alpha2",
		Kind:    "TCPRoute",
	}

	tests := []struct {
		crdCheckError          error
		crdCheckResults        map[schema.GroupVersionKind]bool
		expectedDiscoveredCRDs map[string]bool
		name                   string
		controllers            []ctlrCfg
		expectedControllerCnt  int
		expectError            bool
	}{
		{
			name: "no controllers require CRD check",
			controllers: []ctlrCfg{
				{
					name:            "HTTPRoute",
					objectType:      &gatewayv1.HTTPRoute{},
					requireCRDCheck: false,
				},
				{
					name:            "Gateway",
					objectType:      &gatewayv1.Gateway{},
					requireCRDCheck: false,
				},
			},
			crdCheckResults:        nil,
			expectedControllerCnt:  2,
			expectedDiscoveredCRDs: map[string]bool{},
			expectError:            false,
		},
		{
			name: "all CRDs exist",
			controllers: []ctlrCfg{
				{
					name:            "HTTPRoute",
					objectType:      &gatewayv1.HTTPRoute{},
					requireCRDCheck: false,
				},
				{
					name:            "BackendTLSPolicy",
					objectType:      &gatewayv1.BackendTLSPolicy{},
					requireCRDCheck: true,
					crdGVK:          &backendTLSPolicyGVK,
				},
				{
					name:            "TLSRoute",
					objectType:      &gatewayv1alpha2.TLSRoute{},
					requireCRDCheck: true,
					crdGVK:          &tlsRouteGVK,
				},
			},
			crdCheckResults: map[schema.GroupVersionKind]bool{
				backendTLSPolicyGVK: true,
				tlsRouteGVK:         true,
			},
			expectedControllerCnt: 3,
			expectedDiscoveredCRDs: map[string]bool{
				"BackendTLSPolicy": true,
				"TLSRoute":         true,
			},
			expectError: false,
		},
		{
			name: "some CRDs missing",
			controllers: []ctlrCfg{
				{
					name:            "HTTPRoute",
					objectType:      &gatewayv1.HTTPRoute{},
					requireCRDCheck: false,
				},
				{
					name:            "BackendTLSPolicy",
					objectType:      &gatewayv1.BackendTLSPolicy{},
					requireCRDCheck: true,
					crdGVK:          &backendTLSPolicyGVK,
				},
				{
					name:            "TLSRoute",
					objectType:      &gatewayv1alpha2.TLSRoute{},
					requireCRDCheck: true,
					crdGVK:          &tlsRouteGVK,
				},
			},
			crdCheckResults: map[schema.GroupVersionKind]bool{
				backendTLSPolicyGVK: true,
				tlsRouteGVK:         false,
			},
			expectedControllerCnt: 2, // HTTPRoute and BackendTLSPolicy only
			expectedDiscoveredCRDs: map[string]bool{
				"BackendTLSPolicy": true,
				"TLSRoute":         false,
			},
			expectError: false,
		},
		{
			name: "all CRDs missing",
			controllers: []ctlrCfg{
				{
					name:            "HTTPRoute",
					objectType:      &gatewayv1.HTTPRoute{},
					requireCRDCheck: false,
				},
				{
					name:            "BackendTLSPolicy",
					objectType:      &gatewayv1.BackendTLSPolicy{},
					requireCRDCheck: true,
					crdGVK:          &backendTLSPolicyGVK,
				},
				{
					name:            "TLSRoute",
					objectType:      &gatewayv1alpha2.TLSRoute{},
					requireCRDCheck: true,
					crdGVK:          &tlsRouteGVK,
				},
			},
			crdCheckResults: map[schema.GroupVersionKind]bool{
				backendTLSPolicyGVK: false,
				tlsRouteGVK:         false,
			},
			expectedControllerCnt: 1, // Only HTTPRoute
			expectedDiscoveredCRDs: map[string]bool{
				"BackendTLSPolicy": false,
				"TLSRoute":         false,
			},
			expectError: false,
		},
		{
			name: "CRD check error",
			controllers: []ctlrCfg{
				{
					name:            "BackendTLSPolicy",
					objectType:      &gatewayv1.BackendTLSPolicy{},
					requireCRDCheck: true,
					crdGVK:          &backendTLSPolicyGVK,
				},
			},
			crdCheckResults:        nil,
			crdCheckError:          errors.New("failed to connect to API server"),
			expectedControllerCnt:  0,
			expectedDiscoveredCRDs: nil,
			expectError:            true,
		},
		{
			name: "multiple controllers with same GVK",
			controllers: []ctlrCfg{
				{
					name:            "TLSRoute-1",
					objectType:      &gatewayv1alpha2.TLSRoute{},
					requireCRDCheck: true,
					crdGVK:          &tlsRouteGVK,
				},
				{
					name:            "TLSRoute-2",
					objectType:      &gatewayv1alpha2.TLSRoute{},
					requireCRDCheck: true,
					crdGVK:          &tlsRouteGVK,
				},
			},
			crdCheckResults: map[schema.GroupVersionKind]bool{
				tlsRouteGVK: true,
			},
			expectedControllerCnt: 2,
			expectedDiscoveredCRDs: map[string]bool{
				"TLSRoute": true,
			},
			expectError: false,
		},
		{
			name: "controller without crdGVK override uses object type GVK",
			controllers: []ctlrCfg{
				{
					name:            "TCPRoute",
					objectType:      createTypedObject(&tcpRouteGVK),
					requireCRDCheck: true,
					crdGVK:          nil, // No override, should use object's GVK
				},
			},
			crdCheckResults: map[schema.GroupVersionKind]bool{
				tcpRouteGVK: true,
			},
			expectedControllerCnt: 1,
			expectedDiscoveredCRDs: map[string]bool{
				"TCPRoute": true,
			},
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			// Create a fake config provider
			fakeMgr := &fakeManagerForCRDTest{
				config: &rest.Config{},
			}

			// Create fake checker
			fakeChecker := &crdfakes.FakeChecker{}
			fakeChecker.CheckCRDsExistReturns(test.crdCheckResults, test.crdCheckError)

			// Call the function
			filtered, discoveredCRDs, err := filterControllersByCRDExistence(
				fakeMgr,
				test.controllers,
				fakeChecker,
			)

			// Verify results
			if test.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(filtered).To(HaveLen(test.expectedControllerCnt))
				g.Expect(discoveredCRDs).To(Equal(test.expectedDiscoveredCRDs))

				// Verify that CheckCRDsExist was called with the right config and GVKs
				if len(test.crdCheckResults) > 0 || test.crdCheckError != nil {
					g.Expect(fakeChecker.CheckCRDsExistCallCount()).To(Equal(1))
					config, gvks := fakeChecker.CheckCRDsExistArgsForCall(0)
					g.Expect(config).To(Equal(fakeMgr.config))
					// Verify all expected GVKs were passed
					expectedGVKs := make(map[schema.GroupVersionKind]bool)
					for gvk := range test.crdCheckResults {
						expectedGVKs[gvk] = true
					}
					for _, gvk := range gvks {
						g.Expect(expectedGVKs).To(HaveKey(gvk))
					}
				}
			}
		})
	}
}

// fakeManagerForCRDTest implements only GetConfig() method needed for filterControllersByCRDExistence.
type fakeManagerForCRDTest struct {
	config *rest.Config
}

func (f *fakeManagerForCRDTest) GetConfig() *rest.Config {
	return f.config
}

// createTypedObject creates a typed object with GVK set for testing.
func createTypedObject(gvk *schema.GroupVersionKind) ngftypes.ObjectType {
	obj := &gatewayv1alpha2.TCPRoute{}
	obj.SetGroupVersionKind(*gvk)
	return obj
}

func TestCreatePLMSecretMetadata(t *testing.T) {
	t.Parallel()

	const defaultNamespace = "nginx-gateway"

	tests := []struct {
		expSecrets map[types.NamespacedName]*graph.PLMSecretConfig
		name       string
		plmCfg     config.PLMStorageConfig
	}{
		{
			name:       "PLM not configured (empty URL)",
			plmCfg:     config.PLMStorageConfig{},
			expSecrets: map[types.NamespacedName]*graph.PLMSecretConfig{},
		},
		{
			name: "only credentials secret specified",
			plmCfg: config.PLMStorageConfig{
				URL:                   "http://plm-storage:8333",
				CredentialsSecretName: "plm-creds",
			},
			expSecrets: map[types.NamespacedName]*graph.PLMSecretConfig{
				{Namespace: defaultNamespace, Name: "plm-creds"}: {
					Type: graph.PLMSecretTypeCredentials,
				},
			},
		},
		{
			name: "credentials and CA secrets specified",
			plmCfg: config.PLMStorageConfig{
				URL:                   "http://plm-storage:8333",
				CredentialsSecretName: "plm-creds",
				TLSCACertSecretName:   "plm-ca",
			},
			expSecrets: map[types.NamespacedName]*graph.PLMSecretConfig{
				{Namespace: defaultNamespace, Name: "plm-creds"}: {
					Type: graph.PLMSecretTypeCredentials,
				},
				{Namespace: defaultNamespace, Name: "plm-ca"}: {
					Type: graph.PLMSecretTypeTLSCA,
				},
			},
		},
		{
			name: "all secrets specified",
			plmCfg: config.PLMStorageConfig{
				URL:                    "https://plm-storage:8333",
				CredentialsSecretName:  "plm-creds",
				TLSCACertSecretName:    "plm-ca",
				TLSClientSSLSecretName: "plm-client",
			},
			expSecrets: map[types.NamespacedName]*graph.PLMSecretConfig{
				{Namespace: defaultNamespace, Name: "plm-creds"}: {
					Type: graph.PLMSecretTypeCredentials,
				},
				{Namespace: defaultNamespace, Name: "plm-ca"}: {
					Type: graph.PLMSecretTypeTLSCA,
				},
				{Namespace: defaultNamespace, Name: "plm-client"}: {
					Type: graph.PLMSecretTypeTLSClient,
				},
			},
		},
		{
			name: "secrets with namespace prefix",
			plmCfg: config.PLMStorageConfig{
				URL:                    "https://plm-storage:8333",
				CredentialsSecretName:  "other-ns/plm-creds",
				TLSCACertSecretName:    "ca-ns/plm-ca",
				TLSClientSSLSecretName: "tls-ns/plm-client",
			},
			expSecrets: map[types.NamespacedName]*graph.PLMSecretConfig{
				{Namespace: "other-ns", Name: "plm-creds"}: {
					Type: graph.PLMSecretTypeCredentials,
				},
				{Namespace: "ca-ns", Name: "plm-ca"}: {
					Type: graph.PLMSecretTypeTLSCA,
				},
				{Namespace: "tls-ns", Name: "plm-client"}: {
					Type: graph.PLMSecretTypeTLSClient,
				},
			},
		},
		{
			name: "only TLS CA secret specified",
			plmCfg: config.PLMStorageConfig{
				URL:                 "https://plm-storage:8333",
				TLSCACertSecretName: "plm-ca",
			},
			expSecrets: map[types.NamespacedName]*graph.PLMSecretConfig{
				{Namespace: defaultNamespace, Name: "plm-ca"}: {
					Type: graph.PLMSecretTypeTLSCA,
				},
			},
		},
		{
			name: "only TLS client secret specified",
			plmCfg: config.PLMStorageConfig{
				URL:                    "https://plm-storage:8333",
				TLSClientSSLSecretName: "plm-client",
			},
			expSecrets: map[types.NamespacedName]*graph.PLMSecretConfig{
				{Namespace: defaultNamespace, Name: "plm-client"}: {
					Type: graph.PLMSecretTypeTLSClient,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			plmSecrets := createPLMSecretMetadata(test.plmCfg, defaultNamespace)

			g.Expect(plmSecrets).To(Equal(test.expSecrets))
		})
	}
}

func TestCreateWAFFetcher(t *testing.T) {
	t.Parallel()

	const (
		defaultNamespace = "nginx-gateway"
		plmStorageURL    = "https://plm-storage:8333"
	)

	credsSecret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: defaultNamespace,
			Name:      "plm-creds",
		},
		Data: map[string][]byte{
			secrets.PLMCredentialsKey: []byte("secret-access-key"),
		},
	}

	caSecret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: defaultNamespace,
			Name:      "plm-ca",
		},
		Data: map[string][]byte{
			secrets.CAKey: []byte(validCACert),
		},
	}

	tests := []struct {
		plmSecrets            map[types.NamespacedName]*graph.PLMSecretConfig
		name                  string
		plmStorageURL         string
		k8sObjects            []runtime.Object
		tlsInsecureSkipVerify bool
		expErr                bool
		expFetcherNil         bool
	}{
		{
			name:          "PLM not configured (empty URL)",
			plmSecrets:    map[types.NamespacedName]*graph.PLMSecretConfig{},
			plmStorageURL: "",
			expFetcherNil: true,
			expErr:        false,
		},
		{
			name: "credentials only",
			plmSecrets: map[types.NamespacedName]*graph.PLMSecretConfig{
				{Namespace: defaultNamespace, Name: "plm-creds"}: {
					Type: graph.PLMSecretTypeCredentials,
				},
			},
			plmStorageURL: plmStorageURL,
			k8sObjects:    []runtime.Object{credsSecret},
			expErr:        false,
			expFetcherNil: false,
		},
		{
			name: "credentials and CA cert",
			plmSecrets: map[types.NamespacedName]*graph.PLMSecretConfig{
				{Namespace: defaultNamespace, Name: "plm-creds"}: {
					Type: graph.PLMSecretTypeCredentials,
				},
				{Namespace: defaultNamespace, Name: "plm-ca"}: {
					Type: graph.PLMSecretTypeTLSCA,
				},
			},
			plmStorageURL: plmStorageURL,
			k8sObjects:    []runtime.Object{credsSecret, caSecret},
			expErr:        false,
			expFetcherNil: false,
		},
		{
			name: "TLS insecure skip verify enabled",
			plmSecrets: map[types.NamespacedName]*graph.PLMSecretConfig{
				{Namespace: defaultNamespace, Name: "plm-creds"}: {
					Type: graph.PLMSecretTypeCredentials,
				},
			},
			plmStorageURL:         plmStorageURL,
			tlsInsecureSkipVerify: true,
			k8sObjects:            []runtime.Object{credsSecret},
			expErr:                false,
			expFetcherNil:         false,
		},
		{
			name: "missing credentials secret",
			plmSecrets: map[types.NamespacedName]*graph.PLMSecretConfig{
				{Namespace: defaultNamespace, Name: "plm-creds"}: {
					Type: graph.PLMSecretTypeCredentials,
				},
			},
			plmStorageURL: plmStorageURL,
			k8sObjects:    []runtime.Object{}, // secret not created
			expErr:        true,
			expFetcherNil: false,
		},
		{
			name: "missing CA secret",
			plmSecrets: map[types.NamespacedName]*graph.PLMSecretConfig{
				{Namespace: defaultNamespace, Name: "plm-ca"}: {
					Type: graph.PLMSecretTypeTLSCA,
				},
			},
			plmStorageURL: plmStorageURL,
			k8sObjects:    []runtime.Object{}, // secret not created
			expErr:        true,
			expFetcherNil: false,
		},
		{
			name:          "no secrets configured (just URL)",
			plmSecrets:    map[types.NamespacedName]*graph.PLMSecretConfig{},
			plmStorageURL: plmStorageURL,
			k8sObjects:    []runtime.Object{},
			expErr:        false,
			expFetcherNil: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			scheme := runtime.NewScheme()
			g.Expect(apiv1.AddToScheme(scheme)).To(Succeed())

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(test.k8sObjects...).
				Build()

			fetcher, err := createWAFFetcher(
				test.plmSecrets,
				test.plmStorageURL,
				test.tlsInsecureSkipVerify,
				fakeClient,
				logr.Discard(),
			)

			if test.expErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}

			if test.expFetcherNil {
				g.Expect(fetcher).To(BeNil())
			} else if !test.expErr {
				g.Expect(fetcher).ToNot(BeNil())
			}
		})
	}
}

func TestBuildTLSConfig(t *testing.T) {
	t.Parallel()

	const defaultNamespace = "nginx-gateway"

	caSecret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: defaultNamespace,
			Name:      "plm-ca",
		},
		Data: map[string][]byte{
			secrets.CAKey: []byte(validCACert),
		},
	}

	tests := []struct {
		plmSecrets            map[types.NamespacedName]*graph.PLMSecretConfig
		name                  string
		k8sObjects            []runtime.Object
		tlsInsecureSkipVerify bool
		expErr                bool
		expConfigNil          bool
	}{
		{
			name:         "no TLS secrets",
			plmSecrets:   map[types.NamespacedName]*graph.PLMSecretConfig{},
			k8sObjects:   []runtime.Object{},
			expErr:       false,
			expConfigNil: true,
		},
		{
			name: "CA cert only",
			plmSecrets: map[types.NamespacedName]*graph.PLMSecretConfig{
				{Namespace: defaultNamespace, Name: "plm-ca"}: {
					Type: graph.PLMSecretTypeTLSCA,
				},
			},
			k8sObjects:   []runtime.Object{caSecret},
			expErr:       false,
			expConfigNil: false,
		},
		{
			name: "insecure skip verify enabled",
			plmSecrets: map[types.NamespacedName]*graph.PLMSecretConfig{
				{Namespace: defaultNamespace, Name: "plm-ca"}: {
					Type: graph.PLMSecretTypeTLSCA,
				},
			},
			tlsInsecureSkipVerify: true,
			k8sObjects:            []runtime.Object{caSecret},
			expErr:                false,
			expConfigNil:          false,
		},
		{
			name: "missing CA secret",
			plmSecrets: map[types.NamespacedName]*graph.PLMSecretConfig{
				{Namespace: defaultNamespace, Name: "plm-ca"}: {
					Type: graph.PLMSecretTypeTLSCA,
				},
			},
			k8sObjects:   []runtime.Object{}, // secret not created
			expErr:       true,
			expConfigNil: false,
		},
		{
			name: "missing client secret",
			plmSecrets: map[types.NamespacedName]*graph.PLMSecretConfig{
				{Namespace: defaultNamespace, Name: "plm-client"}: {
					Type: graph.PLMSecretTypeTLSClient,
				},
			},
			k8sObjects:   []runtime.Object{}, // secret not created
			expErr:       true,
			expConfigNil: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			scheme := runtime.NewScheme()
			g.Expect(apiv1.AddToScheme(scheme)).To(Succeed())

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(test.k8sObjects...).
				Build()

			tlsConfig, err := buildTLSConfig(
				test.plmSecrets,
				test.tlsInsecureSkipVerify,
				fakeClient,
			)

			if test.expErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}

			if test.expConfigNil {
				g.Expect(tlsConfig).To(BeNil())
			} else if !test.expErr {
				g.Expect(tlsConfig).ToNot(BeNil())
			}
		})
	}
}

const (
	// Valid CA certificate for testing (from existing test files).
	validCACert = `-----BEGIN CERTIFICATE-----
MIIDSDCCAjACCQDKWvrpwiIyCDANBgkqhkiG9w0BAQsFADBmMQswCQYDVQQGEwJV
UzELMAkGA1UECAwCQ0ExFjAUBgNVBAcMDVNhbiBGcmFuc2lzY28xDjAMBgNVBAoM
BU5HSU5YMQwwCgYDVQQLDANLSUMxFDASBgNVBAMMC2V4YW1wbGUuY29tMB4XDTIw
MTExMjIxMjg0MloXDTMwMTExMDIxMjg0MlowZjELMAkGA1UEBhMCVVMxCzAJBgNV
BAgMAkNBMRYwFAYDVQQHDA1TYW4gRnJhbnNpc2NvMQ4wDAYDVQQKDAVOR0lOWDEM
MAoGA1UECwwDS0lDMRQwEgYDVQQDDAtleGFtcGxlLmNvbTCCASIwDQYJKoZIhvcN
AQEBBQADggEPADCCAQoCggEBAMrlKMqrHfMR4mgaL2zZG2DYYfKCFVmINjlYuOeC
FDTcRgQKtu2YcCxZYBADwHZxEf6NIKtVsMWLhSNS/Nc0BmtiQM/IExhlCiDC6Sl8
ONrI3w7qJzN6IUERB6tVlQt07rgM0V26UTYu0Ikv1Y8trfLYPZckzBkorQjpcium
qoP2BJf4yyc9LqpxtlWKxelkunVL5ijMEzpj9gEE26TEHbsdEbhoR8g0OeHZqH7e
mXCnSIBR0A/o/s6noGNX+F19lY7Tgw77jOuQQ5Ysi+7nhN2lKvcC819RX7oMpgvt
V5B3nI0mF6BaznjeTs4yQcr1Sm3UTVBwX9ZuvL7RbIXkUm8CAwEAATANBgkqhkiG
9w0BAQsFAAOCAQEAgm04w6OIWGj6tka9ccccnblF0oZzeEAIywjvR5sDcPdvLIeM
eesJy6rFH4DBmMygpcIxJGrSOzZlF3LMvw7zK4stqNtm1HiprF8bzxfTffVYncg6
hVKErHtZ2FZRj/2TMJ01aRDZSuVbL6UJiokpU6xxT7yy0dFZkKrjUR349gKxRqJw
Am2as0bhi51EqK1GEx3m4c0un2vNh5qP2hv6e/Qze6P96vefNaSk9QMFfuB1kSAk
fGpkiL7bjmjnhKwAmf8jDWDZltB6S56Qy2QjPR8JoOusbYxar4c6EcIwVHv6mdgP
yZxWqQsgtSfFx+Pwon9IPKuq0jQYgeZPSxRMLA==
-----END CERTIFICATE-----`
)

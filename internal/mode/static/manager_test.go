package static

import (
	"testing"

	. "github.com/onsi/gomega"
	apiv1 "k8s.io/api/core/v1"
	discoveryV1 "k8s.io/api/discovery/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1alpha3 "sigs.k8s.io/gateway-api/apis/v1alpha3"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/config"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/graph"
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
		expectedObjects     []client.Object
		expectedObjectLists []client.ObjectList
		name                string
		cfg                 config.Config
	}{
		{
			name: "gwNsName is nil",
			cfg: config.Config{
				GatewayClassName:     gcName,
				GatewayNsName:        nil,
				ExperimentalFeatures: false,
				SnippetsFilters:      false,
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
				&gatewayv1.GatewayList{},
				&gatewayv1beta1.ReferenceGrantList{},
				&ngfAPIv1alpha1.NginxProxyList{},
				&gatewayv1.GRPCRouteList{},
				partialObjectMetadataList,
				&ngfAPIv1alpha1.ClientSettingsPolicyList{},
				&ngfAPIv1alpha2.ObservabilityPolicyList{},
				&ngfAPIv1alpha1.UpstreamSettingsPolicyList{},
			},
		},
		{
			name: "gwNsName is not nil",
			cfg: config.Config{
				GatewayClassName: gcName,
				GatewayNsName: &types.NamespacedName{
					Namespace: "test",
					Name:      "my-gateway",
				},
				ExperimentalFeatures: false,
				SnippetsFilters:      false,
			},
			expectedObjects: []client.Object{
				&gatewayv1.GatewayClass{ObjectMeta: metav1.ObjectMeta{Name: "nginx"}},
				&gatewayv1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "my-gateway", Namespace: "test"}},
			},
			expectedObjectLists: []client.ObjectList{
				&apiv1.ServiceList{},
				&apiv1.SecretList{},
				&apiv1.NamespaceList{},
				&discoveryV1.EndpointSliceList{},
				&gatewayv1.HTTPRouteList{},
				&gatewayv1beta1.ReferenceGrantList{},
				&ngfAPIv1alpha1.NginxProxyList{},
				&gatewayv1.GRPCRouteList{},
				partialObjectMetadataList,
				&ngfAPIv1alpha1.ClientSettingsPolicyList{},
				&ngfAPIv1alpha2.ObservabilityPolicyList{},
				&ngfAPIv1alpha1.UpstreamSettingsPolicyList{},
			},
		},
		{
			name: "gwNsName is not nil and experimental enabled",
			cfg: config.Config{
				GatewayClassName: gcName,
				GatewayNsName: &types.NamespacedName{
					Namespace: "test",
					Name:      "my-gateway",
				},
				ExperimentalFeatures: true,
				SnippetsFilters:      false,
			},
			expectedObjects: []client.Object{
				&gatewayv1.GatewayClass{ObjectMeta: metav1.ObjectMeta{Name: "nginx"}},
				&gatewayv1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "my-gateway", Namespace: "test"}},
			},
			expectedObjectLists: []client.ObjectList{
				&apiv1.ServiceList{},
				&apiv1.SecretList{},
				&apiv1.NamespaceList{},
				&apiv1.ConfigMapList{},
				&discoveryV1.EndpointSliceList{},
				&gatewayv1.HTTPRouteList{},
				&gatewayv1beta1.ReferenceGrantList{},
				&ngfAPIv1alpha1.NginxProxyList{},
				partialObjectMetadataList,
				&gatewayv1alpha3.BackendTLSPolicyList{},
				&gatewayv1alpha2.TLSRouteList{},
				&gatewayv1.GRPCRouteList{},
				&ngfAPIv1alpha1.ClientSettingsPolicyList{},
				&ngfAPIv1alpha2.ObservabilityPolicyList{},
				&ngfAPIv1alpha1.UpstreamSettingsPolicyList{},
			},
		},
		{
			name: "gwNsName is not nil and snippets filters enabled",
			cfg: config.Config{
				GatewayClassName: gcName,
				GatewayNsName: &types.NamespacedName{
					Namespace: "test",
					Name:      "my-gateway",
				},
				ExperimentalFeatures: false,
				SnippetsFilters:      true,
			},
			expectedObjects: []client.Object{
				&gatewayv1.GatewayClass{ObjectMeta: metav1.ObjectMeta{Name: "nginx"}},
				&gatewayv1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "my-gateway", Namespace: "test"}},
			},
			expectedObjectLists: []client.ObjectList{
				&apiv1.ServiceList{},
				&apiv1.SecretList{},
				&apiv1.NamespaceList{},
				&discoveryV1.EndpointSliceList{},
				&gatewayv1.HTTPRouteList{},
				&gatewayv1beta1.ReferenceGrantList{},
				&ngfAPIv1alpha1.NginxProxyList{},
				partialObjectMetadataList,
				&gatewayv1.GRPCRouteList{},
				&ngfAPIv1alpha1.ClientSettingsPolicyList{},
				&ngfAPIv1alpha2.ObservabilityPolicyList{},
				&ngfAPIv1alpha1.SnippetsFilterList{},
				&ngfAPIv1alpha1.UpstreamSettingsPolicyList{},
			},
		},
		{
			name: "gwNsName is not nil, experimental and snippets filters enabled",
			cfg: config.Config{
				GatewayClassName: gcName,
				GatewayNsName: &types.NamespacedName{
					Namespace: "test",
					Name:      "my-gateway",
				},
				ExperimentalFeatures: true,
				SnippetsFilters:      true,
			},
			expectedObjects: []client.Object{
				&gatewayv1.GatewayClass{ObjectMeta: metav1.ObjectMeta{Name: "nginx"}},
				&gatewayv1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "my-gateway", Namespace: "test"}},
			},
			expectedObjectLists: []client.ObjectList{
				&apiv1.ServiceList{},
				&apiv1.SecretList{},
				&apiv1.NamespaceList{},
				&apiv1.ConfigMapList{},
				&discoveryV1.EndpointSliceList{},
				&gatewayv1.HTTPRouteList{},
				&gatewayv1beta1.ReferenceGrantList{},
				&ngfAPIv1alpha1.NginxProxyList{},
				partialObjectMetadataList,
				&gatewayv1alpha3.BackendTLSPolicyList{},
				&gatewayv1alpha2.TLSRouteList{},
				&gatewayv1.GRPCRouteList{},
				&ngfAPIv1alpha1.ClientSettingsPolicyList{},
				&ngfAPIv1alpha2.ObservabilityPolicyList{},
				&ngfAPIv1alpha1.SnippetsFilterList{},
				&ngfAPIv1alpha1.UpstreamSettingsPolicyList{},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			objects, objectLists := prepareFirstEventBatchPreparerArgs(test.cfg)

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
			plusLicenseField: []byte("data"),
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
			plusCAField: []byte("data"),
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
			plusClientCertField: []byte("data"),
			plusClientKeyField:  []byte("data"),
		},
	}

	clientSecretWrongCert := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ngf",
			Name:      "client",
		},
		Data: map[string][]byte{
			"wrong":            []byte("data"),
			plusClientKeyField: []byte("data"),
		},
	}

	clientSecretWrongKey := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ngf",
			Name:      "client",
		},
		Data: map[string][]byte{
			plusClientCertField: []byte("data"),
			"wrong":             []byte("data"),
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
						FieldName: plusLicenseField,
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
						FieldName: plusLicenseField,
						Type:      graph.PlusReportJWTToken,
					},
				},
				{Name: caSecret.Name, Namespace: jwtSecret.Namespace}: {
					{
						FieldName: plusCAField,
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
						FieldName: plusLicenseField,
						Type:      graph.PlusReportJWTToken,
					},
				},
				{Name: caSecret.Name, Namespace: jwtSecret.Namespace}: {
					{
						FieldName: plusCAField,
						Type:      graph.PlusReportCACertificate,
					},
				},
				{Name: clientSecret.Name, Namespace: jwtSecret.Namespace}: {
					{
						FieldName: plusClientCertField,
						Type:      graph.PlusReportClientSSLCertificate,
					},
					{
						FieldName: plusClientKeyField,
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

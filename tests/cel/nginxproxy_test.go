package cel

import (
	"testing"

	controllerruntime "sigs.k8s.io/controller-runtime"

	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

func TestNginxProxyKubernetes(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha2.NginxProxySpec
		name       string
		wantErrors []string
	}{
		{
			name:       "Validate NginxProxy with both Deployment and DaemonSet is invalid",
			wantErrors: []string{expectedOneOfDeploymentOrDaemonSetError},
			spec: ngfAPIv1alpha2.NginxProxySpec{
				Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
					Deployment: &ngfAPIv1alpha2.DeploymentSpec{},
					DaemonSet:  &ngfAPIv1alpha2.DaemonSetSpec{},
				},
			},
		},
		{
			name: "Validate NginxProxy with Deployment only is valid",
			spec: ngfAPIv1alpha2.NginxProxySpec{
				Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
					Deployment: &ngfAPIv1alpha2.DeploymentSpec{},
				},
			},
		},
		{
			name: "Validate NginxProxy with DaemonSet only is valid",
			spec: ngfAPIv1alpha2.NginxProxySpec{
				Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
					DaemonSet: &ngfAPIv1alpha2.DaemonSetSpec{},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec := tt.spec
			resourceName := uniqueResourceName(testResourceName)

			nginxProxy := &ngfAPIv1alpha2.NginxProxy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      resourceName,
					Namespace: defaultNamespace,
				},
				Spec: spec,
			}
			validateCrd(t, tt.wantErrors, nginxProxy, k8sClient)
		})
	}
}

func TestNginxProxyRewriteClientIP(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha2.NginxProxySpec
		name       string
		wantErrors []string
	}{
		{
			name:       "Validate NginxProxy is invalid when trustedAddresses is not set and mode is set",
			wantErrors: []string{expectedIfModeSetTrustedAddressesError},
			spec: ngfAPIv1alpha2.NginxProxySpec{
				RewriteClientIP: &ngfAPIv1alpha2.RewriteClientIP{
					Mode: helpers.GetPointer[ngfAPIv1alpha2.RewriteClientIPModeType]("XForwardedFor"),
				},
			},
		},
		{
			name: "Validate NginxProxy is valid when both mode and trustedAddresses are set",
			spec: ngfAPIv1alpha2.NginxProxySpec{
				RewriteClientIP: &ngfAPIv1alpha2.RewriteClientIP{
					Mode: helpers.GetPointer[ngfAPIv1alpha2.RewriteClientIPModeType]("XForwardedFor"),
					TrustedAddresses: []ngfAPIv1alpha2.RewriteClientIPAddress{
						{
							Type:  ngfAPIv1alpha2.RewriteClientIPAddressType("CIDR"),
							Value: "10.0.0.0/8",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec := tt.spec
			resourceName := uniqueResourceName(testResourceName)

			nginxProxy := &ngfAPIv1alpha2.NginxProxy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      resourceName,
					Namespace: defaultNamespace,
				},
				Spec: spec,
			}
			validateCrd(t, tt.wantErrors, nginxProxy, k8sClient)
		})
	}
}

func TestNginxProxyAutoscaling(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha2.NginxProxySpec
		name       string
		wantErrors []string
	}{
		{
			name:       "Validate NginxProxy is invalid when MinReplicas not less than, or equal to MaxReplicas",
			wantErrors: []string{expectedMinReplicasLessThanOrEqualError},
			spec: ngfAPIv1alpha2.NginxProxySpec{
				Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
					Deployment: &ngfAPIv1alpha2.DeploymentSpec{
						Autoscaling: &ngfAPIv1alpha2.AutoscalingSpec{
							MinReplicas: helpers.GetPointer[int32](10),
							MaxReplicas: 5,
						},
					},
				},
			},
		},
		{
			name: "Validate NginxProxy is valid when MinReplicas is less than MaxReplicas",
			spec: ngfAPIv1alpha2.NginxProxySpec{
				Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
					Deployment: &ngfAPIv1alpha2.DeploymentSpec{
						Autoscaling: &ngfAPIv1alpha2.AutoscalingSpec{
							MinReplicas: helpers.GetPointer[int32](1),
							MaxReplicas: 5,
						},
					},
				},
			},
		},
		{
			name: "Validate NginxProxy is valid when MinReplicas is equal to MaxReplicas",
			spec: ngfAPIv1alpha2.NginxProxySpec{
				Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
					Deployment: &ngfAPIv1alpha2.DeploymentSpec{
						Autoscaling: &ngfAPIv1alpha2.AutoscalingSpec{
							MinReplicas: helpers.GetPointer[int32](5),
							MaxReplicas: 5,
						},
					},
				},
			},
		},
		{
			name: "Validate NginxProxy is valid when MinReplicas is nil",
			spec: ngfAPIv1alpha2.NginxProxySpec{
				Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
					Deployment: &ngfAPIv1alpha2.DeploymentSpec{
						Autoscaling: &ngfAPIv1alpha2.AutoscalingSpec{
							MinReplicas: nil,
							MaxReplicas: 5,
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec := tt.spec
			resourceName := uniqueResourceName(testResourceName)

			nginxProxy := &ngfAPIv1alpha2.NginxProxy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      resourceName,
					Namespace: defaultNamespace,
				},
				Spec: spec,
			}
			validateCrd(t, tt.wantErrors, nginxProxy, k8sClient)
		})
	}
}

func TestNginxProxyCompression(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha2.NginxProxySpec
		name       string
		wantErrors []string
	}{
		{
			name:       "Validate NginxProxy is invalid when compression has no mimeTypes",
			wantErrors: []string{expectedCompressionMimeTypesRequiredError},
			spec: ngfAPIv1alpha2.NginxProxySpec{
				Compression: &ngfAPIv1alpha2.Compression{
					Type: ngfAPIv1alpha2.GzipCompressionType,
					Gzip: &ngfAPIv1alpha2.GzipSettings{},
				},
			},
		},
		{
			name:       "Validate NginxProxy is invalid when gzip type without gzip settings",
			wantErrors: []string{expectedCompressionGzipRequiredError},
			spec: ngfAPIv1alpha2.NginxProxySpec{
				Compression: &ngfAPIv1alpha2.Compression{
					Type:      ngfAPIv1alpha2.GzipCompressionType,
					MimeTypes: []string{"text/css"},
				},
			},
		},
		{
			name: "Validate NginxProxy is valid with complete gzip compression",
			spec: ngfAPIv1alpha2.NginxProxySpec{
				Compression: &ngfAPIv1alpha2.Compression{
					Type:      ngfAPIv1alpha2.GzipCompressionType,
					MimeTypes: []string{"text/css", "application/json"},
					Gzip: &ngfAPIv1alpha2.GzipSettings{
						Vary:    helpers.GetPointer(true),
						Proxied: []ngfAPIv1alpha2.GzipProxiedType{ngfAPIv1alpha2.GzipProxiedAny},
					},
				},
			},
		},
		{
			name: "Validate NginxProxy is valid with compression buffers",
			spec: ngfAPIv1alpha2.NginxProxySpec{
				Compression: &ngfAPIv1alpha2.Compression{
					Type:      ngfAPIv1alpha2.GzipCompressionType,
					MimeTypes: []string{"text/css"},
					Gzip:      &ngfAPIv1alpha2.GzipSettings{},
					Buffers: &ngfAPIv1alpha2.CompressionBuffers{
						Number: 32,
						Size:   "4k",
					},
				},
			},
		},
		{
			name: "Validate NginxProxy is valid with gzip http version",
			spec: ngfAPIv1alpha2.NginxProxySpec{
				Compression: &ngfAPIv1alpha2.Compression{
					Type:      ngfAPIv1alpha2.GzipCompressionType,
					MimeTypes: []string{"text/css"},
					Gzip: &ngfAPIv1alpha2.GzipSettings{
						HTTPVersion: helpers.GetPointer(ngfAPIv1alpha2.GzipHTTPVersion10),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec := tt.spec
			resourceName := uniqueResourceName(testResourceName)

			nginxProxy := &ngfAPIv1alpha2.NginxProxy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      resourceName,
					Namespace: defaultNamespace,
				},
				Spec: spec,
			}
			validateCrd(t, tt.wantErrors, nginxProxy, k8sClient)
		})
	}
}

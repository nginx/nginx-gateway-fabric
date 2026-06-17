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

func TestNginxProxyAccessLogFormat(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha2.NginxProxySpec
		name       string
		wantErrors []string
	}{
		{
			name: "Validate NginxProxy with valid standard access log format is accepted",
			spec: ngfAPIv1alpha2.NginxProxySpec{
				Logging: &ngfAPIv1alpha2.NginxLogging{
					AccessLog: &ngfAPIv1alpha2.NginxAccessLog{
						Format: helpers.GetPointer(
							`$remote_addr - $remote_user [$time_local] "$request" $status`,
						),
					},
				},
			},
		},
		{
			name: "Validate NginxProxy with valid JSON access log format is accepted",
			spec: ngfAPIv1alpha2.NginxProxySpec{
				Logging: &ngfAPIv1alpha2.NginxLogging{
					AccessLog: &ngfAPIv1alpha2.NginxAccessLog{
						Format: helpers.GetPointer(
							`{"remote_addr": "$remote_addr", "status": "$status"}`,
						),
					},
				},
			},
		},
		{
			name:       "Validate NginxProxy with single quote in access log format is rejected",
			wantErrors: []string{expectedAccessLogFormatPatternError},
			spec: ngfAPIv1alpha2.NginxProxySpec{
				Logging: &ngfAPIv1alpha2.NginxLogging{
					AccessLog: &ngfAPIv1alpha2.NginxAccessLog{
						Format: helpers.GetPointer(`'; bad stuff; #`),
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

func TestNginxProxyServerTokens(t *testing.T) {
	t.Parallel()
	k8sClient := getKubernetesClient(t)

	tests := []struct {
		spec       ngfAPIv1alpha2.NginxProxySpec
		name       string
		wantErrors []string
	}{
		{
			name: "Validate NginxProxy with valid keyword serverTokens is accepted",
			spec: ngfAPIv1alpha2.NginxProxySpec{
				ServerTokens: helpers.GetPointer("on"),
			},
		},
		{
			name: "Validate NginxProxy with valid custom serverTokens is accepted",
			spec: ngfAPIv1alpha2.NginxProxySpec{
				ServerTokens: helpers.GetPointer("my-custom-server"),
			},
		},
		{
			name: "Validate NginxProxy with empty string serverTokens is accepted",
			spec: ngfAPIv1alpha2.NginxProxySpec{
				ServerTokens: helpers.GetPointer(""),
			},
		},
		{
			name:       "Validate NginxProxy with bare double quote in serverTokens is rejected",
			wantErrors: []string{expectedServerTokensPatternError},
			spec: ngfAPIv1alpha2.NginxProxySpec{
				ServerTokens: helpers.GetPointer(`bad"value`),
			},
		},
		{
			name:       "Validate NginxProxy with trailing backslash in serverTokens is rejected",
			wantErrors: []string{expectedServerTokensPatternError},
			spec: ngfAPIv1alpha2.NginxProxySpec{
				ServerTokens: helpers.GetPointer(`bad\`),
			},
		},
		{
			name: "Validate NginxProxy with escaped quote in serverTokens is accepted",
			spec: ngfAPIv1alpha2.NginxProxySpec{
				ServerTokens: helpers.GetPointer(`my \"server\"`),
			},
		},
		{
			name: "Validate NginxProxy with dollar sign in serverTokens is accepted",
			spec: ngfAPIv1alpha2.NginxProxySpec{
				ServerTokens: helpers.GetPointer("$hostname"),
			},
		},
		{
			name:       "Validate NginxProxy with bare double quote in serverTokens is rejected",
			wantErrors: []string{expectedServerTokensPatternError},
			spec: ngfAPIv1alpha2.NginxProxySpec{
				ServerTokens: helpers.GetPointer(`bad"value`),
			},
		},
		{
			name:       "Validate NginxProxy with trailing backslash in serverTokens is rejected",
			wantErrors: []string{expectedServerTokensPatternError},
			spec: ngfAPIv1alpha2.NginxProxySpec{
				ServerTokens: helpers.GetPointer(`bad\`),
			},
		},
		{
			name: "Validate NginxProxy with escaped quote in serverTokens is accepted",
			spec: ngfAPIv1alpha2.NginxProxySpec{
				ServerTokens: helpers.GetPointer(`my \"server\"`),
			},
		},
		{
			name: "Validate NginxProxy with dollar sign in serverTokens is accepted",
			spec: ngfAPIv1alpha2.NginxProxySpec{
				ServerTokens: helpers.GetPointer("$hostname"),
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

package cel

import (
	"testing"

	. "github.com/onsi/gomega"
	controllerruntime "sigs.k8s.io/controller-runtime"

	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

func TestNginxProxyKubernetes(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	k8sClient, err := getKubernetesClient(t)
	g.Expect(err).ToNot(HaveOccurred())

	tests := []struct {
		policySpec ngfAPIv1alpha2.NginxProxySpec
		name       string
		wantErrors []string
	}{
		{
			name:       "Validate NginxProxy with both Deployment and DaemonSet is invalid",
			wantErrors: []string{expectedOneOfDeploymentOrDaemonSetError},
			policySpec: ngfAPIv1alpha2.NginxProxySpec{
				Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
					Deployment: &ngfAPIv1alpha2.DeploymentSpec{},
					DaemonSet:  &ngfAPIv1alpha2.DaemonSetSpec{},
				},
			},
		},
		{
			name: "Validate NginxProxy with Deployment only is valid",
			policySpec: ngfAPIv1alpha2.NginxProxySpec{
				Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
					Deployment: &ngfAPIv1alpha2.DeploymentSpec{},
				},
			},
		},
		{
			name: "Validate NginxProxy with DaemonSet only is valid",
			policySpec: ngfAPIv1alpha2.NginxProxySpec{
				Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
					DaemonSet: &ngfAPIv1alpha2.DaemonSetSpec{},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			policySpec := tt.policySpec
			policyName := uniqueResourceName(testPolicyName)

			nginxProxy := &ngfAPIv1alpha2.NginxProxy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      policyName,
					Namespace: defaultNamespace,
				},
				Spec: policySpec,
			}
			validateCrd(t, tt.wantErrors, g, nginxProxy, k8sClient)
		})
	}
}

func TestNginxProxyRewriteClientIP(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	k8sClient, err := getKubernetesClient(t)
	g.Expect(err).ToNot(HaveOccurred())

	tests := []struct {
		policySpec ngfAPIv1alpha2.NginxProxySpec
		name       string
		wantErrors []string
	}{
		{
			name:       "Validate NginxProxy is invalid when trustedAddresses is not set and mode is set",
			wantErrors: []string{expectedIfModeSetTrustedAddressesError},
			policySpec: ngfAPIv1alpha2.NginxProxySpec{
				RewriteClientIP: &ngfAPIv1alpha2.RewriteClientIP{
					Mode: helpers.GetPointer[ngfAPIv1alpha2.RewriteClientIPModeType]("XForwardedFor"),
				},
			},
		},
		{
			name: "Validate NginxProxy is valid when both mode and trustedAddresses are set",
			policySpec: ngfAPIv1alpha2.NginxProxySpec{
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
			policySpec := tt.policySpec
			policyName := uniqueResourceName(testPolicyName)

			nginxProxy := &ngfAPIv1alpha2.NginxProxy{
				ObjectMeta: controllerruntime.ObjectMeta{
					Name:      policyName,
					Namespace: defaultNamespace,
				},
				Spec: policySpec,
			}
			validateCrd(t, tt.wantErrors, g, nginxProxy, k8sClient)
		})
	}
}

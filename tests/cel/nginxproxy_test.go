package cel

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
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
			wantErrors: []string{"only one of deployment or daemonSet can be set"},
			policySpec: ngfAPIv1alpha2.NginxProxySpec{
				Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
					Deployment: &ngfAPIv1alpha2.DeploymentSpec{
						Replicas: helpers.GetPointer[int32](3),
					},
					DaemonSet: &ngfAPIv1alpha2.DaemonSetSpec{},
				},
			},
		},
		{
			name: "Validate NginxProxy with Deployment only is valid",
			policySpec: ngfAPIv1alpha2.NginxProxySpec{
				Kubernetes: &ngfAPIv1alpha2.KubernetesSpec{
					Deployment: &ngfAPIv1alpha2.DeploymentSpec{
						Replicas: helpers.GetPointer[int32](3),
					},
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
			validateNginxProxy(t, tt, g, k8sClient)
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
			wantErrors: []string{"if mode is set, trustedAddresses is a required field"},
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
			validateNginxProxy(t, tt, g, k8sClient)
		})
	}
}

func validateNginxProxy(t *testing.T, tt struct {
	policySpec ngfAPIv1alpha2.NginxProxySpec
	name       string
	wantErrors []string
}, g *WithT, k8sClient client.Client,
) {
	t.Helper()

	policySpec := tt.policySpec
	policyName := uniqueResourceName(testPolicyName)

	nginxProxy := &ngfAPIv1alpha2.NginxProxy{
		ObjectMeta: controllerruntime.ObjectMeta{
			Name:      policyName,
			Namespace: defaultNamespace,
		},
		Spec: policySpec,
	}
	timeoutConfig := framework.DefaultTimeoutConfig()
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.KubernetesClientTimeout)
	err := k8sClient.Create(ctx, nginxProxy)
	defer cancel()

	// Clean up after test
	defer func() {
		_ = k8sClient.Delete(context.Background(), nginxProxy)
	}()

	if len(tt.wantErrors) == 0 {
		g.Expect(err).ToNot(HaveOccurred())
	} else {
		g.Expect(err).To(HaveOccurred())
		for _, wantError := range tt.wantErrors {
			g.Expect(err.Error()).To(ContainSubstring(wantError), "Expected error '%s' not found in: %s", wantError, err.Error())
		}
	}
}

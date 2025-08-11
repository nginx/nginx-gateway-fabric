package cel

import (
	"context"
	"crypto/rand"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

const (
	gatewayKind   = "Gateway"
	httpRouteKind = "HTTPRoute"
	grpcRouteKind = "GRPCRoute"
	tcpRouteKind  = "TCPRoute"
	invalidKind   = "InvalidKind"
)

const (
	gatewayGroup   = "gateway.networking.k8s.io"
	invalidGroup   = "invalid.networking.k8s.io"
	discoveryGroup = "discovery.k8s.io/v1"
)

const (
	expectedTargetRefKindError              = `TargetRef Kind must be one of: Gateway, HTTPRoute, or GRPCRoute`
	expectedTargetRefGroupError             = `TargetRef Group must be gateway.networking.k8s.io.`
	expectedHeaderWithoutServerError        = `header can only be specified if server is specified`
	expectedOneOfDeploymentOrDaemonSetError = `only one of deployment or daemonSet can be set`
	expectedIfModeSetTrustedAddressesError  = `if mode is set, trustedAddresses is a required field`
	expectedMinReplicasLessThanOrEqualError = `minReplicas must be less than or equal to maxReplicas`
)

const (
	defaultNamespace = "default"
)

const (
	testPolicyName    = "test-policy"
	testTargetRefName = "test-targetRef"
)

// getKubernetesClient returns a client connected to a real Kubernetes cluster.
func getKubernetesClient(t *testing.T) (k8sClient client.Client, err error) {
	t.Helper()
	// Use controller-runtime to get cluster connection
	k8sConfig, err := controllerruntime.GetConfig()
	if err != nil {
		return nil, err
	}

	// Set up scheme with NGF types
	scheme := runtime.NewScheme()
	if err = ngfAPIv1alpha1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err = ngfAPIv1alpha2.AddToScheme(scheme); err != nil {
		return nil, err
	}
	// Create a new client with the scheme and return it
	return client.New(k8sConfig, client.Options{Scheme: scheme})
}

// randomPrimeNumber generates a random prime number of 64 bits.
// It panics if it fails to generate a random prime number.
func randomPrimeNumber() int64 {
	primeNum, err := rand.Prime(rand.Reader, 64)
	if err != nil {
		panic(fmt.Errorf("failed to generate random prime number: %w", err))
	}
	return primeNum.Int64()
}

// uniqueResourceName generates a unique resource name by appending a random prime number to the given name.
func uniqueResourceName(name string) string {
	return fmt.Sprintf("%s-%d", name, randomPrimeNumber())
}

// validateCrd creates a k8s resource and validates it against the expected errors.
func validateCrd(t *testing.T, wantErrors []string, g *WithT, crd client.Object, k8sClient client.Client) {
	t.Helper()

	timeoutConfig := framework.DefaultTimeoutConfig()
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.KubernetesClientTimeout)
	err := k8sClient.Create(ctx, crd)
	defer cancel()

	// Clean up after test
	defer func() {
		_ = k8sClient.Delete(context.Background(), crd)
	}()

	if len(wantErrors) == 0 {
		g.Expect(err).ToNot(HaveOccurred())
	} else {
		g.Expect(err).To(HaveOccurred())
		for _, wantError := range wantErrors {
			g.Expect(err.Error()).To(ContainSubstring(wantError), "Expected error '%s' not found in: %s", wantError, err.Error())
		}
	}
}

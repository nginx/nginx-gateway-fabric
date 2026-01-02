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
	serviceKind   = "Service"
)

const (
	gatewayGroup   = "gateway.networking.k8s.io"
	invalidGroup   = "invalid.networking.k8s.io"
	discoveryGroup = "discovery.k8s.io/v1"
	coreGroup      = "core"
	emptyGroup     = ""
)

const (
	// AuthenticationFilter validation errors.
	expectedBasicRequiredError = `for type=Basic, spec.basic must be set`

	// Kind validation errors.
	expectedTargetRefKindMustBeGatewayOrHTTPRouteOrGrpcRouteError = "TargetRef Kind must be one of: " +
		"Gateway, HTTPRoute, or GRPCRoute"
	expectedTargetRefKindMustBeHTTPRouteOrGrpcRouteError = "TargetRef Kind must be: HTTPRoute or GRPCRoute"
	expectedTargetRefKindServiceError                    = "TargetRefs Kind must be: Service"

	// Group validation errors.
	expectedTargetRefGroupError     = "TargetRef Group must be gateway.networking.k8s.io"
	expectedTargetRefGroupCoreError = "TargetRefs Group must be core"

	// Name uniqueness validation errors.
	expectedTargetRefNameUniqueError              = "TargetRef Name must be unique"
	expectedTargetRefKindAndNameComboMustBeUnique = "TargetRef Kind and Name combination must be unique"

	// UpstreamSettingsPolicy validation errors.
	expectedHashKeyLoadBalancingTypeError = `hashMethodKey is required when loadBalancingMethod ` +
		`is 'hash' or 'hash consistent'`

	// Header validation error.
	expectedHeaderWithoutServerError = "header can only be specified if server is specified"

	// Deployment/DaemonSet validation error.
	expectedOneOfDeploymentOrDaemonSetError = "only one of deployment or daemonSet can be set"

	// TrustedAddresses/Mode validation error.
	expectedIfModeSetTrustedAddressesError = "if mode is set, trustedAddresses is a required field"

	// Replicas validation error.
	expectedMinReplicasLessThanOrEqualError = "minReplicas must be less than or equal to maxReplicas"

	// Strategy validation error.
	expectedStrategyMustBeOfTypeRatio = "ratio can only be specified if strategy is of type ratio"

	// SnippetsFilter validation errors.
	expectedSnippetsFilterContextError = "Only one snippet allowed per context"

	// Namespace for tests.
	defaultNamespace = "default"

	// Test resource names.
	testResourceName  = "test-resource"
	testTargetRefName = "test-targetRef"
)

// getKubernetesClient returns a client connected to a real Kubernetes cluster.
func getKubernetesClient(t *testing.T) (k8sClient client.Client) {
	t.Helper()
	g := NewWithT(t)
	// Use controller-runtime to get cluster connection
	k8sConfig, err := controllerruntime.GetConfig()
	g.Expect(err).ToNot(HaveOccurred())

	// Set up scheme with NGF types
	scheme := runtime.NewScheme()
	g.Expect(ngfAPIv1alpha1.AddToScheme(scheme)).To(Succeed())
	g.Expect(ngfAPIv1alpha2.AddToScheme(scheme)).To(Succeed())

	k8sClient, err = client.New(k8sConfig, client.Options{Scheme: scheme})
	g.Expect(err).ToNot(HaveOccurred())

	return k8sClient
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
func validateCrd(t *testing.T, wantErrors []string, crd client.Object, k8sClient client.Client) {
	t.Helper()
	g := NewWithT(t)

	timeoutConfig := framework.DefaultTimeoutConfig()
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.KubernetesClientTimeout)
	defer cancel()
	err := k8sClient.Create(ctx, crd)

	// Check for expected errors
	if len(wantErrors) == 0 {
		g.Expect(err).ToNot(HaveOccurred())
		// Clean up after test
		// Resources only need to be deleted if they were created successfully
		g.Expect(k8sClient.Delete(ctx, crd)).To(Succeed())
	} else {
		g.Expect(err).To(HaveOccurred())
		for _, wantError := range wantErrors {
			g.Expect(err.Error()).To(ContainSubstring(wantError), "Expected error '%s' not found in: %s", wantError, err.Error())
		}
	}
}

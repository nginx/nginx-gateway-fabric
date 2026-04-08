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
	expectedBasicRequiredError           = "type Basic requires spec.basic to be set"
	expectedBasicOnlyNoJWTError          = "type Basic must not set spec.jwt"
	expectedBasicOnlyNoOIDCError         = "type Basic must not set spec.oidc"
	expectedOIDCRequiredError            = "type OIDC requires spec.oidc to be set"
	expectedOIDCNotAllowedWithBasicError = "type OIDC must not set spec.basic"
	expectedOIDCNotAllowedWithJWTError   = "type OIDC must not set spec.jwt"
	expectedJWTRequiredError             = "type JWT requires spec.jwt to be set"
	expectedJWTOnlyNoBasicError          = "type JWT must not set spec.basic"
	expectedJWTOnlyNoOIDCError           = "type JWT must not set spec.oidc"
	expectedJWTFileRequiredError         = "source File requires spec.file to be set"
	expectedJWTFileOnlyError             = "source File must not set spec.remote"
	expectedJWTRemoteRequiredError       = "source Remote requires spec.remote to be set"
	expectedJWTRemoteOnlyError           = "source Remote must not set spec.file"

	expectedTargetRefKindMustBeGatewayOrHTTPRouteOrGrpcRouteError = "TargetRef Kind must be one of: " +
		"Gateway, HTTPRoute, or GRPCRoute"
	expectedTargetRefKindMustBeHTTPRouteOrGrpcRouteError = "TargetRef Kind must be: HTTPRoute or GRPCRoute"
	expectedTargetRefKindServiceError                    = "TargetRefs Kind must be: Service"
	expectedTargetRefKindGatewayError                    = "TargetRef Kind must be: Gateway"
	expectedTargetRefAllSameKindError                    = "All TargetRefs must be the same Kind"

	// Group validation errors.
	expectedTargetRefGroupError     = "TargetRef Group must be gateway.networking.k8s.io"
	expectedTargetRefGroupCoreError = "TargetRefs Group must be core"

	// Name uniqueness validation errors.
	expectedTargetRefNameUniqueError              = "TargetRef Name must be unique"
	expectedTargetRefKindAndNameComboMustBeUnique = "TargetRef Kind and Name combination must be unique"

	// Header validation error.
	expectedHeaderWithoutServerError = "header can only be specified if server is specified"

	// Deployment/DaemonSet validation error.
	expectedOneOfDeploymentOrDaemonSetError = "only one of deployment or daemonSet can be set"

	expectedIfModeSetTrustedAddressesError = "if mode is set, trustedAddresses is a required field"

	// Replicas validation error.
	expectedMinReplicasLessThanOrEqualError = "minReplicas must be less than or equal to maxReplicas"

	// Strategy validation error.
	expectedStrategyMustBeOfTypeRatio = "ratio can only be specified if strategy is of type ratio"

	// Snippets validation errors.
	expectedSnippetsContextError = "Only one snippet allowed per context"

	// HashMethodKey validation error.
	expectedHashKeyLoadBalancingTypeError = `hashMethodKey is required when loadBalancingMethod ` +
		`is 'hash' or 'hash consistent'`

	// WAFGatewayBindingPolicy errors.
	expectedWAFFileIfAndOnlyIfFileTypeError   = "destination.file must be set if and only if type is file"
	expectedWAFSyslogIfAndOnlyIfSyslogType    = "destination.syslog must be set if and only if type is syslog"
	expectedWAFLogSourceMutualExclusionError  = "exactly one of logSource.url or logSource.defaultProfile must be set"
	expectedWAFValidationMutualExclusionError = "verifyChecksum and expectedChecksum are mutually exclusive"
	expectedWAFVerifyChecksumHTTPOnlyError    = "policySource.validation.verifyChecksum is only supported for type HTTP"
	expectedWAFNIMSourceIfAndOnlyIfNIMType    = "policySource.nimSource must be set if and only if type is NIM"
	expectedWAFN1CSourceIfAndOnlyIfN1CType    = "policySource.n1cSource must be set if and only if type is N1C"
	expectedWAFNIMPolicyUIDPatternError       = `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
	expectedWAFN1CPolicyObjectIDPatternError  = `^pol_[A-Za-z0-9_-]+$`
	expectedWAFN1CPolicyVersionIDPatternError = `^pv_[A-Za-z0-9_-]+$`

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

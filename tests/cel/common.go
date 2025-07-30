package cel

import (
	"crypto/rand"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
)

const (
	GatewayKind   = "Gateway"
	HTTPRouteKind = "HTTPRoute"
	GRPCRouteKind = "GRPCRoute"
	TCPRouteKind  = "TCPRoute"
	InvalidKind   = "InvalidKind"
)

const (
	GatewayGroup   = "gateway.networking.k8s.io"
	InvalidGroup   = "invalid.networking.k8s.io"
	DiscoveryGroup = "discovery.k8s.io/v1"
)

const (
	ExpectedTargetRefKindError  = `TargetRef Kind must be one of: Gateway, HTTPRoute, or GRPCRoute`
	ExpectedTargetRefGroupError = `TargetRef Group must be gateway.networking.k8s.io.`
)

const (
	PolicyName = "test-policy"
	TargetRef  = "targetRef-name"
)

// GetKubernetesClient returns a client connected to a real Kubernetes cluster.
func GetKubernetesClient(t *testing.T) (k8sClient client.Client, err error) {
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

// RandomPrimeNumber generates a random prime number of 64 bits.
// It panics if it fails to generate a random prime number.
func RandomPrimeNumber() int64 {
	primeNum, err := rand.Prime(rand.Reader, 64)
	if err != nil {
		panic(fmt.Errorf("failed to generate random prime number: %w", err))
	}
	return primeNum.Int64()
}

// UniqueResourceName generates a unique resource name by appending a random prime number to the given name.
func UniqueResourceName(name string) string {
	return fmt.Sprintf("%s-%d", name, RandomPrimeNumber())
}

package cel

import (
	"k8s.io/apimachinery/pkg/runtime"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
)

// GetKubernetesClient returns a client connected to a real Kubernetes cluster.
func GetKubernetesClient() (k8sClient client.Client, err error) {
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
	// Create a new client with the scheme and return it
	return client.New(k8sConfig, client.Options{Scheme: scheme})
}

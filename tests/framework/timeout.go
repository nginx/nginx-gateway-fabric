package framework

import "time"

type TimeoutConfig struct {
	// CreateTimeout represents the maximum time for a Kubernetes object to be created.
	CreateTimeout time.Duration

	// UpdateTimeout represents the maximum time for a Kubernetes object to be updated.
	UpdateTimeout time.Duration

	// DeleteTimeout represents the maximum time for a Kubernetes object to be deleted.
	DeleteTimeout time.Duration

	// DeleteNamespaceTimeout represents the maximum time for a Kubernetes namespace to be deleted.
	DeleteNamespaceTimeout time.Duration

	// GetTimeout represents the maximum time to get a Kubernetes object.
	GetTimeout time.Duration

	// ManifestFetchTimeout represents the maximum time for getting content from a https:// URL.
	ManifestFetchTimeout time.Duration

	// RequestTimeout represents the maximum time for making an HTTP Request with the roundtripper.
	RequestTimeout time.Duration

	// ContainerRestartTimeout represents the maximum time for a Kubernetes Container to restart.
	ContainerRestartTimeout time.Duration

	// GetLeaderLeaseTimeout represents the maximum time for NGF to retrieve the leader lease.
	GetLeaderLeaseTimeout time.Duration

	// GetStatusTimeout represents the maximum time for NGF to update the status of a resource.
	GetStatusTimeout time.Duration

	// TestForTrafficTimeout represents the maximum time for NGF to test for passing or failing traffic.
	TestForTrafficTimeout time.Duration

	// KubernetesClientTimeout represents the maximum time for Kubernetes client operations.
	KubernetesClientTimeout time.Duration

	// GatewayListenerUpdateTimeout represents the maximum time for Gateway Listener count to be updated.
	GatewayListenerUpdateTimeout time.Duration

	// GetNginxConfigTimeout represents the maximum time for getting NGINX configuration from the NGINX container.
	GetNginxConfigTimeout time.Duration
}

// DefaultTimeoutConfig populates a TimeoutConfig with the default values.
func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		CreateTimeout:                60 * time.Second,
		UpdateTimeout:                60 * time.Second,
		DeleteTimeout:                10 * time.Second,
		DeleteNamespaceTimeout:       150 * time.Second,
		GetTimeout:                   10 * time.Second,
		ManifestFetchTimeout:         10 * time.Second,
		RequestTimeout:               30 * time.Second,
		ContainerRestartTimeout:      10 * time.Second,
		GetLeaderLeaseTimeout:        60 * time.Second,
		GetStatusTimeout:             60 * time.Second,
		TestForTrafficTimeout:        60 * time.Second,
		KubernetesClientTimeout:      10 * time.Second,
		GatewayListenerUpdateTimeout: 60 * time.Second,
		GetNginxConfigTimeout:        90 * time.Second,
	}
}

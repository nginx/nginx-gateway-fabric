package config

import (
	"time"

	"github.com/go-logr/logr"
	"go.uber.org/zap"
)

const DefaultNginxMetricsPort = int32(9113)

type Config struct {
	// AtomicLevel is an atomically changeable, dynamic logging level.
	AtomicLevel zap.AtomicLevel
	// GatewayPodConfig contains information about this Pod.
	GatewayPodConfig GatewayPodConfig
	// PLMStorageConfig contains information about the PLM storage component.
	PLMStorageConfig PLMStorageConfig
	// Logger is the Zap Logger used by all components.
	Logger logr.Logger
	// NGINXSCCName is the name of the SecurityContextConstraints for the NGINX Pods. Only applicable in OpenShift.
	NGINXSCCName string
	// ConfigName is the name of the NginxGateway resource for this controller.
	ConfigName string
	// AgentTLSSecretName is the name of the TLS Secret used by NGINX Agent to communicate with the control plane.
	AgentTLSSecretName string
	// GatewayClassName is the name of the GatewayClass resource that the Gateway will use.
	GatewayClassName string
	// ImageSource is the source of the NGINX Gateway image.
	ImageSource string
	// GatewayCtlrName is the name of this controller.
	GatewayCtlrName string
	// UsageReportConfig specifies the NGINX Plus usage reporting configuration.
	UsageReportConfig UsageReportConfig
	// Flags contains the NGF command-line flag names and values.
	Flags Flags
	// LeaderElection contains the configuration for leader election.
	LeaderElection LeaderElectionConfig
	// NginxDockerSecretNames are the names of any Docker registry Secrets for the NGINX container.
	NginxDockerSecretNames []string
	// WatchNamespaces is the list of namespaces to watch for resources. If empty, all namespaces are watched.
	WatchNamespaces []string
	// NginxOneConsoleTelemetryConfig contains the configuration for NGINX One Console telemetry.
	NginxOneConsoleTelemetryConfig NginxOneConsoleTelemetryConfig
	// ProductTelemetryConfig contains the configuration for collecting product telemetry.
	ProductTelemetryConfig ProductTelemetryConfig
	// HealthConfig specifies the health probe config.
	HealthConfig HealthConfig
	// MetricsConfig specifies the metrics config.
	MetricsConfig MetricsConfig
	// Plus indicates whether NGINX Plus is being used.
	Plus bool
	// ExperimentalFeatures indicates if experimental features are enabled.
	ExperimentalFeatures bool
	// InferenceExtension indicates if Gateway API Inference Extension support is enabled.
	InferenceExtension bool
	// SnippetsFilters indicates if SnippetsFilters are enabled.
	SnippetsFilters bool
	// Snippets indicates if Snippets are enabled. This will enable both SnippetsFilter and SnippetsPolicy APIs.
	Snippets bool
	// EndpointPickerDisableTLS indicates if TLS is disabled for EndpointPicker communication.
	EndpointPickerDisableTLS bool
	// EndpointPickerTLSSkipVerify indicates if secure verification is skipped for EndpointPicker communication.
	EndpointPickerTLSSkipVerify bool
}

// PLMStorageConfig contains the configuration for PLM storage service communication.
type PLMStorageConfig struct {
	// URL is the PLM storage service URL (HTTP or HTTPS).
	URL string
	// CredentialsSecretName is the name of the Secret containing S3 credentials.
	// The Secret should have the "seaweedfs_admin_secret" data field.
	CredentialsSecretName string
	// TLSCACertSecretName is the is the name of the Secret containing the CA certificate file for TLS verification.
	TLSCACertSecretName string
	// TLSClientSSLSecretName is the path to the client certificate file for mutual TLS.
	TLSClientSSLSecretName string
	// TLSInsecureSkipVerify skips TLS certificate verification (not recommended for production).
	TLSInsecureSkipVerify bool
}

// GatewayPodConfig contains information about this Pod.
type GatewayPodConfig struct {
	// ServiceName is the name of the Service that fronts this Pod.
	ServiceName string
	// Namespace is the namespace of this Pod.
	Namespace string
	// Name is the name of the Pod.
	Name string
	// UID is the UID of the Pod.
	UID string
	// InstanceName is the name used in the instance label.
	// Generally this will be the name of the Helm release.
	InstanceName string
	// Version is the running NGF version.
	Version string
	// Image is the image path of the Pod.
	Image string
}

// MetricsConfig specifies the metrics config.
type MetricsConfig struct {
	// Port is the port the metrics should be exposed on.
	Port int
	// Enabled is the flag for toggling metrics on or off.
	Enabled bool
	// Secure is the flag for toggling the metrics endpoint to https.
	Secure bool
}

// HealthConfig specifies the health probe config.
type HealthConfig struct {
	// Port is the port that the health probe server listens on.
	Port int
	// Enabled is the flag for toggling the health probe server on or off.
	Enabled bool
}

// LeaderElectionConfig contains the configuration for leader election.
type LeaderElectionConfig struct {
	// LockName holds the name of the leader election lock.
	LockName string
	// Identity is the unique name of the controller used for identifying the leader.
	Identity string
	// Enabled indicates whether leader election is enabled.
	Enabled bool
}

// ProductTelemetryConfig contains the configuration for collecting product telemetry.
type ProductTelemetryConfig struct {
	// Endpoint is the <host>:<port> of the telemetry service.
	Endpoint string
	// ReportPeriod is the period at which telemetry reports are sent.
	ReportPeriod time.Duration
	// EndpointInsecure controls if TLS should be used for the telemetry service.
	EndpointInsecure bool
	// Enabled is the flag for toggling the collection of product telemetry.
	Enabled bool
}

// UsageReportConfig contains the configuration for NGINX Plus usage reporting.
type UsageReportConfig struct {
	// SecretName is the name of the Secret containing the server credentials.
	SecretName string
	// ClientSSLSecretName is the name of the Secret containing client certificate/key.
	ClientSSLSecretName string
	// CASecretName is the name of the Secret containing the CA certificate.
	CASecretName string
	// Endpoint is the endpoint of the reporting server.
	Endpoint string
	// Resolver is the nameserver for resolving the Endpoint.
	Resolver string
	// SkipVerify controls whether the nginx verifies the server certificate.
	SkipVerify bool
	// EnforceInitialReport controls whether the initial NGINX Plus licensing report is enforced.
	EnforceInitialReport bool
}

// Flags contains the NGF command-line flag names and values.
// Flag Names and Values are paired based off of index in slice.
type Flags struct {
	// Names contains the name of the flag.
	Names []string
	// Values contains the value of the flag in string form.
	// Each Value will be either true or false for boolean flags and default or user-defined for non-boolean flags.
	Values []string
}

// NginxOneConsoleTelemetryConfig contains the configuration for NGINX One Console telemetry.
type NginxOneConsoleTelemetryConfig struct {
	// DataplaneKeySecretName is the name of the Secret containing the dataplane key.
	DataplaneKeySecretName string
	// EndpointHost is the host of the telemetry endpoint.
	EndpointHost string
	// EndpointPort is the port of the telemetry endpoint.
	EndpointPort int
	// EndpointTLSSkipVerify specifies whether to skip TLS verification for the telemetry endpoint.
	EndpointTLSSkipVerify bool
}

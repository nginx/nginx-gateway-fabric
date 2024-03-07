package config

import (
	"time"

	"github.com/go-logr/logr"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
)

type Config struct {
	// AtomicLevel is an atomically changeable, dynamic logging level.
	AtomicLevel zap.AtomicLevel
	// UsageReportConfig specifies the NGINX Plus usage reporting config.
	UsageReportConfig *UsageReportConfig
	// Version is the running NGF version.
	Version string
	// ImageSource is the source of the NGINX Gateway image.
	ImageSource string
	// Flags contains the NGF command-line flag names and values.
	Flags Flags
	// GatewayNsName is the namespaced name of a Gateway resource that the Gateway will use.
	// The Gateway will ignore all other Gateway resources.
	GatewayNsName *types.NamespacedName
	// GatewayPodConfig contains information about this Pod.
	GatewayPodConfig GatewayPodConfig
	// Logger is the Zap Logger used by all components.
	Logger logr.Logger
	// GatewayCtlrName is the name of this controller.
	GatewayCtlrName string
	// ConfigName is the name of the NginxGateway resource for this controller.
	ConfigName string
	// GatewayClassName is the name of the GatewayClass resource that the Gateway will use.
	GatewayClassName string
	// LeaderElection contains the configuration for leader election.
	LeaderElection LeaderElectionConfig
	// ProductTelemetryConfig contains the configuration for collecting product telemetry.
	ProductTelemetryConfig ProductTelemetryConfig
	// MetricsConfig specifies the metrics config.
	MetricsConfig MetricsConfig
	// HealthConfig specifies the health probe config.
	HealthConfig HealthConfig
	// UpdateGatewayClassStatus enables updating the status of the GatewayClass resource.
	UpdateGatewayClassStatus bool
	// Plus indicates whether NGINX Plus is being used.
	Plus bool
	// ExperimentalFeatures indicates if experimental features are enabled.
	ExperimentalFeatures bool
}

// GatewayPodConfig contains information about this Pod.
type GatewayPodConfig struct {
	// PodIP is the IP address of this Pod.
	PodIP string
	// ServiceName is the name of the Service that fronts this Pod.
	ServiceName string
	// Namespace is the namespace of this Pod.
	Namespace string
	// Name is the name of the Pod.
	Name string
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
	// SecretNsName is the namespaced name of the Secret containing the server credentials.
	SecretNsName types.NamespacedName
	// ServerURL is the base URL of the reporting server.
	ServerURL string
	// ClusterDisplayName is the display name of the cluster. Optional.
	ClusterDisplayName string
	// InsecureSkipVerify controls whether the client verifies the server cert.
	InsecureSkipVerify bool
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

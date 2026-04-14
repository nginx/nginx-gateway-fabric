package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:resource:categories=nginx-gateway-fabric,shortName=wgbpolicy
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:metadata:labels="gateway.networking.k8s.io/policy=inherited"

// WAFGatewayBindingPolicy is an Inherited Attached Policy. It provides a way to configure F5 WAF for NGINX
// for Gateways and Routes by referencing compiled WAF policy bundles. Bundles can be fetched directly from an
// HTTP/HTTPS URL (type: HTTP), from an NGINX Instance Manager instance (type: NIM), or from an F5 NGINX One
// Console instance (type: N1C).
type WAFGatewayBindingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the WAFGatewayBindingPolicy.
	Spec WAFGatewayBindingPolicySpec `json:"spec"`

	// Status defines the state of the WAFGatewayBindingPolicy.
	Status gatewayv1.PolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WAFGatewayBindingPolicyList contains a list of WAFGatewayBindingPolicies.
type WAFGatewayBindingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WAFGatewayBindingPolicy `json:"items"`
}

// WAFGatewayBindingPolicySpec defines the desired state of a WAFGatewayBindingPolicy.
//
// +kubebuilder:validation:XValidation:message="policySource.httpSource must be set if and only if type is HTTP",rule="(self.type == 'HTTP') == (has(self.policySource) && has(self.policySource.httpSource))"
// +kubebuilder:validation:XValidation:message="policySource.nimSource must be set if and only if type is NIM",rule="(self.type == 'NIM') == (has(self.policySource) && has(self.policySource.nimSource))"
// +kubebuilder:validation:XValidation:message="policySource.n1cSource must be set if and only if type is N1C",rule="(self.type == 'N1C') == (has(self.policySource) && has(self.policySource.n1cSource))"
// +kubebuilder:validation:XValidation:message="policySource.validation.verifyChecksum is only supported for type HTTP",rule="!(self.type != 'HTTP' && has(self.policySource) && has(self.policySource.validation) && has(self.policySource.validation.verifyChecksum) && self.policySource.validation.verifyChecksum)"
//
//nolint:lll
type WAFGatewayBindingPolicySpec struct {
	// TargetRefs identifies API object(s) to apply the policy to.
	// Objects must be in the same namespace as the policy.
	// All targets must be of the same Kind (all Gateways OR all HTTPRoutes OR all GRPCRoutes).
	// Support: Gateway, HTTPRoute, GRPCRoute.
	//
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	// +kubebuilder:validation:XValidation:message="All TargetRefs must be the same Kind",rule="self.all(t1, self.all(t2, t1.kind == t2.kind))"
	// +kubebuilder:validation:XValidation:message="TargetRef Kind must be one of: Gateway, HTTPRoute, or GRPCRoute",rule="self.all(t, t.kind=='Gateway' || t.kind=='HTTPRoute' || t.kind=='GRPCRoute')"
	// +kubebuilder:validation:XValidation:message="TargetRef Group must be gateway.networking.k8s.io",rule="self.all(t, t.group=='gateway.networking.k8s.io')"
	// +kubebuilder:validation:XValidation:message="TargetRef Name must be unique",rule="self.all(t1, self.exists_one(t2, t1.name == t2.name))"
	//nolint:lll
	TargetRefs []gatewayv1.LocalPolicyTargetReference `json:"targetRefs"`

	// Type identifies the source type for the policy bundle.
	// HTTP fetches directly from a URL; NIM uses the NGINX Instance Manager bundles API;
	// N1C uses the F5 NGINX One Console security policies API.
	Type PolicySourceType `json:"type"`

	// PolicySource holds all policy bundle fetch configuration.
	PolicySource PolicySource `json:"policySource"`

	// SecurityLogs defines security logging configurations.
	//
	// +optional
	// +kubebuilder:validation:MaxItems=32
	SecurityLogs []WAFSecurityLog `json:"securityLogs,omitempty"`
}

// PolicySourceType identifies the source type for a WAF bundle.
//
// +kubebuilder:validation:Enum=HTTP;NIM;N1C
type PolicySourceType string

const (
	// PolicySourceTypeHTTP fetches a compiled .tgz bundle directly from an HTTP/HTTPS URL.
	PolicySourceTypeHTTP PolicySourceType = "HTTP"

	// PolicySourceTypeNIM fetches a compiled bundle from the NGINX Instance Manager security policies API.
	PolicySourceTypeNIM PolicySourceType = "NIM"

	// PolicySourceTypeN1C fetches a compiled bundle from the F5 NGINX One Console security policies API.
	// Requires managedSource.n1cNamespace in addition to managedSource.policyName.
	// Authentication uses the APIToken scheme: the "token" key from the referenced Secret is sent as
	// "Authorization: APIToken <token>".
	PolicySourceTypeN1C PolicySourceType = "N1C"
)

// PolicySource holds all configuration for fetching a WAF policy bundle.
type PolicySource struct {
	// HTTPSource configures direct bundle fetching from an HTTP/HTTPS URL.
	// Required when type is HTTP; must not be set for other types.
	//
	// +optional
	HTTPSource *HTTPBundleSource `json:"httpSource,omitempty"`

	// NIMSource configures bundle fetching from NGINX Instance Manager.
	// Required when type is NIM; must not be set for other types.
	//
	// +optional
	NIMSource *NIMBundleSource `json:"nimSource,omitempty"`

	// N1CSource configures bundle fetching from F5 NGINX One Console.
	// Required when type is N1C; must not be set for other types.
	//
	// +optional
	N1CSource *N1CBundleSource `json:"n1cSource,omitempty"`

	// Auth configures authentication credentials for fetching the bundle.
	//
	// +optional
	Auth *BundleAuth `json:"auth,omitempty"`

	// TLSSecretRef references a Secret containing a custom CA certificate (key: "ca.crt") for
	// verifying the bundle server's TLS certificate.
	//
	// +optional
	TLSSecretRef *LocalObjectReference `json:"tlsSecret,omitempty"`

	// Validation configures integrity verification for the downloaded bundle.
	//
	// +optional
	Validation *BundleValidation `json:"validation,omitempty"`

	// Polling configures automatic periodic re-fetching of the bundle.
	//
	// +optional
	Polling *BundlePolling `json:"polling,omitempty"`

	// Timeout is the maximum duration for a single bundle fetch attempt.
	// Defaults to 30s when not set.
	//
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// RetryPolicy configures retry behavior for transient fetch failures during the
	// initial bundle fetch.
	//
	// +optional
	RetryPolicy *BundleRetryPolicy `json:"retryPolicy,omitempty"`

	// InsecureSkipVerify disables TLS certificate verification when fetching the bundle.
	// Not recommended for production use.
	//
	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

// BundleValidation configures integrity verification for a bundle.
// Exactly one of verifyChecksum or expectedChecksum may be set.
//
// +kubebuilder:validation:XValidation:message="verifyChecksum and expectedChecksum are mutually exclusive",rule="!(has(self.verifyChecksum) && self.verifyChecksum && has(self.expectedChecksum))"
//
//nolint:lll
type BundleValidation struct {
	// ExpectedChecksum is the expected SHA256 checksum of the bundle.
	// If set, the downloaded bundle must match this checksum or it will be rejected.
	//
	// +optional
	// +kubebuilder:validation:MinLength=64
	// +kubebuilder:validation:MaxLength=64
	// +kubebuilder:validation:Pattern=`^[0-9a-fA-F]{64}$`
	ExpectedChecksum *string `json:"expectedChecksum,omitempty"`

	// VerifyChecksum enables automatic checksum verification by fetching a companion
	// checksum file at <url>.sha256 and comparing it against the downloaded bundle.
	// Only supported when the policy source type is HTTP (policySource.httpSource or
	// logSource.url); setting this for NIM or N1C sources is rejected at admission.
	// Mutually exclusive with expectedChecksum.
	//
	// +optional
	VerifyChecksum bool `json:"verifyChecksum,omitempty"`
}

// BundlePolling configures automatic re-fetching of a bundle.
type BundlePolling struct {
	// Interval is the period between poll cycles.
	// Defaults to 5m when polling is enabled but no interval is set.
	//
	// +optional
	Interval *metav1.Duration `json:"interval,omitempty"`

	// Enabled activates periodic re-fetching of the bundle.
	// When true, NGF fetches the bundle on each interval and deploys it only if
	// its checksum differs from the last successfully fetched version.
	//
	// +optional
	Enabled bool `json:"enabled,omitempty"`
}

// BundleRetryPolicy configures retry behavior on bundle fetch failures.
type BundleRetryPolicy struct {
	// Attempts is the maximum number of fetch attempts before giving up.
	//
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	Attempts *int32 `json:"attempts,omitempty"`
}

// HTTPBundleSource configures direct bundle fetching from an HTTP/HTTPS URL.
type HTTPBundleSource struct {
	// URL is the full URL of the compiled policy bundle (.tgz),
	// e.g. "https://storage.example.com/bundles/policy.tgz".
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=2083
	// +kubebuilder:validation:Pattern=`^https?://`
	URL string `json:"url"`
}

// NIMBundleSource configures bundle fetching from NGINX Instance Manager (NIM).
// Exactly one of policyName or policyUID must be set.
//
// +kubebuilder:validation:XValidation:message="exactly one of policyName or policyUID must be set",rule="(has(self.policyName) && !has(self.policyUID)) || (!has(self.policyName) && has(self.policyUID))"
//
//nolint:lll
type NIMBundleSource struct {
	// PolicyName is the name of the compiled policy bundle in NIM.
	// Mutually exclusive with policyUID.
	//
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	PolicyName *string `json:"policyName,omitempty"`

	// PolicyUID is the unique identifier of the compiled policy bundle in NIM.
	// Mutually exclusive with policyName.
	// Must be a valid UUID (e.g. "2bc1e3ac-7990-4ca4-910a-8634c444c804").
	//
	// +optional
	// +kubebuilder:validation:Pattern=`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
	PolicyUID *string `json:"policyUID,omitempty"`

	// URL is the base URL of the NGINX Instance Manager instance,
	// e.g. "https://nim.example.com".
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=2083
	// +kubebuilder:validation:Pattern=`^https?://`
	URL string `json:"url"`
}

// NIMLogProfileBundleSource configures log profile bundle fetching from NGINX Instance Manager (NIM).
type NIMLogProfileBundleSource struct {
	// ProfileName is the name of the compiled log profile bundle in NIM.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	ProfileName string `json:"profileName"`

	// URL is the base URL of the NGINX Instance Manager instance,
	// e.g. "https://nim.example.com".
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=2083
	// +kubebuilder:validation:Pattern=`^https?://`
	URL string `json:"url"`
}

// N1CBundleSource configures bundle fetching from F5 NGINX One Console (N1C).
// Exactly one of policyName or policyObjectID must be set.
//
// +kubebuilder:validation:XValidation:message="exactly one of policyName or policyObjectID must be set",rule="(has(self.policyName) && !has(self.policyObjectID)) || (!has(self.policyName) && has(self.policyObjectID))"
//
//nolint:lll
type N1CBundleSource struct {
	// PolicyName is the name of the security policy in N1C.
	// Mutually exclusive with policyObjectID.
	//
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	PolicyName *string `json:"policyName,omitempty"`

	// PolicyObjectID is the unique object identifier of the security policy in N1C
	// (e.g. "pol_-IUuEUN7ST63oRC7AlQPLw").
	// Mutually exclusive with policyName.
	//
	// +optional
	// +kubebuilder:validation:Pattern=`^pol_[A-Za-z0-9_-]+$`
	PolicyObjectID *string `json:"policyObjectID,omitempty"`

	// PolicyVersionID pins a specific version of the policy bundle using its opaque version ID
	// (e.g. "pv_UJ2gL5fOQ3Gnb3OVuVo1XA"). When omitted, the latest available version is used.
	//
	// +optional
	// +kubebuilder:validation:Pattern=`^pv_[A-Za-z0-9_-]+$`
	PolicyVersionID *string `json:"policyVersionID,omitempty"`

	// URL is the base URL of the F5 NGINX One Console instance,
	// e.g. "https://<tenant>.volterra.us".
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=2083
	// +kubebuilder:validation:Pattern=`^https?://`
	URL string `json:"url"`

	// Namespace is the NGINX One Console namespace that owns the security policy.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Namespace string `json:"namespace"`
}

// BundleAuth configures authentication for bundle fetching.
type BundleAuth struct {
	// SecretRef references a Kubernetes Secret in the same namespace as the WAFGatewayBindingPolicy.
	// The Secret may contain:
	//   - "username" and "password" fields for HTTP Basic Authentication
	//   - "token" field for Bearer Token Authentication (NIM) or APIToken Authentication (N1C)
	SecretRef LocalObjectReference `json:"secretRef"`
}

// DefaultLogProfile identifies a built-in WAF log profile bundle.
//
// +kubebuilder:validation:Enum=log_default;log_all;log_illegal;log_blocked;log_grpc_all;log_grpc_blocked;log_grpc_illegal
//
//nolint:lll
type DefaultLogProfile string

const (
	// DefaultLogProfileDefault logs illegal events (equivalent to log_illegal).
	DefaultLogProfileDefault DefaultLogProfile = "log_default"
	// DefaultLogProfileAll logs all events.
	DefaultLogProfileAll DefaultLogProfile = "log_all"
	// DefaultLogProfileIllegal logs illegal events.
	DefaultLogProfileIllegal DefaultLogProfile = "log_illegal"
	// DefaultLogProfileBlocked logs blocked events.
	DefaultLogProfileBlocked DefaultLogProfile = "log_blocked"
	// DefaultLogProfileGRPCAll logs all gRPC events.
	DefaultLogProfileGRPCAll DefaultLogProfile = "log_grpc_all"
	// DefaultLogProfileGRPCBlocked logs blocked gRPC events.
	DefaultLogProfileGRPCBlocked DefaultLogProfile = "log_grpc_blocked"
	// DefaultLogProfileGRPCIllegal logs illegal gRPC events.
	DefaultLogProfileGRPCIllegal DefaultLogProfile = "log_grpc_illegal"
)

// WAFSecurityLog defines security logging configuration for app_protect_security_log directives.
// Exactly one of logSource.defaultProfile, logSource.httpSource, or logSource.nimSource must be set.
//

type WAFSecurityLog struct {
	// LogSource configures the log profile bundle source for this log entry.
	// Exactly one of url or defaultProfile must be set.
	LogSource LogSource `json:"logSource"`

	// Destination defines where security logs are sent.
	Destination SecurityLogDestination `json:"destination"`
}

// LogSource holds all configuration for fetching a WAF log profile bundle.
// Exactly one of DefaultProfile, HTTPSource, or NIMSource must be set.
//
// +kubebuilder:validation:XValidation:message="exactly one of logSource.defaultProfile, logSource.httpSource, or logSource.nimSource must be set",rule="(has(self.defaultProfile) && !has(self.httpSource) && !has(self.nimSource)) || (!has(self.defaultProfile) && has(self.httpSource) && !has(self.nimSource)) || (!has(self.defaultProfile) && !has(self.httpSource) && has(self.nimSource))"
//
//nolint:lll
type LogSource struct {
	// DefaultProfile selects one of the built-in WAF log profile bundles shipped with the WAF engine.
	// Mutually exclusive with HTTPSource and NIMSource.
	//
	// +optional
	DefaultProfile *DefaultLogProfile `json:"defaultProfile,omitempty"`

	// HTTPSource configures direct bundle fetching from an HTTP/HTTPS URL.
	// Mutually exclusive with DefaultProfile and NIMSource.
	//
	// +optional
	HTTPSource *HTTPBundleSource `json:"httpSource,omitempty"`

	// NIMSource configures bundle fetching from NGINX Instance Manager.
	// Mutually exclusive with DefaultProfile and HTTPSource.
	//
	// +optional
	NIMSource *NIMLogProfileBundleSource `json:"nimSource,omitempty"`

	// Auth configures authentication credentials for fetching the log bundle.
	// Only applicable when url is set.
	//
	// +optional
	Auth *BundleAuth `json:"auth,omitempty"`

	// TLSSecretRef references a Secret containing a custom CA certificate (key: "ca.crt").
	// Only applicable when url is set.
	//
	// +optional
	TLSSecretRef *LocalObjectReference `json:"tlsSecret,omitempty"`

	// Validation configures integrity verification for the downloaded log bundle.
	// Only applicable when url is set.
	//
	// +optional
	Validation *BundleValidation `json:"validation,omitempty"`

	// Polling configures automatic periodic re-fetching of the log bundle.
	// Only applicable when url is set.
	//
	// +optional
	Polling *BundlePolling `json:"polling,omitempty"`

	// Timeout is the maximum duration for a single log bundle fetch attempt.
	// Defaults to 30s when not set. Only applicable when url is set.
	//
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// RetryPolicy configures retry behavior for transient fetch failures during the
	// initial log bundle fetch. Only applicable when url is set.
	//
	// +optional
	RetryPolicy *BundleRetryPolicy `json:"retryPolicy,omitempty"`

	// InsecureSkipVerify disables TLS certificate verification when fetching the bundle.
	// Not recommended for production use.
	//
	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

// SecurityLogDestination defines the destination for security logs.
//
// +kubebuilder:validation:XValidation:message="destination.file must be set if and only if type is file",rule="(self.type == 'file') == has(self.file)"
// +kubebuilder:validation:XValidation:message="destination.syslog must be set if and only if type is syslog",rule="(self.type == 'syslog') == has(self.syslog)"
//
//nolint:lll
type SecurityLogDestination struct {
	// File defines the file destination configuration.
	// Only valid when type is "file".
	//
	// +optional
	File *SecurityLogFile `json:"file,omitempty"`

	// Syslog defines the syslog destination configuration.
	// Only valid when type is "syslog".
	//
	// +optional
	Syslog *SecurityLogSyslog `json:"syslog,omitempty"`

	// Type identifies the type of security log destination.
	//
	// +unionDiscriminator
	// +kubebuilder:default=stderr
	Type SecurityLogDestinationType `json:"type"`
}

// SecurityLogDestinationType defines the supported security log destination types.
//
// +kubebuilder:validation:Enum=stderr;file;syslog
type SecurityLogDestinationType string

const (
	// SecurityLogDestinationTypeStderr outputs logs to container stderr.
	SecurityLogDestinationTypeStderr SecurityLogDestinationType = "stderr"
	// SecurityLogDestinationTypeFile writes logs to a specified file path.
	SecurityLogDestinationTypeFile SecurityLogDestinationType = "file"
	// SecurityLogDestinationTypeSyslog sends logs to a syslog server via TCP.
	SecurityLogDestinationTypeSyslog SecurityLogDestinationType = "syslog"
)

// SecurityLogFile defines the file destination configuration for security logs.
type SecurityLogFile struct {
	// Path is the file path where security logs will be written.
	// Must be accessible to the waf-enforcer container.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=256
	// +kubebuilder:validation:Pattern=`^/.*$`
	Path string `json:"path"`
}

// SecurityLogSyslog defines the syslog destination configuration for security logs.
type SecurityLogSyslog struct {
	// Server is the syslog server address in the format "host:port".
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9.-]+:[0-9]+$`
	Server string `json:"server"`
}

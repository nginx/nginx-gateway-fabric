// Package v1 contains lightweight Go structs for type-safe parsing of PLM-managed
// APPolicy and APLogConf status sub-resources. These are NOT controller-gen managed CRD types;
// they exist only for JSON deserialization from unstructured objects.
package v1

const (
	// Group is the API group for PLM CRDs.
	Group = "appprotect.f5.com"
	// Version is the API version for PLM CRDs.
	Version = "v1"

	// APPolicyKind is the Kind for the APPolicy CRD.
	APPolicyKind = "APPolicy"
	// APLogConfKind is the Kind for the APLogConf CRD.
	APLogConfKind = "APLogConf"

	// BundleStateReady indicates the bundle has been compiled successfully and is available for download.
	BundleStateReady = "ready"
	// BundleStatePending indicates the bundle compilation has not yet started.
	BundleStatePending = "pending"
	// BundleStateProcessing indicates the bundle is currently being compiled.
	BundleStateProcessing = "processing"
	// BundleStateInvalid indicates the policy/log conf is invalid and cannot be compiled.
	BundleStateInvalid = "invalid"
)

// APPolicyStatus is the status sub-resource of an APPolicy CRD.
type APPolicyStatus struct {
	Bundle *BundleStatus `json:"bundle,omitempty"`
}

// APLogConfStatus is the status sub-resource of an APLogConf CRD.
type APLogConfStatus struct {
	Bundle *BundleStatus `json:"bundle,omitempty"`
}

// BundleStatus describes the compiled bundle for a PLM resource.
type BundleStatus struct {
	// State is the compilation state (ready, pending, processing, invalid).
	State string `json:"state"`
	// Location is the S3 URI where the compiled bundle can be downloaded,
	// e.g. "s3://bucket/path/bundle.tgz".
	Location string `json:"location,omitempty"`
	// SHA256 is the hex-encoded SHA-256 checksum of the bundle.
	SHA256 string `json:"sha256,omitempty"`
	// CompilerVersion is the version of the compiler that produced the bundle.
	CompilerVersion string `json:"compilerVersion,omitempty"`
	// ObservedGeneration is the .metadata.generation of the resource that was compiled.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

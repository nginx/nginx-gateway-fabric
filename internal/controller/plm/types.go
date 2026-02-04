// Package plm provides types and utilities for integrating with the F5 Policy Lifecycle Manager (PLM).
// PLM manages APPolicy and APLogConf CRDs, which NGF watches for compiled bundle information.
package plm

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

// BundleStatus contains the bundle information from APPolicy/APLogConf status.
// Based on the actual PLM CRD status structure: status.bundle.*.
type BundleStatus struct {
	// State is the current bundle state (pending, processing, ready, invalid).
	State string
	// Location is the path/URL where the compiled bundle is stored; only set when State == "ready".
	Location string
	// Sha256 is the SHA256 hash of the bundle file.
	Sha256 string
}

// ProcessingStatus contains the compiler/validation metadata from APPolicy/APLogConf status.
// Based on the actual PLM CRD status structure: status.processing.*.
type ProcessingStatus struct {
	// Datetime is when the last compile/validation occurred.
	Datetime string
	// Errors holds any validation or compile errors (only if State == "invalid").
	Errors []string
	// IsCompiled is true if we compiled the bundle; false means we accepted a pre-compiled one.
	IsCompiled bool
}

// APPolicyStatus contains the relevant status fields extracted from an APPolicy CRD.
// NGF watches APPolicy status to determine when compiled bundles are available.
type APPolicyStatus struct {
	// Bundle holds the "ready/pending/invalid" bundle info.
	Bundle BundleStatus
	// Processing holds the compiler/validation metadata.
	Processing ProcessingStatus
	// ObservedGeneration is the generation of the APPolicy that was last processed.
	ObservedGeneration int64
}

// APLogConfStatus contains the relevant status fields extracted from an APLogConf CRD.
// NGF watches APLogConf status to determine when compiled log profile bundles are available.
type APLogConfStatus struct {
	// Bundle holds the "ready/pending/invalid" bundle info.
	Bundle BundleStatus
	// Processing holds the compiler/validation metadata.
	Processing ProcessingStatus
	// ObservedGeneration is the generation of the APLogConf that was last processed.
	ObservedGeneration int64
}

// PLM Bundle State constants.
const (
	// StatePending indicates the bundle is pending compilation.
	StatePending = "pending"
	// StateProcessing indicates the bundle is being processed/compiled.
	StateProcessing = "processing"
	// StateReady indicates the bundle is compiled and ready for use.
	StateReady = "ready"
	// StateInvalid indicates the bundle failed validation or compilation.
	StateInvalid = "invalid"
)

// ExtractAPPolicyStatus extracts the relevant status fields from an unstructured APPolicy.
func ExtractAPPolicyStatus(obj *unstructured.Unstructured) (*APPolicyStatus, error) {
	status, found, err := unstructured.NestedMap(obj.Object, "status")
	if err != nil {
		return nil, err
	}
	if !found {
		return &APPolicyStatus{}, nil
	}

	result := &APPolicyStatus{}

	// Extract bundle info from status.bundle
	result.Bundle = extractBundleStatus(status)

	// Extract processing info from status.processing
	result.Processing = extractProcessingStatus(status)

	// Extract observedGeneration from status.observedGeneration
	if og, found, err := unstructured.NestedInt64(status, "observedGeneration"); err == nil && found {
		result.ObservedGeneration = og
	}

	return result, nil
}

// ExtractAPLogConfStatus extracts the relevant status fields from an unstructured APLogConf.
func ExtractAPLogConfStatus(obj *unstructured.Unstructured) (*APLogConfStatus, error) {
	status, found, err := unstructured.NestedMap(obj.Object, "status")
	if err != nil {
		return nil, err
	}
	if !found {
		return &APLogConfStatus{}, nil
	}

	result := &APLogConfStatus{}

	// Extract bundle info from status.bundle
	result.Bundle = extractBundleStatus(status)

	// Extract processing info from status.processing
	result.Processing = extractProcessingStatus(status)

	// Extract observedGeneration from status.observedGeneration
	if og, found, err := unstructured.NestedInt64(status, "observedGeneration"); err == nil && found {
		result.ObservedGeneration = og
	}

	return result, nil
}

// extractBundleStatus extracts the bundle status from a status map.
func extractBundleStatus(status map[string]any) BundleStatus {
	bundle := BundleStatus{}

	bundleMap, found, err := unstructured.NestedMap(status, "bundle")
	if err != nil || !found {
		return bundle
	}

	if state, found, err := unstructured.NestedString(bundleMap, "state"); err == nil && found {
		bundle.State = state
	}

	if location, found, err := unstructured.NestedString(bundleMap, "location"); err == nil && found {
		bundle.Location = location
	}

	if sha256, found, err := unstructured.NestedString(bundleMap, "sha256"); err == nil && found {
		bundle.Sha256 = sha256
	}

	return bundle
}

// extractProcessingStatus extracts the processing status from a status map.
func extractProcessingStatus(status map[string]any) ProcessingStatus {
	processing := ProcessingStatus{}

	processingMap, found, err := unstructured.NestedMap(status, "processing")
	if err != nil || !found {
		return processing
	}

	if datetime, found, err := unstructured.NestedString(processingMap, "datetime"); err == nil && found {
		processing.Datetime = datetime
	}

	if errors, found, err := unstructured.NestedStringSlice(processingMap, "errors"); err == nil && found {
		processing.Errors = errors
	}

	if isCompiled, found, err := unstructured.NestedBool(processingMap, "isCompiled"); err == nil && found {
		processing.IsCompiled = isCompiled
	}

	return processing
}

// NewAPPolicyUnstructured creates a new unstructured APPolicy for use with dynamic client.
func NewAPPolicyUnstructured() *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(kinds.APPolicyGVK)
	return obj
}

// NewAPLogConfUnstructured creates a new unstructured APLogConf for use with dynamic client.
func NewAPLogConfUnstructured() *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(kinds.APLogConfGVK)
	return obj
}

// NewAPPolicyListUnstructured creates a new unstructured APPolicyList for use with dynamic client.
func NewAPPolicyListUnstructured() *unstructured.UnstructuredList {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(kinds.APPolicyGVK)
	list.SetKind(kinds.APPolicy + "List")
	return list
}

// NewAPLogConfListUnstructured creates a new unstructured APLogConfList for use with dynamic client.
func NewAPLogConfListUnstructured() *unstructured.UnstructuredList {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(kinds.APLogConfGVK)
	list.SetKind(kinds.APLogConf + "List")
	return list
}

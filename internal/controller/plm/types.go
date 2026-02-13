// Package plm provides types and utilities for integrating with the F5 Policy Lifecycle Manager (PLM).
// PLM manages APPolicy and APLogConf CRDs, which NGF watches for compiled bundle information.
package plm

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

// BundleState represents the state of a compiled bundle in PLM.
type BundleState string

// BundleStatus contains the bundle information from APPolicy/APLogConf status.
// Based on the actual PLM CRD status structure: status.bundle.*.
type BundleStatus struct {
	// State is the current bundle state (pending, processing, ready, invalid).
	State BundleState
	// Location is the path/URL where the compiled bundle is stored; only set when State == StateReady.
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

// APResourceStatus contains the relevant status fields extracted from APPolicy or APLogConf CRDs.
// NGF watches these resources to determine when compiled bundles are available.
type APResourceStatus struct {
	// Bundle holds the "ready/pending/invalid" bundle info.
	Bundle BundleStatus
	// Processing holds the compiler/validation metadata.
	Processing ProcessingStatus
	// ObservedGeneration is the generation of the resource that was last processed.
	ObservedGeneration int64
}

// PLM Bundle State constants.
const (
	// StatePending indicates the bundle is pending compilation.
	StatePending BundleState = "pending"
	// StateProcessing indicates the bundle is being processed/compiled.
	StateProcessing BundleState = "processing"
	// StateReady indicates the bundle is compiled and ready for use.
	StateReady BundleState = "ready"
	// StateInvalid indicates the bundle failed validation or compilation.
	StateInvalid BundleState = "invalid"
)

// Field names for extracting data from unstructured APPolicy/APLogConf resources.
const (
	fieldStatus             = "status"
	fieldObservedGeneration = "observedGeneration"
	fieldBundle             = "bundle"
	fieldState              = "state"
	fieldLocation           = "location"
	fieldSha256             = "sha256"
	fieldProcessing         = "processing"
	fieldDatetime           = "datetime"
	fieldErrors             = "errors"
	fieldIsCompiled         = "isCompiled"
	suffixList              = "List"
)

// ExtractAPResourceStatus is the shared implementation for extracting status fields
// from both APPolicy and APLogConf unstructured resources.
func ExtractAPResourceStatus(obj *unstructured.Unstructured) (*APResourceStatus, error) {
	status, found, err := unstructured.NestedMap(obj.Object, fieldStatus)
	if err != nil {
		return nil, err
	}
	if !found {
		return &APResourceStatus{}, nil
	}

	result := &APResourceStatus{}

	// Extract bundle info from status.bundle
	result.Bundle = extractBundleStatus(status)

	// Extract processing info from status.processing
	result.Processing = extractProcessingStatus(status)

	// Extract observedGeneration from status.observedGeneration
	if og, found, err := unstructured.NestedInt64(status, fieldObservedGeneration); err == nil && found {
		result.ObservedGeneration = og
	}

	return result, nil
}

// extractBundleStatus extracts the bundle status from a status map.
func extractBundleStatus(status map[string]any) BundleStatus {
	bundle := BundleStatus{}

	bundleMap, found, err := unstructured.NestedMap(status, fieldBundle)
	if err != nil || !found {
		return bundle
	}

	if state, found, err := unstructured.NestedString(bundleMap, fieldState); err == nil && found {
		bundle.State = BundleState(state)
	}

	if location, found, err := unstructured.NestedString(bundleMap, fieldLocation); err == nil && found {
		bundle.Location = location
	}

	if sha256, found, err := unstructured.NestedString(bundleMap, fieldSha256); err == nil && found {
		bundle.Sha256 = sha256
	}

	return bundle
}

// extractProcessingStatus extracts the processing status from a status map.
func extractProcessingStatus(status map[string]any) ProcessingStatus {
	processing := ProcessingStatus{}

	processingMap, found, err := unstructured.NestedMap(status, fieldProcessing)
	if err != nil || !found {
		return processing
	}

	if datetime, found, err := unstructured.NestedString(processingMap, fieldDatetime); err == nil && found {
		processing.Datetime = datetime
	}

	if errors, found, err := unstructured.NestedStringSlice(processingMap, fieldErrors); err == nil && found {
		processing.Errors = errors
	}

	if isCompiled, found, err := unstructured.NestedBool(processingMap, fieldIsCompiled); err == nil && found {
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
	list.SetKind(kinds.APPolicy + suffixList)
	return list
}

// NewAPLogConfListUnstructured creates a new unstructured APLogConfList for use with dynamic client.
func NewAPLogConfListUnstructured() *unstructured.UnstructuredList {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(kinds.APLogConfGVK)
	list.SetKind(kinds.APLogConf + suffixList)
	return list
}

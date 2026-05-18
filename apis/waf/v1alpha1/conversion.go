package v1alpha1

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ParseAPPolicyStatus extracts a typed APPolicyStatus from an unstructured APPolicy object.
// Returns an error if the status cannot be parsed.
func ParseAPPolicyStatus(obj *unstructured.Unstructured) (*APPolicyStatus, error) {
	statusRaw, ok := obj.Object["status"]
	if !ok {
		return nil, fmt.Errorf("APPolicy %s/%s has no status", obj.GetNamespace(), obj.GetName())
	}

	data, err := json.Marshal(statusRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal APPolicy status: %w", err)
	}

	var status APPolicyStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("failed to unmarshal APPolicy status: %w", err)
	}

	return &status, nil
}

// ParseAPLogConfStatus extracts a typed APLogConfStatus from an unstructured APLogConf object.
// Returns an error if the status cannot be parsed.
func ParseAPLogConfStatus(obj *unstructured.Unstructured) (*APLogConfStatus, error) {
	statusRaw, ok := obj.Object["status"]
	if !ok {
		return nil, fmt.Errorf("APLogConf %s/%s has no status", obj.GetNamespace(), obj.GetName())
	}

	data, err := json.Marshal(statusRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal APLogConf status: %w", err)
	}

	var status APLogConfStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("failed to unmarshal APLogConf status: %w", err)
	}

	return &status, nil
}

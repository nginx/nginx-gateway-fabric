package v1

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ParseAPPolicyStatus extracts a typed APPolicyStatus from an unstructured APPolicy object.
// Returns an error if the status cannot be parsed.
func ParseAPPolicyStatus(obj *unstructured.Unstructured) (*APPolicyStatus, error) {
	var status APPolicyStatus
	if err := parseStatus(obj, "APPolicy", &status); err != nil {
		return nil, err
	}

	return &status, nil
}

// ParseAPLogConfStatus extracts a typed APLogConfStatus from an unstructured APLogConf object.
// Returns an error if the status cannot be parsed.
func ParseAPLogConfStatus(obj *unstructured.Unstructured) (*APLogConfStatus, error) {
	var status APLogConfStatus
	if err := parseStatus(obj, "APLogConf", &status); err != nil {
		return nil, err
	}

	return &status, nil
}

func parseStatus(
	obj *unstructured.Unstructured,
	resourceName string,
	out any,
) error {
	if err := validateTypeMeta(obj, resourceName); err != nil {
		return err
	}

	statusRaw, ok := obj.Object["status"]
	if !ok {
		return fmt.Errorf("%s %s/%s has no status", resourceName, obj.GetNamespace(), obj.GetName())
	}

	data, err := json.Marshal(statusRaw)
	if err != nil {
		return fmt.Errorf("failed to marshal %s status: %w", resourceName, err)
	}

	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("failed to unmarshal %s status: %w", resourceName, err)
	}

	return nil
}

func validateTypeMeta(obj *unstructured.Unstructured, resourceName string) error {
	expectedAPIVersion := fmt.Sprintf("%s/%s", Group, Version)
	if obj.GetAPIVersion() != expectedAPIVersion || obj.GetKind() != resourceName {
		return fmt.Errorf(
			"expected %s %s, got %s %s",
			expectedAPIVersion,
			resourceName,
			obj.GetAPIVersion(),
			obj.GetKind(),
		)
	}

	return nil
}

package controller

import (
	"fmt"
	"strings"
)

// inferencePoolServiceSuffix is the suffix of the headless Service name for an InferencePool.
const inferencePoolServiceSuffix = "-pool-svc"

// CreateNginxResourceName creates the base resource name for all nginx resources
// created by the control plane.
func CreateNginxResourceName(prefix, suffix string) string {
	return fmt.Sprintf("%s-%s", prefix, suffix)
}

// CreateInferencePoolServiceName creates the name for a headless Service that
// we create for an InferencePool.
func CreateInferencePoolServiceName(name string) string {
	return fmt.Sprintf("%s%s", name, inferencePoolServiceSuffix)
}

// GetInferencePoolName returns the name of the InferencePool for a given headless Service name.
func GetInferencePoolName(serviceName string) string {
	return strings.TrimSuffix(serviceName, inferencePoolServiceSuffix)
}

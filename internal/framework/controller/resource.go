package controller

import (
	"crypto/sha256"
	"encoding/hex"
)

const (
	// inferencePoolServiceSuffix is the suffix of the headless Service name for an InferencePool.
	inferencePoolServiceSuffix = "pool-svc"
	MaxServiceNameLen          = 63
	hashLen                    = 8
)

// CreateNginxResourceName creates the base resource name for all nginx resources
// created by the control plane.
func CreateNginxResourceName(prefix, suffix string) string {
	return truncateAndHashName(prefix, suffix)
}

// CreateInferencePoolServiceName creates the name for a headless Service that
// we create for an InferencePool.
func CreateInferencePoolServiceName(name string) string {
	return truncateAndHashName(name, inferencePoolServiceSuffix)
}

// truncateAndHashName truncates the input name to fit within maxLen,
// appending a hash for uniqueness if needed.
func truncateAndHashName(name string, suffix string) string {
	sep := "-"
	full := name + sep + suffix
	if len(full) <= MaxServiceNameLen {
		return full
	}

	// Always include the suffix, truncate name as needed
	hash := sha256.Sum256([]byte(full))
	hashStr := hex.EncodeToString(hash[:])[:hashLen]
	maxNameLen := MaxServiceNameLen - (len(sep) * 2) - hashLen - len(suffix)
	truncName := name
	if len(name) > maxNameLen {
		truncName = name[:maxNameLen]
	}

	return truncName + sep + hashStr + sep + suffix
}

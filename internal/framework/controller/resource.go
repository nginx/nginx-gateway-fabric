package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
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

	hash := sha256.Sum256([]byte(full))
	hashStr := hex.EncodeToString(hash[:])[:hashLen]

	// overhead = 2 separators + hash
	overhead := (len(sep) * 2) + hashLen
	budget := MaxServiceNameLen - overhead // chars available for name + suffix

	truncSuffix := suffix
	if len(suffix) >= budget {
		// Reserve at least 1 char for the name so the result doesn't start with "-".
		truncSuffix = suffix[:budget-1]
	}

	maxNameLen := budget - len(truncSuffix)
	truncName := name
	if len(name) > maxNameLen {
		truncName = name[:maxNameLen]
	}

	// Remove trailing dashes to avoid double-dash when separator is added.
	truncName = strings.TrimRight(truncName, sep)

	return truncName + sep + hashStr + sep + truncSuffix
}

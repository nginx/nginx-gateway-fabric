package shared //nolint:revive,nolintlint // ignoring meaningless package name

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	// DefaultZoneSizeMaxSize is the default maximum size, in bytes, that an automatically-sized
	// ("auto") upstream zone can grow to (512m).
	DefaultZoneSizeMaxSize = int64(512 * 1024 * 1024)

	// AutoStartZoneSizeBytes is the flat cold-start zone size, in bytes, used for every profile
	// when ZoneSize is explicitly set to "auto".
	AutoStartZoneSizeBytes = int64(64 * 1024)
)

// sizeRegex matches a size string: a number followed by an optional unit ('k', 'm', or 'g').
var sizeRegex = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*([kmg]?)$`)

// ParseSize parses a size string (e.g., "256k", "1m", "1g") to bytes.
func ParseSize(sizeStr string) (int64, error) {
	sizeStr = strings.TrimSpace(sizeStr)
	if sizeStr == "" {
		return 0, fmt.Errorf("empty size string")
	}

	matches := sizeRegex.FindStringSubmatch(sizeStr)
	if len(matches) != 3 {
		return 0, fmt.Errorf("invalid size format: %s (expected format like '256k', '1m', or '1g')", sizeStr)
	}

	numStr := matches[1]
	unit := matches[2]

	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number in size: %s", numStr)
	}

	bytes := int64(num)
	switch unit {
	case "k":
		bytes *= 1024
	case "m":
		bytes *= 1024 * 1024
	case "g":
		bytes *= 1024 * 1024 * 1024
	case "":
		// No unit, assume bytes
	}

	return bytes, nil
}

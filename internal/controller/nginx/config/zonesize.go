package config

import (
	"fmt"
)

// ZoneSizeProfile identifies a unique combination of NGINX variant (OSS or Plus) and upstream
// protocol (HTTP or Stream).
type ZoneSizeProfile uint8

const (
	HTTPOSS ZoneSizeProfile = iota
	HTTPPlus
	StreamOSS
	StreamPlus
)

// ZoneSizeAuto is the special ZoneSize value that enables automatic zone sizing.
const ZoneSizeAuto = "auto"

// HTTPProfile returns the ZoneSizeProfile for an HTTP upstream, given whether the NGINX variant
// is Plus.
func HTTPProfile(isPlus bool) ZoneSizeProfile {
	if isPlus {
		return HTTPPlus
	}
	return HTTPOSS
}

// StreamProfile returns the ZoneSizeProfile for a stream upstream, given whether the NGINX
// variant is Plus.
func StreamProfile(isPlus bool) ZoneSizeProfile {
	if isPlus {
		return StreamPlus
	}
	return StreamOSS
}

// defaultZoneSizeBytes holds the static default zone size, in bytes, for each profile, used for
// upstreams whose ZoneSize is left unset (nil). These restore the static values NGF used before
// automatic zone sizing was introduced:
//   - HTTP OSS: 512k
//   - HTTP Plus: 2m
//   - Stream OSS: 512k
//   - Stream Plus: 1m
//
// This table is immutable and shared across all calculator instances. It is NOT used for
// upstreams with ZoneSize explicitly set to "auto" -- see AutoStartZoneSizeBytes.
var defaultZoneSizeBytes = map[ZoneSizeProfile]int64{
	HTTPOSS:    512 * 1024,
	HTTPPlus:   2 * 1024 * 1024,
	StreamOSS:  512 * 1024,
	StreamPlus: 1 * 1024 * 1024,
}

// AutoStartZoneSizeBytes is the flat cold-start zone size, in bytes, used for every profile when
// ZoneSize is explicitly set to "auto".
const AutoStartZoneSizeBytes int64 = 64 * 1024

// ZoneSizeGrowthFactor is the multiplier applied to an upstream's zone size each time NGINX
// fails to reload because the zone is too small.
const ZoneSizeGrowthFactor = 2.0

// ZoneSizeCalculator resolves the zone size for an upstream, taking into account any explicit
// ZoneSize override, the "auto" opt-in, and any sizes previously learned via retry after an
// NGINX reload failure.
type ZoneSizeCalculator struct {
	// overrides holds the current effective size, in bytes, for upstreams using "auto" sizing
	// that have previously failed to reload at a smaller size. Keyed by upstream name.
	overrides map[string]int64
	// maxSize is the maximum zone size, in bytes, that "auto" sizing is allowed to grow to.
	maxSize int64
}

// NewZoneSizeCalculator creates a new zone size calculator.
func NewZoneSizeCalculator(overrides map[string]int64, maxSize int64) *ZoneSizeCalculator {
	return &ZoneSizeCalculator{
		overrides: overrides,
		maxSize:   maxSize,
	}
}

// Resolve returns the zone size string (e.g. "512k", "2m") to use for the given upstream.
func (z *ZoneSizeCalculator) Resolve(upstreamName string, explicit *string, profile ZoneSizeProfile) string {
	// If explicit is nil, use the static per-profile default.
	if explicit == nil {
		return bytesToString(defaultZoneSizeBytes[profile])
	}

	// If explicit is not "auto", return it verbatim as a static, user-specified size.
	if *explicit != ZoneSizeAuto {
		return *explicit
	}

	// If explicit is "auto", check if we have a previously-learned override for this upstream.
	if size, ok := z.overrides[upstreamName]; ok {
		return bytesToString(size)
	}

	// Otherwise, use the flat auto-start size.
	return bytesToString(AutoStartZoneSizeBytes)
}

// IsAuto returns true if explicit requests automatic zone sizing.
func IsAuto(explicit *string) bool {
	return explicit != nil && *explicit == ZoneSizeAuto
}

// CurrentSizeBytes returns the current effective zone size, in bytes, for the given upstream.
func (z *ZoneSizeCalculator) CurrentSizeBytes(upstreamName string) int64 {
	if size, ok := z.overrides[upstreamName]; ok {
		return size
	}

	return AutoStartZoneSizeBytes
}

// NextSize returns the next size, in bytes, to try for an upstream whose zone was reported as
// too small at currentSize.
func (z *ZoneSizeCalculator) NextSize(currentSize int64) (int64, bool) {
	if currentSize >= z.maxSize {
		return currentSize, false
	}

	next := int64(float64(currentSize) * ZoneSizeGrowthFactor)
	if next > z.maxSize {
		next = z.maxSize
	}
	if next <= currentSize {
		return currentSize, false
	}

	return next, true
}

// PrevSize returns the previous (halved) size, in bytes, to shrink an upstream's zone to when
// its endpoint count has dropped well below what currentSize was sized for.
func (z *ZoneSizeCalculator) PrevSize(currentSize int64) (int64, bool) {
	if currentSize <= AutoStartZoneSizeBytes {
		return currentSize, false
	}

	prev := int64(float64(currentSize) / ZoneSizeGrowthFactor)
	if prev < AutoStartZoneSizeBytes {
		prev = AutoStartZoneSizeBytes
	}
	if prev >= currentSize {
		return currentSize, false
	}

	return prev, true
}

// bytesToString converts bytes to a human-readable size string, rounding up to nearest k.
// Examples: 256,000 bytes -> "256k", 1,048,576 bytes -> "1m".
func bytesToString(bytes int64) string {
	const (
		kilo = 1024
		mega = 1024 * 1024
	)

	// Convert to kilobytes and round up to nearest k
	kb := (bytes + kilo - 1) / kilo // Ceiling division

	if kb >= 1024 {
		// If >= 1024k, prefer megabytes
		mb := (kb + 512) / 1024 // Round to nearest m (512k = 0.5m)
		if mb*mega >= bytes {
			return fmt.Sprintf("%dm", mb)
		}
	}

	// Return in kilobytes
	return fmt.Sprintf("%dk", kb)
}

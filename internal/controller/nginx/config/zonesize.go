package config

import (
	"fmt"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/shared"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
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

// bytesPerEndpoint holds empirical bytes-per-endpoint zone sizing data derived from NGINX
// documentation on maximum upstream servers per zone size:
//   - HTTP OSS: 512k supports 648 servers => ~809 bytes/server
//   - HTTP Plus: 2m supports 545 servers => ~3847 bytes/server
//   - Stream OSS: 512k supports 576 servers => ~910 bytes/server
//   - Stream Plus: 1m supports 991 servers => ~1058 bytes/server
//
// This table is immutable and shared across all calculator instances.
var bytesPerEndpoint = map[ZoneSizeProfile]float64{
	HTTPOSS:    809.0,
	HTTPPlus:   3847.0,
	StreamOSS:  910.0,
	StreamPlus: 1058.0,
}

// ZoneSizeCalculator computes optimal upstream zone sizes based on endpoint count.
type ZoneSizeCalculator struct {
	bufferMultiplier float64
	minSize          int64
	maxSize          int64
}

// ZoneSizeCalculatorConfig holds configuration for zone size calculation.
type ZoneSizeCalculatorConfig struct {
	// BufferMultiplier is the growth safety margin applied to calculated sizes.
	// Default: 1.25 (25% buffer). Must be >= 1.0.
	BufferMultiplier float64

	// MinSize is the minimum zone size in bytes. Default: 128k (131,072 bytes).
	MinSize int64

	// MaxSize is the maximum zone size in bytes. Default: 512m (536,870,912 bytes).
	MaxSize int64
}

// DefaultZoneSizeCalculatorConfig returns a ZoneSizeCalculatorConfig with sensible defaults.
func DefaultZoneSizeCalculatorConfig() ZoneSizeCalculatorConfig {
	return ZoneSizeCalculatorConfig{
		BufferMultiplier: shared.DefaultZoneSizeBufferMultiplier,
		MinSize:          shared.DefaultZoneSizeMinSize,
		MaxSize:          shared.DefaultZoneSizeMaxSize,
	}
}

// zoneSizeCalculatorConfigFromDataplane resolves a ZoneSizeCalculatorConfig from the dataplane's
// UpstreamZoneAutoSizing settings.
func zoneSizeCalculatorConfigFromDataplane(a dataplane.UpstreamZoneAutoSizing) ZoneSizeCalculatorConfig {
	cfg := DefaultZoneSizeCalculatorConfig()

	if a.BufferMultiplier > 0 {
		cfg.BufferMultiplier = a.BufferMultiplier
	}
	if a.MinSize > 0 {
		cfg.MinSize = a.MinSize
	}
	if a.MaxSize > 0 {
		cfg.MaxSize = a.MaxSize
	}

	return cfg
}

// NewZoneSizeCalculator creates a new zone size calculator.
func NewZoneSizeCalculator(config ZoneSizeCalculatorConfig) *ZoneSizeCalculator {
	return &ZoneSizeCalculator{
		bufferMultiplier: config.BufferMultiplier,
		minSize:          config.MinSize,
		maxSize:          config.MaxSize,
	}
}

// Calculate returns the calculated zone size as a string (e.g., "256k", "1m") for the given
// endpoint count and ZoneSizeProfile (NGINX variant + upstream protocol).
// The calculation is: (endpoints * bytes_per_endpoint) * buffer, clamped to [min, max], rounded to nearest k.
// A zero or negative endpointCount naturally clamps to the configured minimum size, the same as
// any endpoint count whose calculated size falls below it.
func (z *ZoneSizeCalculator) Calculate(endpointCount int, profile ZoneSizeProfile) string {
	// Calculate: endpoints * bytes_per_endpoint * buffer
	calculated := float64(endpointCount) * bytesPerEndpoint[profile] * z.bufferMultiplier

	// Clamp to [min, max]
	calculatedInt := int64(calculated)
	if calculatedInt < z.minSize {
		calculatedInt = z.minSize
	}
	if calculatedInt > z.maxSize {
		calculatedInt = z.maxSize
	}

	return z.bytesToString(calculatedInt)
}

// bytesToString converts bytes to a human-readable size string, rounding up to nearest k.
// Examples: 256,000 bytes -> "256k", 1,048,576 bytes -> "1m".
func (z *ZoneSizeCalculator) bytesToString(bytes int64) string {
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

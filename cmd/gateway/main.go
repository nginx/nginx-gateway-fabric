package main

import (
	"fmt"
	"os"
)

// Set during go build.
var (
	version string

	// telemetryReportPeriod is the period at which telemetry reports are sent.
	telemetryReportPeriod string
	// telemetryEndpoint is the endpoint to which telemetry reports are sent.
	telemetryEndpoint string
	// telemetryEndpointInsecure controls whether TLS should be used when sending telemetry reports.
	telemetryEndpointInsecure string
)

func main() {
	rootCmd := createRootCommand()

	rootCmd.AddCommand(
		createControllerCommand(),
		createGenerateCertsCommand(),
		createInitializeCommand(),
		createSleepCommand(),
	)

	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
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

// TEMPORARY CODE TO VERIFY SECURITY WORKFLOW
func handler(w http.ResponseWriter, r *http.Request) {
	// Get user input from the query parameter "cmd"
	cmd := r.URL.Query().Get("cmd")

	// Vulnerable code: directly concatenates user input into an OS command
	output, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		fmt.Println("Error executing command:", err)
		return
	}

	// Output the result to the client
	fmt.Fprintf(w, "Command output: %s", string(output))
}

func main() {
	http.HandleFunc("/", handler)

	fmt.Println("Server started on :8080")
	http.ListenAndServe(":8080", nil)

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

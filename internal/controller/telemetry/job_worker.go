package telemetry

import (
	"context"

	"github.com/go-logr/logr"
)

//counterfeiter:generate . DataCollector

// DataCollector collects telemetry data.
type DataCollector interface {
	// Collect collects and returns telemetry Data.
	Collect(ctx context.Context) (Data, error)
}

func CreateTelemetryJobWorker(
	logger logr.Logger,
	exporter Exporter,
	dataCollector DataCollector,
) func(ctx context.Context) {
	return func(ctx context.Context) {
		// Gather telemetry
		logger.V(1).Info("Gathering telemetry data")

		// We will need to gather data as defined in https://github.com/nginx/nginx-gateway-fabric/issues/793
		data, err := dataCollector.Collect(ctx)
		if err != nil {
			logger.Error(err, "Failed to collect telemetry data")
			return
		}

		// Export telemetry
		logger.V(1).Info("Exporting telemetry data")

		if err := exporter.Export(ctx, &data); err != nil {
			logger.Error(err, "Failed to export telemetry data")
		}
	}
}

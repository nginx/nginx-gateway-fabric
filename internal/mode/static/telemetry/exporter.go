package telemetry

import (
	"context"

	"github.com/go-logr/logr"
	tel "github.com/nginx/telemetry-exporter/pkg/telemetry"
)

// Exporter exports telemetry data to some destination.
//
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate . Exporter
type Exporter interface {
	Export(ctx context.Context, data tel.Exportable) error
}

// LoggingExporter logs telemetry data.
type LoggingExporter struct {
	logger logr.Logger
}

// NewLoggingExporter creates a new LoggingExporter.
func NewLoggingExporter(logger logr.Logger) *LoggingExporter {
	return &LoggingExporter{
		logger: logger,
	}
}

// Export logs the provided telemetry data.
func (e *LoggingExporter) Export(_ context.Context, data tel.Exportable) error {
	e.logger.Info("Exporting telemetry", "data", data)
	return nil
}

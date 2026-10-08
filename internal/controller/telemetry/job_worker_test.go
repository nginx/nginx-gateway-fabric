package telemetry_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	tel "github.com/nginx/telemetry-exporter/pkg/telemetry"
	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/telemetry"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/telemetry/telemetryfakes"
)

func TestCreateTelemetryJobWorker_Succeeds(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	exporter := &telemetryfakes.ExporterMock{
		ExportFunc: func(context.Context, tel.Exportable) error {
			return nil
		},
	}
	dataCollector := &telemetryfakes.DataCollectorMock{}

	worker := telemetry.CreateTelemetryJobWorker(logr.Discard(), exporter, dataCollector)

	expData := telemetry.Data{
		Data: tel.Data{
			ProjectName: "NGF",
		},
	}
	dataCollector.CollectFunc = func(context.Context) (telemetry.Data, error) {
		return expData, nil
	}

	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()

	worker(ctx)
	g.Expect(exporter.ExportCalls()[0].Data).To(Equal(&expData))
}

func TestCreateTelemetryJobWorker_CollectFails(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	exporter := &telemetryfakes.ExporterMock{
		ExportFunc: func(context.Context, tel.Exportable) error {
			return nil
		},
	}
	dataCollector := &telemetryfakes.DataCollectorMock{}

	worker := telemetry.CreateTelemetryJobWorker(logr.Discard(), exporter, dataCollector)

	expData := telemetry.Data{}
	dataCollector.CollectFunc = func(context.Context) (telemetry.Data, error) {
		return expData, errors.New("failed to collect cluster information")
	}

	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()

	worker(ctx)
	g.Expect(exporter.ExportCalls()).To(BeEmpty())
}

package telemetry_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	tel "github.com/nginx/telemetry-exporter/pkg/telemetry"
	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/internal/controller/telemetry"
	"github.com/nginx/nginx-gateway-fabric/internal/controller/telemetry/telemetryfakes"
)

func TestCreateTelemetryJobWorker_Succeeds(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	exporter := &telemetryfakes.FakeExporter{}
	dataCollector := &telemetryfakes.FakeDataCollector{}

	worker := telemetry.CreateTelemetryJobWorker(logr.Discard(), exporter, dataCollector)

	expData := telemetry.Data{
		Data: tel.Data{
			ProjectName: "NGF",
		},
	}
	dataCollector.CollectReturns(expData, nil)

	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	worker(ctx)
	_, data := exporter.ExportArgsForCall(0)
	g.Expect(data).To(Equal(&expData))
}

func TestCreateTelemetryJobWorker_CollectFails(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	exporter := &telemetryfakes.FakeExporter{}
	dataCollector := &telemetryfakes.FakeDataCollector{}

	worker := telemetry.CreateTelemetryJobWorker(logr.Discard(), exporter, dataCollector)

	expData := telemetry.Data{}
	dataCollector.CollectReturns(expData, errors.New("failed to collect cluster information"))

	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	worker(ctx)
	g.Expect(exporter.ExportCallCount()).To(Equal(0))
}

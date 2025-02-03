package telemetry_test

import (
	"context"
	"errors"
	"testing"
	"time"

	tel "github.com/nginx/telemetry-exporter/pkg/telemetry"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/telemetry"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/telemetry/telemetryfakes"
)

func TestCreateTelemetryJobWorker_Succeeds(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	exporter := &telemetryfakes.FakeExporter{}
	dataCollector := &telemetryfakes.FakeDataCollector{}

	worker := telemetry.CreateTelemetryJobWorker(zap.New(), exporter, dataCollector)

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

	worker := telemetry.CreateTelemetryJobWorker(zap.New(), exporter, dataCollector)

	expData := telemetry.Data{}
	dataCollector.CollectReturns(expData, errors.New("failed to collect cluster information"))

	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	worker(ctx)
	g.Expect(exporter.ExportCallCount()).To(Equal(0))
}

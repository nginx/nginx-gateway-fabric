package telemetry

import (
	"bytes"
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestLoggingExporter(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var buffer bytes.Buffer
	logger := zap.New(zap.WriteTo(&buffer))
	exporter := NewLoggingExporter(logger)

	err := exporter.Export(context.Background(), &Data{})

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(buffer.String()).To(ContainSubstring(`"level":"info"`))
	g.Expect(buffer.String()).To(ContainSubstring(`"msg":"Exporting telemetry"`))
}

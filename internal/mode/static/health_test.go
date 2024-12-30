package static

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestReadyCheck(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	healthChecker := newGraphBuiltHealthChecker()
	g.Expect(healthChecker.readyCheck(nil)).ToNot(Succeed())

	healthChecker.ready = true
	g.Expect(healthChecker.readyCheck(nil)).To(Succeed())
}

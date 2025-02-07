package static

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/config"
)

func TestReadyCheck(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	healthChecker := newGraphBuiltHealthChecker()

	g.Expect(healthChecker.readyCheck(nil)).To(MatchError(errors.New("this NGF Pod is not currently leader")))

	healthChecker.graphBuilt = true
	g.Expect(healthChecker.readyCheck(nil)).To(MatchError(errors.New("this NGF Pod is not currently leader")))

	healthChecker.graphBuilt = false
	healthChecker.leader = true
	g.Expect(healthChecker.readyCheck(nil)).
		To(MatchError(errors.New("control plane initial graph has not been built")))

	healthChecker.graphBuilt = true
	g.Expect(healthChecker.readyCheck(nil)).To(Succeed())
}

func TestSetAsLeader(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	healthChecker := newGraphBuiltHealthChecker()

	g.Expect(healthChecker.leader).To(BeFalse())
	g.Expect(healthChecker.readyCh).ShouldNot(BeClosed())

	healthChecker.setAsLeader()

	g.Expect(healthChecker.leader).To(BeTrue())
	g.Expect(healthChecker.readyCh).To(BeClosed())
}

func TestSetGraphBuilt(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	healthChecker := newGraphBuiltHealthChecker()

	g.Expect(healthChecker.graphBuilt).To(BeFalse())

	healthChecker.setGraphBuilt()

	g.Expect(healthChecker.graphBuilt).To(BeTrue())
}

func TestReadyHandler(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	healthChecker := newGraphBuiltHealthChecker()

	r := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()

	healthChecker.readyHandler(w, r)
	g.Expect(w.Result().StatusCode).To(Equal(http.StatusServiceUnavailable))

	healthChecker.graphBuilt = true
	healthChecker.leader = true

	w = httptest.NewRecorder()
	healthChecker.readyHandler(w, r)
	g.Expect(w.Result().StatusCode).To(Equal(http.StatusOK))
}

func TestCreateHealthProbe(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	healthChecker := newGraphBuiltHealthChecker()

	cfg := config.Config{HealthConfig: config.HealthConfig{Port: 100000}}
	_, err := createHealthProbe(cfg, healthChecker)
	g.Expect(err).To(MatchError("error listening on :100000: listen tcp: address 100000: invalid port"))

	cfg = config.Config{HealthConfig: config.HealthConfig{Port: 8081}}
	hp, err := createHealthProbe(cfg, healthChecker)
	g.Expect(err).ToNot(HaveOccurred())

	addr, ok := (hp.Listener.Addr()).(*net.TCPAddr)
	g.Expect(ok).To(BeTrue())

	g.Expect(addr.Port).To(Equal(cfg.HealthConfig.Port))
	g.Expect(hp.Server).ToNot(BeNil())
}

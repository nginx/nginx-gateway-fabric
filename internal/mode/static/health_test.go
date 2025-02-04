package static

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"
)

func TestReadyCheck(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	healthChecker := newGraphBuiltHealthChecker()
	g.Expect(healthChecker.readyCheck(nil)).To(MatchError(errors.New("control plane is not yet ready")))

	healthChecker.ready = true
	g.Expect(healthChecker.readyCheck(nil)).To(MatchError(errors.New("this NGF Pod is not currently leader")))

	healthChecker.ready = false
	healthChecker.leader = true
	g.Expect(healthChecker.readyCheck(nil)).To(MatchError(errors.New("control plane is not yet ready")))

	healthChecker.ready = true
	g.Expect(healthChecker.readyCheck(nil)).To(Succeed())
}

func TestSetAsLeader(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	healthChecker := newGraphBuiltHealthChecker()
	healthChecker.eventCh = make(chan interface{}, 1)

	g.Expect(healthChecker.leader).To(BeFalse())
	g.Expect(healthChecker.eventCh).ShouldNot(Receive())

	healthChecker.setAsLeader()

	g.Expect(healthChecker.leader).To(BeTrue())
	g.Expect(healthChecker.eventCh).Should(Receive())
}

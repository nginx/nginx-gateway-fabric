package runnables

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
)

func TestLeader(t *testing.T) {
	t.Parallel()
	leader := &Leader{}

	g := NewWithT(t)
	g.Expect(leader.NeedLeaderElection()).To(BeTrue())
}

func TestLeaderOrNonLeader(t *testing.T) {
	t.Parallel()
	leaderOrNonLeader := &LeaderOrNonLeader{}

	g := NewWithT(t)
	g.Expect(leaderOrNonLeader.NeedLeaderElection()).To(BeFalse())
}

func TestCallFunctionsAfterBecameLeader(t *testing.T) {
	t.Parallel()
	enabled := false
	leader := false

	callFunctionsAfterBecameLeader := NewCallFunctionsAfterBecameLeader(
		func(_ context.Context) { enabled = true },
		func() { leader = true },
	)

	g := NewWithT(t)
	g.Expect(callFunctionsAfterBecameLeader.NeedLeaderElection()).To(BeTrue())
	g.Expect(enabled).To(BeFalse())

	err := callFunctionsAfterBecameLeader.Start(context.Background())
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(enabled).To(BeTrue())
	g.Expect(leader).To(BeTrue())
}

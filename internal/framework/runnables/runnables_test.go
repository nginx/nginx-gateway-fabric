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
	statusUpdaterEnabled := false
	provisionerEnabled := false

	callFunctionsAfterBecameLeader := NewCallFunctionsAfterBecameLeader([]func(ctx context.Context){
		func(_ context.Context) { statusUpdaterEnabled = true },
		func(_ context.Context) { provisionerEnabled = true },
	})

	g := NewWithT(t)
	g.Expect(callFunctionsAfterBecameLeader.NeedLeaderElection()).To(BeTrue())

	err := callFunctionsAfterBecameLeader.Start(context.Background())
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(statusUpdaterEnabled).To(BeTrue())
	g.Expect(provisionerEnabled).To(BeTrue())
}

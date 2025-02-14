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

func TestEnableAfterBecameLeader(t *testing.T) {
	t.Parallel()
	enabled := false
	enableAfterBecameLeader := NewEnableAfterBecameLeader(func(_ context.Context) {
		enabled = true
	})

	g := NewWithT(t)
	g.Expect(enableAfterBecameLeader.NeedLeaderElection()).To(BeTrue())
	g.Expect(enabled).To(BeFalse())

	err := enableAfterBecameLeader.Start(context.Background())
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(enabled).To(BeTrue())
}

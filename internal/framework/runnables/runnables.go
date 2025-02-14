package runnables

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Leader is a Runnable that needs to be run only when the current instance is the leader.
type Leader struct {
	manager.Runnable
}

var (
	_ manager.LeaderElectionRunnable = &Leader{}
	_ manager.Runnable               = &Leader{}
)

func (r *Leader) NeedLeaderElection() bool {
	return true
}

// LeaderOrNonLeader is a Runnable that needs to be run regardless of whether the current instance is the leader.
type LeaderOrNonLeader struct {
	manager.Runnable
}

var (
	_ manager.LeaderElectionRunnable = &LeaderOrNonLeader{}
	_ manager.Runnable               = &LeaderOrNonLeader{}
)

func (r *LeaderOrNonLeader) NeedLeaderElection() bool {
	return false
}

// EnableAfterBecameLeader is a Runnable that will call the enable function when the current instance becomes
// the leader.
type EnableAfterBecameLeader struct {
	enable func(context.Context)
}

var (
	_ manager.LeaderElectionRunnable = &EnableAfterBecameLeader{}
	_ manager.Runnable               = &EnableAfterBecameLeader{}
)

// NewEnableAfterBecameLeader creates a new EnableAfterBecameLeader Runnable.
func NewEnableAfterBecameLeader(enable func(context.Context)) *EnableAfterBecameLeader {
	return &EnableAfterBecameLeader{
		enable: enable,
	}
}

func (j *EnableAfterBecameLeader) Start(ctx context.Context) error {
	j.enable(ctx)
	return nil
}

func (j *EnableAfterBecameLeader) NeedLeaderElection() bool {
	return true
}

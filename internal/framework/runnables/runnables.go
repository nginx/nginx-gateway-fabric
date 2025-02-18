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

// CallFunctionsAfterBecameLeader is a Runnable that will call the given functions when the current instance becomes
// the leader.
type CallFunctionsAfterBecameLeader struct {
	enableFunctions []func(context.Context)
}

var (
	_ manager.LeaderElectionRunnable = &CallFunctionsAfterBecameLeader{}
	_ manager.Runnable               = &CallFunctionsAfterBecameLeader{}
)

// NewCallFunctionsAfterBecameLeader creates a new CallFunctionsAfterBecameLeader Runnable.
func NewCallFunctionsAfterBecameLeader(
	enableFunctions []func(context.Context),
) *CallFunctionsAfterBecameLeader {
	return &CallFunctionsAfterBecameLeader{
		enableFunctions: enableFunctions,
	}
}

func (j *CallFunctionsAfterBecameLeader) Start(ctx context.Context) error {
	for _, f := range j.enableFunctions {
		f(ctx)
	}

	return nil
}

func (j *CallFunctionsAfterBecameLeader) NeedLeaderElection() bool {
	return true
}

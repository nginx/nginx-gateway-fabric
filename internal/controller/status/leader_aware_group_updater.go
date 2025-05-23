package status

import (
	"context"
	"errors"
	"sync"
)

// GroupUpdater updates statuses of groups of resources.
//
// Note: this interface is created so that it that we can create a fake from it and use it
// in controller/handler_test.go (to avoid import cycles).
//
//counterfeiter:generate . GroupUpdater
type GroupUpdater interface {
	UpdateGroup(ctx context.Context, name string, reqs ...UpdateRequest)
}

// LeaderAwareGroupUpdater updates statuses of groups of resources.
// Before it is enabled, it saves all requests.
// When it is enabled, it updates status using the saved requests. Note: it can only be enabled once.
// After it is enabled, it will not save requests anymore and update statuses immediately.
type LeaderAwareGroupUpdater struct {
	updater   *Updater
	lock      *sync.Mutex
	groupReqs map[string][]UpdateRequest
	enabled   bool
}

// NewLeaderAwareGroupUpdater creates a new LeaderAwareGroupUpdater.
func NewLeaderAwareGroupUpdater(updater *Updater) *LeaderAwareGroupUpdater {
	return &LeaderAwareGroupUpdater{
		updater:   updater,
		lock:      &sync.Mutex{},
		groupReqs: make(map[string][]UpdateRequest),
	}
}

// UpdateGroup updates statuses of a group of resources.
func (u *LeaderAwareGroupUpdater) UpdateGroup(ctx context.Context, name string, reqs ...UpdateRequest) {
	u.lock.Lock()
	defer u.lock.Unlock()

	if !u.enabled {
		if len(reqs) == 0 {
			delete(u.groupReqs, name)
			return
		}

		u.groupReqs[name] = reqs
		return
	}

	u.updater.Update(ctx, reqs...)
}

// Enable enables the LeaderAwareGroupUpdater, updating statuses using the saved requests.
func (u *LeaderAwareGroupUpdater) Enable(ctx context.Context) {
	u.lock.Lock()
	defer u.lock.Unlock()

	if u.enabled {
		panic(errors.New("LeaderAwareGroupUpdater can only be enabled once"))
	}

	u.enabled = true

	for name, reqs := range u.groupReqs {
		u.updater.Update(ctx, reqs...)
		delete(u.groupReqs, name)
	}
}

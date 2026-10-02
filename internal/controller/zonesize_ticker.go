package controller

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/events"
)

// zoneSizeShrinkCheckInterval is how often the zoneSizeTicker checks whether any Deployment has
// an auto-sized upstream zone eligible to shrink.
const zoneSizeShrinkCheckInterval = 30 * time.Second

// zoneSizeDeploymentLister is the narrow slice of agent.DeploymentStorer that zoneSizeTicker
// needs: the ability to iterate over all currently known Deployments.
type zoneSizeDeploymentLister interface {
	Range(f func(types.NamespacedName, *agent.Deployment) bool)
}

// zoneSizeTicker periodically checks all known Deployments for auto-sized upstream zones that
// have become eligible to shrink back down, and if it finds any, injects a
// events.ZoneSizeReevaluateEvent onto the event loop to trigger a re-reconcile.
type zoneSizeTicker struct {
	deployments zoneSizeDeploymentLister
	eventCh     chan<- any
	logger      logr.Logger
	interval    time.Duration
}

// newZoneSizeTicker creates a new zoneSizeTicker using the default check interval.
func newZoneSizeTicker(
	deployments zoneSizeDeploymentLister,
	eventCh chan<- any,
	logger logr.Logger,
) *zoneSizeTicker {
	return &zoneSizeTicker{
		deployments: deployments,
		eventCh:     eventCh,
		logger:      logger,
		interval:    zoneSizeShrinkCheckInterval,
	}
}

// Start begins the ticker loop. It blocks until ctx is canceled.
func (t *zoneSizeTicker) Start(ctx context.Context) error {
	t.logger.Info("Zone size shrink check started", "interval", t.interval)

	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-ticker.C:
			if !t.anyShrinkDue(now) {
				continue
			}

			t.logger.V(1).Info("Auto-sized upstream zone(s) eligible to shrink, triggering re-reconcile")

			select {
			case t.eventCh <- events.ZoneSizeReevaluateEvent{}:
			case <-ctx.Done():
				return nil
			}
		}
	}
}

// anyShrinkDue reports whether any known Deployment has at least one auto-sized upstream zone
// that has become eligible to shrink back down as of now.
func (t *zoneSizeTicker) anyShrinkDue(now time.Time) bool {
	due := false

	t.deployments.Range(func(_ types.NamespacedName, deployment *agent.Deployment) bool {
		if deployment.HasPendingZoneShrink(now) {
			due = true
			return false
		}
		return true
	})

	return due
}

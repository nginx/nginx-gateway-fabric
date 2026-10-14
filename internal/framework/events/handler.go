package events

import (
	"context"

	"github.com/go-logr/logr"
)

//go:generate go tool moq -skip-ensure -pkg eventsfakes -out eventsfakes/fake_event_handler.go . EventHandler

// EventHandler handles events.
type EventHandler interface {
	// HandleEventBatch handles a batch of events.
	// EventBatch can include duplicated events.
	HandleEventBatch(ctx context.Context, logger logr.Logger, batch EventBatch)
}

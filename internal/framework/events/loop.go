package events

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
)

// EventLoop is the main event loop of the Gateway. It handles events coming through the event channel.
//
// When a new event comes, there are two cases:
// - If there is no event(s) currently being handled, the new event is handled immediately.
// - Otherwise, the new event will be saved for later handling. All saved events will be handled after the handling of
// the current event(s) finishes. Multiple saved events will be handled at once -- they will be batched.
//
// Batching is needed because handling an event (or multiple events at once) will typically result in
// reloading NGINX, which is an operation we want to minimize for the following reasons:
// (1) A reload takes time - at least 200ms. The time depends on the size of the configuration, the number of
// TLS certs, and the number of available CPU cycles.
// (2) A reload can have side-effects for data plane traffic.
// FIXME(pleshakov): better document the side effects and how to prevent and mitigate them.
// So when the EventLoop have 100 saved events, it is better to process them at once rather than one by one.
// https://github.com/nginx/nginx-gateway-fabric/issues/551
type EventLoop struct {
	handler  EventHandler
	preparer FirstEventBatchPreparer
	eventCh  <-chan interface{}
	logger   logr.Logger

	// The EventLoop uses double buffering to handle event batch processing.
	// The goroutine that handles the batch will always read from the currentBatch slice.
	// While the current batch is being handled, new events are added to the nextBatch slice.
	// The batches are swapped before starting the handler goroutine.
	currentBatch EventBatch
	nextBatch    EventBatch

	// the ID of the current batch
	currentBatchID int
}

// NewEventLoop creates a new EventLoop.
func NewEventLoop(
	eventCh <-chan interface{},
	logger logr.Logger,
	handler EventHandler,
	preparer FirstEventBatchPreparer,
) *EventLoop {
	return &EventLoop{
		eventCh:      eventCh,
		logger:       logger,
		handler:      handler,
		preparer:     preparer,
		currentBatch: make(EventBatch, 0),
		nextBatch:    make(EventBatch, 0),
	}
}

// Start starts the EventLoop.
// This method will block until the EventLoop stops, which will happen after the ctx is closed.
func (el *EventLoop) Start(ctx context.Context) error {
	// handling tells if any batch is currently being handled.
	var handling bool
	// handlingDone is used to signal the completion of handling a batch.
	handlingDone := make(chan struct{})

	handleBatch := func() {
		go func(batch EventBatch) {
			el.currentBatchID++
			batchLogger := el.logger.WithName("eventHandler").WithValues("batchID", el.currentBatchID)

			batchLogger.V(1).Info("Handling events from the batch", "total", len(batch))

			el.handler.HandleEventBatch(ctx, batchLogger, batch)

			batchLogger.V(1).Info("Finished handling the batch")
			handlingDone <- struct{}{}
		}(el.currentBatch)
	}

	swapAndHandleBatch := func() {
		el.swapBatches()
		handleBatch()
		handling = true
	}

	// Prepare the fist event batch, which includes the UpsertEvents for all relevant cluster resources.
	// This is necessary so that the first time the EventHandler generates NGINX configuration, it derives it from
	// a complete view of the cluster. Otherwise, the handler would generate incomplete configuration, which can lead
	// to clients seeing transient 404 errors from NGINX and incorrect statuses of the resources updated by the Gateway.
	//
	// Note:
	// After the handler goroutine handles the first batch, the loop will start receiving events from
	// the controllers, which at the beginning will be UpsertEvents with the relevant cluster resources - i.e. they
	// will be duplicates of the events in the first batch. This is OK, because it is expected that the EventHandler will
	// not trigger any reconfiguration after receiving an upsert for an existing resource with the same Generation.

	var err error
	el.currentBatch, err = el.preparer.Prepare(ctx)
	if err != nil {
		return fmt.Errorf("failed to prepare the first batch: %w", err)
	}

	// Handle the first batch
	handleBatch()
	handling = true

	// Note: at any point of time, no more than one batch is currently being handled.

	// The event loop
	for {
		select {
		case <-ctx.Done():
			// Wait for the completion if a batch is being handled.
			if handling {
				<-handlingDone
			}
			return nil
		case e := <-el.eventCh:
			// Add the event to the current batch.
			el.nextBatch = append(el.nextBatch, e)

			el.logger.V(1).Info(
				"added an event to the next batch",
				"type", fmt.Sprintf("%T", e),
				"total", len(el.nextBatch),
			)

			// If no batch is currently being handled, swap batches and begin handling the batch.
			if !handling {
				swapAndHandleBatch()
			}
		case <-handlingDone:
			handling = false

			// If there's at least one event in the next batch, swap batches and begin handling the batch.
			if len(el.nextBatch) > 0 {
				swapAndHandleBatch()
			}
		}
	}
}

// swapBatches swaps the current and next batches.
func (el *EventLoop) swapBatches() {
	el.currentBatch, el.nextBatch = el.nextBatch, el.currentBatch
	el.nextBatch = el.nextBatch[:0]
}

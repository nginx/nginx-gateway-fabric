package events_test

import (
	"context"
	"errors"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/config"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/events"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/events/eventsfakes"
)

var _ = Describe("EventLoop", func() {
	var (
		fakeHandler  *eventsfakes.EventHandlerMock
		eventCh      chan any
		fakePreparer *eventsfakes.FirstEventBatchPreparerMock
		eventLoop    *events.EventLoop
		errorCh      chan error
		pauseNext    chan struct{}
		batchStarted chan struct{}
		batchRelease chan struct{}
	)

	BeforeEach(func() {
		pauseNext = make(chan struct{}, 1)
		batchStarted = make(chan struct{})
		batchRelease = make(chan struct{})
		fakeHandler = &eventsfakes.EventHandlerMock{
			HandleEventBatchFunc: func(_ context.Context, _ logr.Logger, batch events.EventBatch) {
				if len(batch) == 1 && batch[0] == "event1" {
					select {
					case <-pauseNext:
						close(batchStarted)
						<-batchRelease
					default:
					}
				}
			},
		}
		eventCh = make(chan any)
		fakePreparer = &eventsfakes.FirstEventBatchPreparerMock{}

		eventLoop = events.NewEventLoop(eventCh, config.RuntimeLogger{Logger: logr.Discard()}, fakeHandler, fakePreparer)

		errorCh = make(chan error)
	})

	Describe("Normal processing", func() {
		BeforeEach(func() {
			ctx, cancel := context.WithCancel(context.Background())
			DeferCleanup(func(dctx SpecContext) {
				cancel()
				var err error
				Eventually(errorCh).WithContext(dctx).Should(Receive(&err))
				Expect(err).ToNot(HaveOccurred())
			}, NodeTimeout(time.Second*10))

			batch := events.EventBatch{
				"event0",
			}
			fakePreparer.PrepareFunc = func(context.Context) (events.EventBatch, error) {
				return batch, nil
			}

			go func() {
				errorCh <- eventLoop.Start(ctx)
			}()

			// Ensure  the first batch is handled
			Eventually(fakeHandler.HandleEventBatchCalls).Should(HaveLen(1))
			calls := fakeHandler.HandleEventBatchCalls()
			batch = calls[0].Batch

			var expectedBatch events.EventBatch = []any{"event0"}
			Expect(batch).Should(Equal(expectedBatch))
		})

		// Because BeforeEach() creates the first batch and waits for it to be handled, in the tests below
		// HandleEventBatchCallCount() is already 1.

		It("should process a single event", func() {
			e := "event"

			eventCh <- e

			Eventually(fakeHandler.HandleEventBatchCalls).Should(HaveLen(2))
			calls := fakeHandler.HandleEventBatchCalls()
			batch := calls[1].Batch

			var expectedBatch events.EventBatch = []any{e}
			Expect(batch).Should(Equal(expectedBatch))
		})

		It("should batch multiple events", func() {
			e1 := "event1"
			e2 := "event2"
			e3 := "event3"
			pauseNext <- struct{}{}

			eventCh <- e1

			// Making sure the handler goroutine started handling the batch with e1.
			<-batchStarted

			eventCh <- e2
			eventCh <- e3
			// The event loop will add the e2 and e3 event to current batch before starting another handler goroutine.

			// Unpause the handler goroutine so that it can handle the current batch.
			close(batchRelease)

			Eventually(fakeHandler.HandleEventBatchCalls).Should(HaveLen(3))
			calls := fakeHandler.HandleEventBatchCalls()
			batch := calls[1].Batch

			var expectedBatch events.EventBatch = []any{e1}

			// the first HandleEventBatch() call must have handled a batch with e1
			Expect(batch).Should(Equal(expectedBatch))

			batch = calls[2].Batch

			expectedBatch = []any{e2, e3}
			// the second HandleEventBatch() call must have handled a batch with e2 and e3
			Expect(batch).Should(Equal(expectedBatch))
		})
	})

	Describe("Edge cases", func() {
		It("should return error when preparer returns error without blocking", func(ctx SpecContext) {
			preparerError := errors.New("test")
			fakePreparer.PrepareFunc = func(context.Context) (events.EventBatch, error) {
				return events.EventBatch{}, preparerError
			}

			err := eventLoop.Start(ctx)

			Expect(err).Should(MatchError(preparerError))
		})

		It("should return nil when started with canceled context without blocking", func(ctx context.Context) {
			fakePreparer.PrepareFunc = func(context.Context) (events.EventBatch, error) {
				return events.EventBatch{}, nil
			}

			ctx, cancel := context.WithCancel(ctx)
			cancel()
			err := eventLoop.Start(ctx)

			Expect(err).ToNot(HaveOccurred())
		})
	})
})

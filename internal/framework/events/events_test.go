package events

import (
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/config"
)

func TestEventLoop_SwapBatches(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	eventLoop := NewEventLoop(nil, config.RuntimeLogger{Logger: logr.Discard()}, nil, nil)

	eventLoop.currentBatch = EventBatch{
		"event0",
		"event1",
		"event2",
	}

	nextBatch := EventBatch{
		"event3",
		"event4",
		"event5",
		"event6",
	}

	eventLoop.nextBatch = nextBatch

	eventLoop.swapBatches()

	g.Expect(eventLoop.currentBatch).To(HaveLen(len(nextBatch)))
	g.Expect(eventLoop.currentBatch).To(Equal(nextBatch))
	g.Expect(eventLoop.nextBatch).To(BeEmpty())
	g.Expect(eventLoop.nextBatch).To(HaveCap(3))
}

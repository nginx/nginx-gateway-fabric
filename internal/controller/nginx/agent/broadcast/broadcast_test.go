package broadcast_test

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/broadcast"
)

func TestSubscribe(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	stopCh := make(chan struct{})
	defer close(stopCh)

	broadcaster := broadcast.NewDeploymentBroadcaster(t.Context(), stopCh)

	subscriber := broadcaster.Subscribe()
	g.Expect(subscriber.ID).NotTo(BeEmpty())

	// Give time for subscription to be processed by the subscriber goroutine
	time.Sleep(10 * time.Millisecond)

	message := broadcast.NginxAgentMessage{
		ConfigVersion: "v1",
		Type:          broadcast.ConfigApplyRequest,
	}

	sendDone := make(chan bool)
	go func() {
		result := broadcaster.Send(message)
		sendDone <- result
	}()

	// Subscriber should receive the message
	g.Eventually(subscriber.ListenCh).Should(Receive(Equal(message)))

	// Send response to complete the broadcast
	subscriber.ResponseCh <- struct{}{}

	// Send should complete and return true
	g.Eventually(sendDone).Should(Receive(BeTrue()))
}

func TestSubscribe_MultipleListeners(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	stopCh := make(chan struct{})
	defer close(stopCh)

	broadcaster := broadcast.NewDeploymentBroadcaster(t.Context(), stopCh)

	subscriber1 := broadcaster.Subscribe()
	subscriber2 := broadcaster.Subscribe()

	g.Expect(subscriber1.ID).NotTo(BeEmpty())
	g.Expect(subscriber2.ID).NotTo(BeEmpty())

	// Give time for both subscriptions to be processed by the subscriber goroutine
	time.Sleep(10 * time.Millisecond)

	message := broadcast.NginxAgentMessage{
		ConfigVersion: "v1",
		Type:          broadcast.ConfigApplyRequest,
	}

	sendDone := make(chan bool)
	go func() {
		result := broadcaster.Send(message)
		sendDone <- result
	}()

	// Both subscribers should receive the message
	g.Eventually(subscriber1.ListenCh).Should(Receive(Equal(message)))
	g.Eventually(subscriber2.ListenCh).Should(Receive(Equal(message)))

	// Send responses to complete the broadcast
	subscriber1.ResponseCh <- struct{}{}
	subscriber2.ResponseCh <- struct{}{}

	// Send should complete and return true
	g.Eventually(sendDone).Should(Receive(BeTrue()))
}

func TestSubscribe_NoListeners(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	stopCh := make(chan struct{})
	defer close(stopCh)

	broadcaster := broadcast.NewDeploymentBroadcaster(t.Context(), stopCh)

	message := broadcast.NginxAgentMessage{
		ConfigVersion: "v1",
		Type:          broadcast.ConfigApplyRequest,
	}

	result := broadcaster.Send(message)
	g.Expect(result).To(BeFalse())
}

func TestCancelSubscription(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	stopCh := make(chan struct{})
	defer close(stopCh)

	broadcaster := broadcast.NewDeploymentBroadcaster(t.Context(), stopCh)

	subscriber := broadcaster.Subscribe()

	broadcaster.CancelSubscription(subscriber.ID)

	message := broadcast.NginxAgentMessage{
		ConfigVersion: "v1",
		Type:          broadcast.ConfigApplyRequest,
	}

	result := broadcaster.Send(message)
	g.Expect(result).To(BeFalse())

	g.Consistently(subscriber.ListenCh).ShouldNot(Receive())
}

func TestShutdown_MessagesIgnoredAfterStopCh(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	stopCh := make(chan struct{})
	broadcaster := broadcast.NewDeploymentBroadcaster(t.Context(), stopCh)

	subscriber := broadcaster.Subscribe()

	message := broadcast.NginxAgentMessage{
		ConfigVersion: "v1",
		Type:          broadcast.ConfigApplyRequest,
	}

	// Close stopCh to trigger shutdown
	close(stopCh)

	sendDone := make(chan bool)
	go func() {
		// Send message after shutdown
		result := broadcaster.Send(message)
		sendDone <- result
	}()

	// Send should return false because broadcaster is shut down
	g.Eventually(sendDone).Should(Receive(BeFalse()))

	// Message should NOT reach subscriber during shutdown
	g.Consistently(subscriber.ListenCh, "100ms").ShouldNot(Receive())
}

func TestShutdown_ClosedStopChannelAfterListenerReceivedMessage(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	stopCh := make(chan struct{})
	broadcaster := broadcast.NewDeploymentBroadcaster(t.Context(), stopCh)

	subscriber := broadcaster.Subscribe()

	message := broadcast.NginxAgentMessage{
		ConfigVersion: "v1",
		Type:          broadcast.ConfigApplyRequest,
	}

	sendDone := make(chan bool)
	go func() {
		// Start sending a message
		result := broadcaster.Send(message)
		sendDone <- result
	}()

	// Wait for message to be received
	g.Eventually(subscriber.ListenCh).Should(Receive(Equal(message)))

	// Close stopCh while publisher is waiting for response
	close(stopCh)

	// Send should complete because response channel gets closed during shutdown
	// Note: Returns false because shutdown closes the response channel while the
	// publisher is waiting for the response
	g.Eventually(sendDone).Should(Receive(BeFalse()))
}

func TestCancelSubscription_UnblocksPublisherListenerReceivedMessage(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	stopCh := make(chan struct{})
	defer close(stopCh)

	broadcaster := broadcast.NewDeploymentBroadcaster(t.Context(), stopCh)

	subscriber := broadcaster.Subscribe()

	message := broadcast.NginxAgentMessage{
		ConfigVersion: "v1",
		Type:          broadcast.ConfigApplyRequest,
	}

	sendDone := make(chan bool)
	go func() {
		// Start sending a message
		result := broadcaster.Send(message)
		sendDone <- result
	}()

	// Wait for message to be received
	g.Eventually(subscriber.ListenCh).Should(Receive(Equal(message)))

	// Cancel subscription while publisher is waiting for response
	broadcaster.CancelSubscription(subscriber.ID)

	// Send should complete because response channel gets closed during cancellation
	// Note: Returns false because listener was canceled while the publisher was waiting
	// for the response (after the message was received)
	g.Eventually(sendDone).Should(Receive(BeFalse()))
}

func TestCancelSubscription_UnblocksPublisherListenerDidNotReceiveMessage(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	stopCh := make(chan struct{})
	defer close(stopCh)

	broadcaster := broadcast.NewDeploymentBroadcaster(t.Context(), stopCh)

	subscriber := broadcaster.Subscribe()

	message := broadcast.NginxAgentMessage{
		ConfigVersion: "v1",
		Type:          broadcast.ConfigApplyRequest,
	}

	sendDone := make(chan bool)
	go func() {
		// Start sending a message
		result := broadcaster.Send(message)
		sendDone <- result
	}()

	// Cancel subscription before publisher can send message to listenCh
	//
	// Note: Technically the publisher can receive the message before cancellation
	// due to goroutine scheduling, but the immediate cancellation following the
	// send should make it unlikely that the message is sent to the listenCh before cancellation.
	// However, in both situations where the message is received or not received, the cancellation
	// should unblock the publisher and allow Send to complete.
	broadcaster.CancelSubscription(subscriber.ID)

	// Send should complete because response channel gets closed during cancellation
	// Note: Returns false because listener was canceled before message was received
	g.Eventually(sendDone).Should(Receive(BeFalse()))
}

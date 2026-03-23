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
	time.Sleep(1 * time.Second)

	message := broadcast.NginxAgentMessage{
		ConfigVersion: "v1",
		Type:          broadcast.ConfigApplyRequest,
	}

	result := broadcaster.Send(message)
	g.Expect(result).To(BeFalse()) // No listeners after cancellation

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

	// Wait for shutdown to process by trying repeatedly
	g.Eventually(func() bool {
		result := broadcaster.Send(message)
		// Send() returns listener count from snapshot, so should still be true
		// But message should not be sent due to context cancellation
		return result == true
	}).Should(BeTrue())

	// Message should NOT reach subscriber during shutdown
	g.Consistently(subscriber.ListenCh, "100ms").ShouldNot(Receive())
}

func TestShutdown_ResponseChannelsClosedOnExit(t *testing.T) {
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
	// Note: Returns true because listener existed when message was queued
	g.Eventually(sendDone).Should(Receive(BeTrue()))
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
	// Note: Returns true because listener existed when message was queued
	g.Eventually(sendDone).Should(Receive(BeTrue()))
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

	// Cancel subscription before publisher can receive message
	broadcaster.CancelSubscription(subscriber.ID)

	// Send should complete because response channel gets closed during cancellation
	// Note: Returns true because listener existed when message was queued
	g.Eventually(sendDone).Should(Receive(BeTrue()))
}

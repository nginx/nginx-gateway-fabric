package broadcast_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/internal/controller/nginx/agent/broadcast"
)

func TestSubscribe(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	stopCh := make(chan struct{})
	defer close(stopCh)

	broadcaster := broadcast.NewDeploymentBroadcaster(context.Background(), stopCh)

	subscriber := broadcaster.Subscribe()
	g.Expect(subscriber.ID).NotTo(BeEmpty())

	message := broadcast.NginxAgentMessage{
		ConfigVersion: "v1",
		Type:          broadcast.ConfigApplyRequest,
	}

	go func() {
		result := broadcaster.Send(message)
		g.Expect(result).To(BeTrue())
	}()

	g.Eventually(subscriber.ListenCh).Should(Receive(Equal(message)))
}

func TestSubscribe_MultipleListeners(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	stopCh := make(chan struct{})
	defer close(stopCh)

	broadcaster := broadcast.NewDeploymentBroadcaster(context.Background(), stopCh)

	subscriber1 := broadcaster.Subscribe()
	subscriber2 := broadcaster.Subscribe()

	message := broadcast.NginxAgentMessage{
		ConfigVersion: "v1",
		Type:          broadcast.ConfigApplyRequest,
	}

	go func() {
		result := broadcaster.Send(message)
		g.Expect(result).To(BeTrue())
	}()

	g.Eventually(subscriber1.ListenCh).Should(Receive(Equal(message)))
	g.Eventually(subscriber2.ListenCh).Should(Receive(Equal(message)))

	subscriber1.ResponseCh <- struct{}{}
	subscriber2.ResponseCh <- struct{}{}
}

func TestSubscribe_NoListeners(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	stopCh := make(chan struct{})
	defer close(stopCh)

	broadcaster := broadcast.NewDeploymentBroadcaster(context.Background(), stopCh)

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

	broadcaster := broadcast.NewDeploymentBroadcaster(context.Background(), stopCh)

	subscriber := broadcaster.Subscribe()

	broadcaster.CancelSubscription(subscriber.ID)

	message := broadcast.NginxAgentMessage{
		ConfigVersion: "v1",
		Type:          broadcast.ConfigApplyRequest,
	}

	go func() {
		result := broadcaster.Send(message)
		g.Expect(result).To(BeFalse())
	}()

	g.Consistently(subscriber.ListenCh).ShouldNot(Receive())
}

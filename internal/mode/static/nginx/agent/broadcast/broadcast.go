package broadcast

import (
	"context"
	"errors"
	"sync"

	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . Broadcaster

// Broadcaster defines an interface for consumers to subscribe to File updates.
type Broadcaster interface {
	Subscribe() (chan NginxAgentMessage, chan<- error)
	Send(NginxAgentMessage) error
	CancelSubscription(chan NginxAgentMessage)
}

type subscriberChannels struct {
	listenCh   chan NginxAgentMessage
	responseCh chan error
}

// DeploymentBroadcaster sends out a signal when an nginx Deployment has updated
// configuration files. The signal is received by any agent Subscription that cares
// about this Deployment. The agent Subscription will then send a response of whether or not
// the configuration was successfully applied.
type DeploymentBroadcaster struct {
	publishCh chan NginxAgentMessage
	subCh     chan subscriberChannels
	unsubCh   chan chan NginxAgentMessage
	listeners map[chan NginxAgentMessage]chan error
	errorCh   chan error
}

// NewDeploymentBroadcaster returns a new instance of a DeploymentBroadcaster.
func NewDeploymentBroadcaster(ctx context.Context) *DeploymentBroadcaster {
	broadcaster := &DeploymentBroadcaster{
		listeners: make(map[chan NginxAgentMessage]chan error),
		publishCh: make(chan NginxAgentMessage),
		subCh:     make(chan subscriberChannels),
		unsubCh:   make(chan chan NginxAgentMessage),
		errorCh:   make(chan error),
	}
	go broadcaster.run(ctx)

	return broadcaster
}

// Subscribe allows a listener to subscribe to broadcast messages. It returns the channel
// to listen on for messages, as well as a channel to respond on.
func (b *DeploymentBroadcaster) Subscribe() (chan NginxAgentMessage, chan<- error) {
	listenCh := make(chan NginxAgentMessage)
	responseCh := make(chan error)
	sc := subscriberChannels{
		listenCh:   listenCh,
		responseCh: responseCh,
	}

	b.subCh <- sc
	return listenCh, responseCh
}

// Send the message to all listeners. Wait for a response from each listener
// or until a timeout.
func (b *DeploymentBroadcaster) Send(message NginxAgentMessage) error {
	b.publishCh <- message

	return <-b.errorCh
}

// CancelSubscription removes a Subscriber from the channel list.
func (b *DeploymentBroadcaster) CancelSubscription(channel chan NginxAgentMessage) {
	b.unsubCh <- channel
}

// run starts the broadcaster loop. It handles the following events:
// - if context is canceled, return.
// - if receiving a new subscriber, add it to the subscriber list.
// - if receiving a canceled subscription, remove it from the subscriber list.
// - if receiving a message to publish, send it to all subscribers.
func (b *DeploymentBroadcaster) run(ctx context.Context) {
	defer func() {
		for listener := range b.listeners {
			if listener != nil {
				close(listener)
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case channels := <-b.subCh:
			b.listeners[channels.listenCh] = channels.responseCh
		case msgCh := <-b.unsubCh:
			delete(b.listeners, msgCh)
		case msg := <-b.publishCh:
			var wg sync.WaitGroup
			wg.Add(len(b.listeners))

			responses := make(chan error, len(b.listeners))
			for msgCh, responseCh := range b.listeners {
				go func() {
					defer wg.Done()

					// send message and wait for it to be read
					msgCh <- msg
					// wait for response
					res := <-responseCh
					// add response to the list of responses
					responses <- res
				}()
			}
			wg.Wait()

			var err error
			for range len(b.listeners) {
				err = errors.Join(err, <-responses)
			}
			b.errorCh <- err
		}
	}
}

// MessageType is the type of message to be sent.
type MessageType int

const (
	// ConfigApplyRequest sends files to update nginx configuration.
	ConfigApplyRequest MessageType = iota
	// APIRequest sends an NGINX Plus API request to update configuration.
	APIRequest
)

// NginxAgentMessage is sent to all subscribers to send to the nginx agents for either a ConfigApplyRequest
// or an APIActionRequest.
type NginxAgentMessage struct {
	// ConfigVersion is the hashed configuration version of the included files.
	ConfigVersion string
	// NGINXPlusAction is an NGINX Plus API action to be sent.
	NGINXPlusAction *pb.NGINXPlusAction
	// FileOverviews contain the overviews of all files to be sent.
	FileOverviews []*pb.File
	// Type defines the type of message to be sent.
	Type MessageType
}

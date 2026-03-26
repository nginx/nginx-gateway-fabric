package broadcast

import (
	"context"
	"sync"

	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
)

//go:generate go tool counterfeiter -generate

//counterfeiter:generate . Broadcaster

// Broadcaster defines an interface for consumers to subscribe to File updates.
type Broadcaster interface {
	Subscribe() SubscriberChannels
	Send(NginxAgentMessage) bool
	CancelSubscription(string)
}

// SubscriberChannels are the channels sent to the subscriber to listen and respond on.
// The ID is used for map lookup to delete a subscriber when it's gone.
type SubscriberChannels struct {
	ListenCh   <-chan NginxAgentMessage
	ResponseCh chan<- struct{}
	ID         string
}

// storedChannels are the same channels used in the SubscriberChannels, but reverse direction.
// These are used to store the channels for the broadcaster to send and listen on,
// and can be looked up in the map using the same ID.
type storedChannels struct {
	listenCh chan<- NginxAgentMessage
	// responseCh is only received from by the broadcaster; subscribers send on it to signal completion.
	responseCh <-chan struct{}
	// listenerCtx is used to unblock the publisher if a listener unsubscribes or if the broadcaster is shutting
	// down. It is a child of the broadcaster context, so it will also be canceled on shutdown.
	listenerCtx context.Context
	cancel      context.CancelFunc
	id          string
}

// DeploymentBroadcaster sends out a signal when an nginx Deployment has updated
// configuration files. The signal is received by any agent Subscription that cares
// about this Deployment. The agent Subscription will then send a response of whether or not
// the configuration was successfully applied.
type DeploymentBroadcaster struct {
	publishCh chan NginxAgentMessage
	subCh     chan storedChannels
	unsubCh   chan string
	listeners map[string]storedChannels
	doneCh    chan struct{}
	// broadcasterCtx is the main context for the broadcaster, which is canceled on shutdown.
	// It is the parent context for all listener contexts.
	broadcasterCtx    context.Context
	broadcasterCancel context.CancelFunc
	// mu protects the listeners map. It is needed for concurrent access to
	// the listeners map in the publisher and subscriber goroutines.
	mu sync.RWMutex
}

// NewDeploymentBroadcaster returns a new instance of a DeploymentBroadcaster.
func NewDeploymentBroadcaster(ctx context.Context, stopCh chan struct{}) *DeploymentBroadcaster {
	//nolint:gosec // G118: broadcasterCancel is called in subscriber() defer
	broadcasterCtx, broadcasterCancel := context.WithCancel(ctx)

	broadcaster := &DeploymentBroadcaster{
		listeners:         make(map[string]storedChannels),
		publishCh:         make(chan NginxAgentMessage),
		subCh:             make(chan storedChannels),
		unsubCh:           make(chan string),
		doneCh:            make(chan struct{}),
		broadcasterCtx:    broadcasterCtx,
		broadcasterCancel: broadcasterCancel,
	}

	go broadcaster.subscriber(stopCh)
	go broadcaster.publisher()

	return broadcaster
}

// Subscribe allows a listener to subscribe to broadcast messages. It returns the channel
// to listen on for messages, as well as a channel to respond on.
func (b *DeploymentBroadcaster) Subscribe() SubscriberChannels {
	listenCh := make(chan NginxAgentMessage)
	responseCh := make(chan struct{})
	id := string(uuid.NewUUID())
	// Create listener context as child of broadcaster context
	//nolint:gosec // G118: cancel is called when unsubscribing in subscriber()
	listenerCtx, cancel := context.WithCancel(b.broadcasterCtx)

	subscriberChans := SubscriberChannels{
		ID:         id,
		ListenCh:   listenCh,
		ResponseCh: responseCh,
	}
	storedChans := storedChannels{
		id:          id,
		listenCh:    listenCh,
		responseCh:  responseCh,
		listenerCtx: listenerCtx,
		cancel:      cancel,
	}

	select {
	case <-b.broadcasterCtx.Done():
		// Broadcaster is shutting down or already shut down; existing value to
		// unblock subscriber, but return channels so the caller
		// can still interact with them while shutting down.
		return subscriberChans
	case b.subCh <- storedChans:
		// Subscription sent successfully
	}

	return subscriberChans
}

// Send the message to all listeners. Wait for all listeners to respond.
// Returns true if there were listeners that received and responded to the message.
func (b *DeploymentBroadcaster) Send(message NginxAgentMessage) bool {
	// Try to send message, but can be interrupted by shutdown
	select {
	case b.publishCh <- message:
	case <-b.broadcasterCtx.Done():
		return false
	}

	// Wait for completion, but can be interrupted by shutdown
	select {
	case <-b.doneCh:
	case <-b.broadcasterCtx.Done():
		return false
	}

	b.mu.RLock()
	listenerCount := len(b.listeners)
	b.mu.RUnlock()

	return listenerCount > 0
}

// CancelSubscription removes a Subscriber from the channel list.
func (b *DeploymentBroadcaster) CancelSubscription(id string) {
	select {
	case b.unsubCh <- id:
	case <-b.broadcasterCtx.Done():
		// Broadcaster is shutting down or already shut down; avoid blocking send.
		return
	}
}

// subscriber handles subscription management and stop conditions. It is responsible for cleaning up resources
// on shutdown/function return, specifically by canceling the broadcaster context (and thus all listener
// contexts) to unblock any pending publisher goroutines.
func (b *DeploymentBroadcaster) subscriber(stopCh chan struct{}) {
	defer func() {
		// Canceling the broadcaster context will cancel all listener contexts since they are children,
		// which will unblock any publishers waiting on those contexts.
		b.broadcasterCancel()
	}()

	for {
		select {
		case <-stopCh:
			return
		case <-b.broadcasterCtx.Done():
			return
		case channels := <-b.subCh:
			b.mu.Lock()
			b.listeners[channels.id] = channels
			b.mu.Unlock()
		case id := <-b.unsubCh:
			b.mu.Lock()
			if channels, exists := b.listeners[id]; exists {
				// Cancel listener's context to unblock publisher
				channels.cancel()
				delete(b.listeners, id)
			}
			b.mu.Unlock()
		}
	}
}

// publisher handles message publishing.
func (b *DeploymentBroadcaster) publisher() {
	// Due to the split between the subscription management and publishing,
	// every blocking select in this function needs a way to be unblocked
	// by the subscriber function.
	for {
		select {
		case <-b.broadcasterCtx.Done():
			return
		case msg := <-b.publishCh:
			b.mu.RLock()
			currentListeners := make(map[string]storedChannels, len(b.listeners))
			for k, v := range b.listeners {
				currentListeners[k] = v
			}
			b.mu.RUnlock()

			// Send to all listeners
			var wg sync.WaitGroup
			for _, channels := range currentListeners {
				wg.Go(func() {
					select {
					case <-channels.listenerCtx.Done():
						return
					case <-b.broadcasterCtx.Done():
						return
					case channels.listenCh <- msg:
						// Message sent successfully, now wait for response in next select
					}

					select {
					case <-channels.listenerCtx.Done():
						return
					case <-b.broadcasterCtx.Done():
						return
					case <-channels.responseCh:
						// Response received, continue
						return
					}
				})
			}
			wg.Wait()

			select {
			// If the broadcaster context is done, there may be nothing to receive
			// the done signal, so we return to avoid blocking.
			case <-b.broadcasterCtx.Done():
				return
			case b.doneCh <- struct{}{}:
				// Signal that publishing is done and all responses received
			}
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

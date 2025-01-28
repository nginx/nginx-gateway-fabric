package messenger

import (
	"context"
	"errors"

	pb "github.com/nginx/agent/v3/api/grpc/mpi/v1"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . Messenger

// Messenger is a wrapper around a gRPC stream with the nginx agent.
type Messenger interface {
	Run(context.Context)
	Send(context.Context, *pb.ManagementPlaneRequest) error
	Messages() <-chan *pb.DataPlaneResponse
	Errors() <-chan error
}

// NginxAgentMessenger is the implementation of the Messenger interface.
type NginxAgentMessenger struct {
	incoming chan *pb.ManagementPlaneRequest
	outgoing chan *pb.DataPlaneResponse
	errorCh  chan error
	server   pb.CommandService_SubscribeServer
}

// New returns a new Messenger instance.
func New(server pb.CommandService_SubscribeServer) Messenger {
	return &NginxAgentMessenger{
		incoming: make(chan *pb.ManagementPlaneRequest),
		outgoing: make(chan *pb.DataPlaneResponse),
		errorCh:  make(chan error),
		server:   server,
	}
}

// Run starts the Messenger to listen for any Send() or Recv() events over the stream.
func (m *NginxAgentMessenger) Run(ctx context.Context) {
	go m.handleRecv(ctx)
	m.handleSend(ctx)
}

// Send a message, will return error if the context is Done.
func (m *NginxAgentMessenger) Send(ctx context.Context, msg *pb.ManagementPlaneRequest) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case m.incoming <- msg:
	}
	return nil
}

func (m *NginxAgentMessenger) handleSend(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-m.incoming:
			err := m.server.Send(msg)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
					return
				}
				m.errorCh <- err

				return
			}
		}
	}
}

// Messages returns the data plane response channel.
func (m *NginxAgentMessenger) Messages() <-chan *pb.DataPlaneResponse {
	return m.outgoing
}

// Errors returns the error channel.
func (m *NginxAgentMessenger) Errors() <-chan error {
	return m.errorCh
}

// handleRecv handles an incoming message from the nginx agent.
// It blocks until Recv returns. The result from the Recv is either going to Error or Messages channel.
func (m *NginxAgentMessenger) handleRecv(ctx context.Context) {
	for {
		msg, err := m.server.Recv()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case m.errorCh <- err:
			}
			return
		}

		if msg == nil {
			// close the outgoing channel to signal no more messages to be sent
			close(m.outgoing)
			return
		}

		select {
		case <-ctx.Done():
			return
		case m.outgoing <- msg:
		}
	}
}

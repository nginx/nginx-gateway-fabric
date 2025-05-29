package messenger_test

import (
	"context"
	"errors"
	"testing"

	v1 "github.com/nginx/agent/v3/api/grpc/mpi/v1"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/agent/grpc/messenger"
)

type mockServer struct {
	grpc.ServerStream
	sendChan chan *v1.ManagementPlaneRequest
	recvChan chan *v1.DataPlaneResponse
}

func (m *mockServer) Send(msg *v1.ManagementPlaneRequest) error {
	m.sendChan <- msg
	return nil
}

func (m *mockServer) Recv() (*v1.DataPlaneResponse, error) {
	msg, ok := <-m.recvChan
	if !ok {
		return nil, errors.New("channel closed")
	}
	return msg, nil
}

type mockErrorServer struct {
	grpc.ServerStream
	sendChan chan *v1.ManagementPlaneRequest
	recvChan chan *v1.DataPlaneResponse
}

func (m *mockErrorServer) Send(_ *v1.ManagementPlaneRequest) error {
	return errors.New("error sending to server")
}

func (m *mockErrorServer) Recv() (*v1.DataPlaneResponse, error) {
	<-m.recvChan
	return nil, errors.New("error received from server")
}

func createServer() *mockServer {
	return &mockServer{
		sendChan: make(chan *v1.ManagementPlaneRequest, 1),
		recvChan: make(chan *v1.DataPlaneResponse, 1),
	}
}

func createErrorServer() *mockErrorServer {
	return &mockErrorServer{
		sendChan: make(chan *v1.ManagementPlaneRequest, 1),
		recvChan: make(chan *v1.DataPlaneResponse, 1),
	}
}

func TestSend(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := createServer()
	msgr := messenger.New(server)

	go msgr.Run(ctx)

	msg := &v1.ManagementPlaneRequest{
		MessageMeta: &v1.MessageMeta{
			MessageId: "test",
		},
	}
	g.Expect(msgr.Send(ctx, msg)).To(Succeed())

	g.Eventually(server.sendChan).Should(Receive(Equal(msg)))

	cancel()

	g.Expect(msgr.Send(ctx, &v1.ManagementPlaneRequest{})).ToNot(Succeed())
}

func TestMessages(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := createServer()
	msgr := messenger.New(server)

	go msgr.Run(ctx)

	msg := &v1.DataPlaneResponse{InstanceId: "test"}
	server.recvChan <- msg

	g.Eventually(msgr.Messages()).Should(Receive(Equal(msg)))
}

func TestErrors(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := createErrorServer()
	msgr := messenger.New(server)

	go msgr.Run(ctx)

	g.Expect(msgr.Send(ctx, &v1.ManagementPlaneRequest{})).To(Succeed())
	g.Eventually(msgr.Errors()).Should(Receive(MatchError("error sending to server")))

	server.recvChan <- &v1.DataPlaneResponse{}

	g.Eventually(msgr.Errors()).Should(Receive(MatchError("error received from server")))
}

package commander

import (
	"context"
	"errors"
	"testing"

	"github.com/nginx/agent/sdk/v2/proto"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/nginxinc/nginx-kubernetes-gateway/internal/grpc/commander/exchanger/exchangerfakes"
)

func TestConnection_Run_ExchangerErr(t *testing.T) {
	g := NewGomegaWithT(t)

	exchangerClose := make(chan struct{})
	exchangerErr := errors.New("exchanger error")

	fakeExchanger := &exchangerfakes.FakeCommandExchanger{
		RunStub: func(ctx context.Context) error {
			<-exchangerClose
			return errors.New("exchanger error")
		},
	}

	conn := newConnection("id", zap.New(), fakeExchanger)

	errCh := make(chan error)
	go func() {
		errCh <- conn.run(context.Background())
	}()

	close(exchangerClose)

	err := <-errCh
	g.Expect(err).Should(MatchError(exchangerErr))
}

func TestConnection_Run_ConnectionError(t *testing.T) {
	g := NewGomegaWithT(t)
	ctx, cancel := context.WithCancel(context.Background())

	fakeExchanger := &exchangerfakes.FakeCommandExchanger{
		RunStub: func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		},
	}

	conn := newConnection("id", zap.New(), fakeExchanger)

	errCh := make(chan error)
	go func() {
		errCh <- conn.run(ctx)
	}()

	cancel()

	err := <-errCh
	g.Expect(err).Should(MatchError(context.Canceled))
}

func TestConnection_Receive(t *testing.T) {
	g := NewGomegaWithT(t)

	out := make(chan *proto.Command)
	in := make(chan *proto.Command)

	ctx, cancel := context.WithCancel(context.Background())

	fakeExchanger := &exchangerfakes.FakeCommandExchanger{
		OutStub: func() chan<- *proto.Command {
			return out
		},
		InStub: func() <-chan *proto.Command {
			return in
		},
	}

	conn := newConnection("id", zap.New(), fakeExchanger)

	errCh := make(chan error)
	go func() {
		errCh <- conn.receive(ctx)
	}()

	sendCmdAndVerifyResponse := func(msgID string) {
		in <- CreateAgentConnectRequestCmd(msgID)

		res := <-out
		g.Expect(res).ToNot(BeNil())
		meta := res.GetMeta()
		g.Expect(meta).ToNot(BeNil())
		g.Expect(meta.MessageId).To(Equal(msgID))
	}

	sendCmdAndVerifyResponse("msg-1")
	sendCmdAndVerifyResponse("msg-2")

	cancel()

	receiveErr := <-errCh
	g.Expect(receiveErr).Should(MatchError(context.Canceled))
}

func TestConnection_State(t *testing.T) {
	g := NewGomegaWithT(t)

	conn := newConnection("id", zap.New(), new(exchangerfakes.FakeCommandExchanger))
	g.Expect(conn.State()).To(Equal(StateConnected))

	// change state
	conn.state = StateRegistered
	g.Expect(conn.State()).To(Equal(StateRegistered))
}

func TestConnection_ID(t *testing.T) {
	g := NewGomegaWithT(t)

	conn := newConnection("id", zap.New(), new(exchangerfakes.FakeCommandExchanger))
	g.Expect(conn.ID()).To(Equal("id"))
}

func TestConnection_HandleCommand(t *testing.T) {
	tests := []struct {
		cmd           *proto.Command
		expCmdType    *proto.Command
		msg           string
		expInboundCmd bool
	}{
		{
			msg:           "unsupported command",
			cmd:           &proto.Command{Data: &proto.Command_EventReport{}},
			expInboundCmd: false,
		},
		{
			msg:           "agent connect request command",
			cmd:           CreateAgentConnectRequestCmd("msg-id"),
			expInboundCmd: true,
			expCmdType:    &proto.Command{Data: &proto.Command_AgentConnectResponse{}},
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			g := NewGomegaWithT(t)

			out := make(chan *proto.Command, 1)

			fakeExchanger := &exchangerfakes.FakeCommandExchanger{
				OutStub: func() chan<- *proto.Command {
					return out
				},
			}

			conn := newConnection("id", zap.New(), fakeExchanger)

			conn.handleCommand(context.Background(), test.cmd)

			if test.expInboundCmd {
				cmd := <-out
				g.Expect(cmd.Data).To(BeAssignableToTypeOf(test.expCmdType.Data))
			} else {
				g.Expect(out).To(BeEmpty())
			}

			close(out)
		})
	}
}

func TestConnection_HandleAgentConnectRequest(t *testing.T) {
	invalidConnectRequest := &proto.Command{
		Meta: &proto.Metadata{
			MessageId: "msg-id",
		},
		Data: &proto.Command_AgentConnectRequest{
			AgentConnectRequest: &proto.AgentConnectRequest{
				Meta:    &proto.AgentMeta{},
				Details: []*proto.NginxDetails{},
			},
		},
	}

	tests := []struct {
		request       *proto.Command
		name          string
		expStatusMsg  string
		expStatusCode proto.AgentConnectStatus_StatusCode
	}{
		{
			name:          "normal",
			request:       CreateAgentConnectRequestCmd("msg-id"),
			expStatusCode: proto.AgentConnectStatus_CONNECT_OK,
			expStatusMsg:  "Connected",
		},
		{
			name:          "invalid",
			request:       invalidConnectRequest,
			expStatusCode: proto.AgentConnectStatus_CONNECT_REJECTED_OTHER,
			expStatusMsg:  "missing nginxID: '' and/or systemID: ''",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			out := make(chan *proto.Command)

			fakeExchanger := &exchangerfakes.FakeCommandExchanger{
				OutStub: func() chan<- *proto.Command {
					return out
				},
			}

			conn := newConnection("id", zap.New(), fakeExchanger)

			go conn.handleAgentConnectRequest(context.Background(), test.request)

			response := <-out

			meta := response.GetMeta()
			g.Expect(meta).ToNot(BeNil())
			g.Expect(meta.MessageId).To(Equal("msg-id"))

			agentConnResponse := response.GetAgentConnectResponse()
			g.Expect(agentConnResponse).ToNot(BeNil())
			g.Expect(agentConnResponse.Status.StatusCode).To(Equal(test.expStatusCode))
			g.Expect(agentConnResponse.Status.Message).To(Equal(test.expStatusMsg))

			if test.expStatusCode == proto.AgentConnectStatus_CONNECT_OK {
				g.Expect(conn.state).To(Equal(StateRegistered))
			} else {
				g.Expect(conn.state).To(Equal(StateInvalid))
			}
		})
	}
}

func TestConnection_HandleAgentConnectRequest_CtxCanceled(t *testing.T) {
	g := NewGomegaWithT(t)

	out := make(chan *proto.Command)

	fakeExchanger := &exchangerfakes.FakeCommandExchanger{
		OutStub: func() chan<- *proto.Command {
			return out
		},
	}

	conn := newConnection("id", zap.New(), fakeExchanger)

	ctx, cancel := context.WithCancel(context.Background())

	cmd := CreateAgentConnectRequestCmd("msg-id")

	done := make(chan struct{})
	go func() {
		conn.handleAgentConnectRequest(ctx, cmd)
		close(done)
	}()

	cancel()

	g.Eventually(done).Should(BeClosed())
}

func TestConnection_Register(t *testing.T) {
	tests := []struct {
		msg         string
		nginxID     string
		systemID    string
		expRegister bool
	}{
		{
			msg:         "valid nginxID and systemID",
			nginxID:     "nginx",
			systemID:    "system",
			expRegister: true,
		},
		{
			msg:         "invalid nginxID",
			nginxID:     "",
			systemID:    "system",
			expRegister: false,
		},
		{
			msg:         "invalid systemID",
			nginxID:     "nginx",
			systemID:    "",
			expRegister: false,
		},
		{
			msg:         "invalid nginxID and systemID",
			nginxID:     "",
			systemID:    "",
			expRegister: false,
		},
	}

	for _, test := range tests {
		t.Run(test.msg, func(t *testing.T) {
			g := NewGomegaWithT(t)

			conn := newConnection(
				"conn-id",
				zap.New(),
				new(exchangerfakes.FakeCommandExchanger),
			)

			g.Expect(conn.state).To(Equal(StateConnected))
			g.Expect(conn.nginxID).To(BeEmpty())
			g.Expect(conn.systemID).To(BeEmpty())

			err := conn.register(test.nginxID, test.systemID)
			if test.expRegister {
				g.Expect(err).To(BeNil())
				g.Expect(conn.state).To(Equal(StateRegistered))
				g.Expect(conn.nginxID).To(Equal(test.nginxID))
				g.Expect(conn.systemID).To(Equal(test.systemID))
			} else {
				g.Expect(err).ToNot(BeNil())
				g.Expect(conn.state).To(Equal(StateInvalid))
				g.Expect(conn.nginxID).To(BeEmpty())
				g.Expect(conn.systemID).To(BeEmpty())
			}
		})
	}
}

func TestGetFirstNginxID(t *testing.T) {
	tests := []struct {
		name    string
		expID   string
		details []*proto.NginxDetails
	}{
		{
			name: "details with many nginxes",
			details: []*proto.NginxDetails{
				{
					NginxId: "1",
				},
				{
					NginxId: "2",
				},
				{
					NginxId: "3",
				},
			},
			expID: "1",
		},
		{
			name:    "nil details",
			details: nil,
			expID:   "",
		},
		{
			name:    "empty details",
			details: []*proto.NginxDetails{},
			expID:   "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			id := getFirstNginxID(test.details)
			g.Expect(id).To(Equal(test.expID))
		})
	}
}

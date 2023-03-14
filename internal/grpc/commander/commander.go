package commander

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	"github.com/nginx/agent/sdk/v2/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// nolint:lll
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 github.com/nginx/agent/sdk/v2/proto.Commander_CommandChannelServer
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 github.com/nginx/agent/sdk/v2/proto.Commander_UploadServer

const serverUUIDKey = "uuid"

// Commander implements the proto.CommanderServer interface.
type Commander struct {
	agentMgr AgentManager
	logger   logr.Logger
}

// NewCommander returns a new instance of the Commander.
func NewCommander(logger logr.Logger, agentMgr AgentManager) *Commander {
	return &Commander{
		logger:   logger,
		agentMgr: agentMgr,
	}
}

// CommandChannel is a bidirectional streaming channel that is established by the agent and remains open for the
// agent's lifetime.
//
// On every invocation, the Commander will create a new connection with the UUID of the server,
// add the connection to the AgentManager, and invoke the connection's blocking Run method.
// If the UUID field is not present in the server's context metadata, no connection is created and an error is returned.
// Once the Run method returns, ths Commander will remove the connection from the AgentManager.
// This ensures that only active (connected) connections are tracked by the AgentManager.
func (c *Commander) CommandChannel(server proto.Commander_CommandChannelServer) error {
	c.logger.Info("Commander CommandChannel")

	id, err := getUUIDFromContext(server.Context())
	if err != nil {
		c.logger.Error(err, "cannot get the UUID of the agent")
		return err
	}

	c.logger.Info("New agent connection", "id", id)

	defer func() {
		c.logger.Info("Removing agent from manager")
		c.agentMgr.RemoveAgent(id)
	}()

	agentLogger := c.logger.WithValues("id", id)

	agentConn := newConnection(
		id,
		agentLogger.WithName("connection"),
		NewBidirectionalChannel(server, agentLogger.WithName("channel")),
	)

	c.logger.Info("Adding agent to manager")
	c.agentMgr.AddAgent(agentConn)

	return agentConn.run(server.Context())
}

// Download will be implemented in a future PR.
func (c *Commander) Download(_ *proto.DownloadRequest, _ proto.Commander_DownloadServer) error {
	return nil
}

func (c *Commander) Upload(_ proto.Commander_UploadServer) error {
	c.logger.Info("Commander Upload requested")

	return status.Error(codes.Unimplemented, "upload method is not implemented")
}

func getUUIDFromContext(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", errors.New("metadata is not provided")
	}

	vals := md.Get(serverUUIDKey)
	if len(vals) == 0 {
		return "", errors.New("uuid is not in metadata")
	}

	if len(vals) > 1 {
		return "", errors.New("more than one value for uuid in metadata")
	}

	return vals[0], nil
}

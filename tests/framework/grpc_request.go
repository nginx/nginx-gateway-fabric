package framework

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
)

type GRPCRequest struct {
	Headers map[string]string // optional metadata headers (e.g., Authorization: Basic ...)
	Address string            // host:port to dial (e.g., 127.0.0.1:80)
	Timeout time.Duration
}

// SendGRPCRequest performs a unary gRPC call to helloworld.Greeter/SayHello using generic Invoke.
func SendGRPCRequest(request GRPCRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), request.Timeout)
	defer cancel()

	// Extract port from addr and build a passthrough target with desired :authority.
	// Example: addr = "127.0.0.1:80", authority = "cafe.example.com" => target = "passthrough:///cafe.example.com:80".
	hostPort := strings.Split(request.Address, ":")
	authority := "cafe.example.com"
	target := request.Address
	if len(hostPort) > 1 {
		target = fmt.Sprintf("passthrough:///%s:%s", authority, hostPort[len(hostPort)-1])
	}

	// Override dialing to connect to the actual addr while preserving target's :authority.
	dialer := func(ctx context.Context, _ string) (net.Conn, error) {
		d := &net.Dialer{}
		return d.DialContext(ctx, "tcp", request.Address)
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer),
	}

	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		return fmt.Errorf("grpc dial failed: %w", err)
	}
	defer conn.Close()

	if len(request.Headers) > 0 {
		md := metadata.New(request.Headers)
		ctx = metadata.NewOutgoingContext(ctx, md)
	}

	// Use Empty messages to marshal/unmarshal; server will ignore empty request fields, and
	// client will ignore unknown response fields.
	in := &emptypb.Empty{}
	out := &emptypb.Empty{}
	if err := conn.Invoke(ctx, "/helloworld.Greeter/SayHello", in, out); err != nil {
		fmt.Println("gRPC request failed:", err)
		return err
	}
	return nil
}

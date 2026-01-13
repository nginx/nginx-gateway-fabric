package framework

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type gRPCRequest struct {
	Headers map[string]string // optional metadata headers (e.g., Authorization: Basic ...)
	Address string            // host:port to dial (e.g., 127.0.0.1:80)
	Timeout time.Duration
}

// SendGRPCRequest performs a unary gRPC call to helloworld.Greeter/SayHello using generic Invoke.
func sendGRPCRequest(request gRPCRequest) error {
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

	requestDetails := fmt.Sprintf(
		"Target: %s, Address: %s, Headers: %v, Timeout (µs): %d\n",
		target,
		request.Address,
		request.Headers,
		request.Timeout.Microseconds(),
	)
	GinkgoWriter.Printf("Sending gRPCrequest: %s", requestDetails)

	// Use Empty messages to marshal/unmarshal; server will ignore empty request fields, and
	// client will ignore unknown response fields.
	in := &emptypb.Empty{}
	out := &emptypb.Empty{}
	if err := conn.Invoke(ctx, "/helloworld.Greeter/SayHello", in, out); err != nil {
		GinkgoWriter.Printf("ERROR: gRPC request failed: %v\n", err)

		return err
	}

	return nil
}

func ExpectGRPCRequestToSucceed(
	timeout time.Duration,
	address string,
	opts ...Option,
) error {
	options := LogOptions(opts...)
	request := gRPCRequest{
		Headers: options.requestHeaders,
		Address: address,
		Timeout: timeout,
	}
	err := sendGRPCRequest(request)
	if err != nil {
		return fmt.Errorf("expected gRPC request to succeed, but got error: %w", err)
	}

	return nil
}

func ExpectUnauthenticatedGRPCRequest(
	timeout time.Duration,
	address string,
	opts ...Option,
) error {
	options := LogOptions(opts...)
	request := gRPCRequest{
		Headers: options.requestHeaders,
		Address: address,
		Timeout: timeout,
	}
	err := sendGRPCRequest(request)
	if err == nil {
		return errors.New("expected Unauthenticated error, but gRPC request succeeded")
	}

	// Verify the gRPC status code is Unauthenticated (HTTP 401 equivalent).
	if status.Code(err) != codes.Unauthenticated {
		return fmt.Errorf("expected gRPC code %s, got %s", codes.Unauthenticated, status.Code(err))
	}

	return nil
}

func Expect500GRPCResponse(
	timeout time.Duration,
	address string,
	opts ...Option,
) error {
	options := LogOptions(opts...)
	request := gRPCRequest{
		Headers: options.requestHeaders,
		Address: address,
		Timeout: timeout,
	}
	err := sendGRPCRequest(request)

	if err == nil {
		return errors.New("expected 500 error, but gRPC request succeeded")
	}

	// Verify gRPC reflects HTTP 500 via Unknown and message includes 500.
	if status.Code(err) != codes.Unknown || !strings.Contains(err.Error(), "500") {
		return fmt.Errorf("expected gRPC code %s with HTTP 500, got %s (err: %w)", codes.Unknown, status.Code(err), err)
	}

	return nil
}

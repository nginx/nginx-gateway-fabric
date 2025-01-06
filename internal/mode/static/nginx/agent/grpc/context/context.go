package context

import (
	"context"
)

// GrpcInfo for storing identity information for the gRPC client.
type GrpcInfo struct {
	IPAddress string `json:"ip_address"` // ip address of the agent
}

type contextGRPCKey struct{}

// NewGrpcContext returns a new context.Context that has the provided GrpcInfo attached.
func NewGrpcContext(ctx context.Context, r GrpcInfo) context.Context {
	return context.WithValue(ctx, contextGRPCKey{}, r)
}

// GrpcInfoFromContext returns the GrpcInfo saved in ctx if it exists.
// Returns false if there's no GrpcInfo in the context.
func GrpcInfoFromContext(ctx context.Context) (GrpcInfo, bool) {
	v, ok := ctx.Value(contextGRPCKey{}).(GrpcInfo)
	return v, ok
}

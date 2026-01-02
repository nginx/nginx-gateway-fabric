package grpcinfo_test

import (
	"testing"

	. "github.com/onsi/gomega"

	grpcContext "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/grpc/context"
)

func TestGrpcInfoInContext(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	grpcInfo := grpcContext.GrpcInfo{Token: "test"}

	newCtx := grpcContext.NewGrpcContext(t.Context(), grpcInfo)
	info, ok := grpcContext.FromContext(newCtx)
	g.Expect(ok).To(BeTrue())
	g.Expect(info).To(Equal(grpcInfo))
}

func TestGrpcInfoNotInContext(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	info, ok := grpcContext.FromContext(t.Context())
	g.Expect(ok).To(BeFalse())
	g.Expect(info).To(Equal(grpcContext.GrpcInfo{}))
}

package controller

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

//go:generate go tool moq -skip-ensure -pkg controllerfakes -out controllerfakes/fake_getter.go . Getter

// Getter gets a resource from the k8s API.
// It allows us to mock the client.Reader.Get method.
type Getter interface {
	// Get is from client.Reader.
	Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error
}

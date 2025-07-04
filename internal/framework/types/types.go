package types //nolint:revive // ignoring meaningless package name

import "sigs.k8s.io/controller-runtime/pkg/client"

// ObjectType is used when we only care about the type of client.Object.
// The fields of the client.Object may be empty.
type ObjectType client.Object

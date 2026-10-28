package types //nolint:revive, nolintlint // ignoring “meaningless package name” and the unused-nolint warning

import "sigs.k8s.io/controller-runtime/pkg/client"

// ObjectType is used when we only care about the type of client.Object.
// The fields of the client.Object may be empty.
type ObjectType client.Object

package grpc_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"

	agentgrpc "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/grpc"
)

func TestGetConnection(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	tracker := agentgrpc.NewConnectionsTracker()

	conn := agentgrpc.Connection{
		InstanceID: "instance1",
		ParentName: types.NamespacedName{Namespace: "default", Name: "parent1"},
	}
	tracker.Track("key1", conn)

	trackedConn := tracker.GetConnection("key1")
	g.Expect(trackedConn).To(Equal(conn))

	nonExistent := tracker.GetConnection("nonexistent")
	g.Expect(nonExistent).To(Equal(agentgrpc.Connection{}))
}

func TestConnectionIsReady(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	conn := agentgrpc.Connection{
		InstanceID: "instance1",
		ParentName: types.NamespacedName{Namespace: "default", Name: "parent1"},
	}

	g.Expect(conn.Ready()).To(BeTrue())
}

func TestConnectionIsNotReady(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	conn := agentgrpc.Connection{
		ParentName: types.NamespacedName{Namespace: "default", Name: "parent1"},
	}

	g.Expect(conn.Ready()).To(BeFalse())
}

func TestSetInstanceID(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	tracker := agentgrpc.NewConnectionsTracker()
	conn := agentgrpc.Connection{
		ParentName: types.NamespacedName{Namespace: "default", Name: "parent1"},
	}
	tracker.Track("key1", conn)

	trackedConn := tracker.GetConnection("key1")
	g.Expect(trackedConn.Ready()).To(BeFalse())

	tracker.SetInstanceID("key1", "instance1")

	trackedConn = tracker.GetConnection("key1")
	g.Expect(trackedConn.Ready()).To(BeTrue())
	g.Expect(trackedConn.InstanceID).To(Equal("instance1"))
}

func TestTrackPreservesInstanceID(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	tracker := agentgrpc.NewConnectionsTracker()

	// Track a fully-ready connection.
	original := agentgrpc.Connection{
		InstanceID: "instance1",
		ParentType: "Deployment",
		ParentName: types.NamespacedName{Namespace: "default", Name: "parent1"},
	}
	tracker.Track("key1", original)

	// Simulate the agent reconnecting with the same UUID but before it
	// has rediscovered its nginx instance (InstanceID is empty).
	reconnected := agentgrpc.Connection{
		ParentType: "Deployment",
		ParentName: types.NamespacedName{Namespace: "default", Name: "parent1"},
	}
	tracker.Track("key1", reconnected)

	// The existing InstanceID must be preserved so that in-flight
	// GetFile calls from the prior Subscribe stream succeed.
	trackedConn := tracker.GetConnection("key1")
	g.Expect(trackedConn.Ready()).To(BeTrue())
	g.Expect(trackedConn.InstanceID).To(Equal("instance1"))
	g.Expect(trackedConn.ParentName).To(Equal(reconnected.ParentName))
}

func TestTrackOverwritesInstanceID(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	tracker := agentgrpc.NewConnectionsTracker()

	original := agentgrpc.Connection{
		InstanceID: "instance1",
		ParentName: types.NamespacedName{Namespace: "default", Name: "parent1"},
	}
	tracker.Track("key1", original)

	// When the new connection supplies a non-empty InstanceID, the
	// value must be updated (e.g. the agent restarted on a new nginx).
	updated := agentgrpc.Connection{
		InstanceID: "instance2",
		ParentName: types.NamespacedName{Namespace: "default", Name: "parent1"},
	}
	tracker.Track("key1", updated)

	trackedConn := tracker.GetConnection("key1")
	g.Expect(trackedConn.InstanceID).To(Equal("instance2"))
}

func TestRemoveConnection(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	tracker := agentgrpc.NewConnectionsTracker()
	conn := agentgrpc.Connection{
		InstanceID: "instance1",
		ParentName: types.NamespacedName{Namespace: "default", Name: "parent1"},
	}
	tracker.Track("key1", conn)

	trackedConn := tracker.GetConnection("key1")
	g.Expect(trackedConn).To(Equal(conn))

	tracker.RemoveConnection("key1")
	g.Expect(tracker.GetConnection("key1")).To(Equal(agentgrpc.Connection{}))
}

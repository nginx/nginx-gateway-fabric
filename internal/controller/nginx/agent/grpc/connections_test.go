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
	conn.Generation = tracker.Track("key1", conn)

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

func TestRemoveConnection(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	tracker := agentgrpc.NewConnectionsTracker()
	conn := agentgrpc.Connection{
		InstanceID: "instance1",
		ParentName: types.NamespacedName{Namespace: "default", Name: "parent1"},
	}
	conn.Generation = tracker.Track("key1", conn)

	trackedConn := tracker.GetConnection("key1")
	g.Expect(trackedConn).To(Equal(conn))

	tracker.RemoveConnection("key1", conn.Generation)
	g.Expect(tracker.GetConnection("key1")).To(Equal(agentgrpc.Connection{}))
}

// TestRemoveConnection_StaleGenerationIsNoOp: a stale (old-generation) RemoveConnection must not
// delete a re-tracked connection.
func TestRemoveConnection_StaleGenerationIsNoOp(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	tracker := agentgrpc.NewConnectionsTracker()
	conn := agentgrpc.Connection{
		InstanceID: "instance1",
		ParentName: types.NamespacedName{Namespace: "default", Name: "parent1"},
	}

	staleGen := tracker.Track("key1", conn) // original stream
	liveGen := tracker.Track("key1", conn)  // reconnect re-tracks same key
	g.Expect(liveGen).ToNot(Equal(staleGen))

	// Stale stream's deferred cleanup must NOT wipe the live entry.
	tracker.RemoveConnection("key1", staleGen)
	liveConn := tracker.GetConnection("key1")
	g.Expect(liveConn.Ready()).To(BeTrue())
	g.Expect(liveConn.Generation).To(Equal(liveGen))

	// The live stream's own cleanup still removes it.
	tracker.RemoveConnection("key1", liveGen)
	g.Expect(tracker.GetConnection("key1")).To(Equal(agentgrpc.Connection{}))
}

package grpc_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"

	agentgrpc "github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/agent/grpc"
)

func TestGetConnection(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	tracker := agentgrpc.NewConnectionsTracker()

	conn := agentgrpc.Connection{
		PodName:    "pod1",
		InstanceID: "instance1",
		Parent:     types.NamespacedName{Namespace: "default", Name: "parent1"},
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
		PodName:    "pod1",
		InstanceID: "instance1",
		Parent:     types.NamespacedName{Namespace: "default", Name: "parent1"},
	}

	g.Expect(conn.Ready()).To(BeTrue())
}

func TestConnectionIsNotReady(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	conn := agentgrpc.Connection{
		PodName: "pod1",
		Parent:  types.NamespacedName{Namespace: "default", Name: "parent1"},
	}

	g.Expect(conn.Ready()).To(BeFalse())
}

func TestSetInstanceID(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	tracker := agentgrpc.NewConnectionsTracker()
	conn := agentgrpc.Connection{
		PodName: "pod1",
		Parent:  types.NamespacedName{Namespace: "default", Name: "parent1"},
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
		PodName:    "pod1",
		InstanceID: "instance1",
		Parent:     types.NamespacedName{Namespace: "default", Name: "parent1"},
	}
	tracker.Track("key1", conn)

	trackedConn := tracker.GetConnection("key1")
	g.Expect(trackedConn).To(Equal(conn))

	tracker.RemoveConnection("key1")
	g.Expect(tracker.GetConnection("key1")).To(Equal(agentgrpc.Connection{}))
}

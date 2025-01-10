package grpc

import (
	"sync"

	"k8s.io/apimachinery/pkg/types"
)

type Connection struct {
	PodName    string
	InstanceID string
	Parent     types.NamespacedName
}

// ConnectionsTracker keeps track of all connections between the control plane and nginx agents.
type ConnectionsTracker struct {
	// connections contains a map of all IP addresses that have connected and their connection info.
	connections map[string]Connection

	lock sync.RWMutex
}

// NewConnectionsTracker returns a new ConnectionsTracker instance.
func NewConnectionsTracker() *ConnectionsTracker {
	return &ConnectionsTracker{
		connections: make(map[string]Connection),
	}
}

// Track adds a connection to the tracking map.
// TODO(sberman): we need to handle the case when the token expires (once we support the token).
// This likely involves setting a callback to cancel a context when the token expires, which triggers
// the connection to be removed from the tracking list.
func (c *ConnectionsTracker) Track(key string, conn Connection) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.connections[key] = conn
}

// GetConnection returns the requested connection.
func (c *ConnectionsTracker) GetConnection(key string) Connection {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.connections[key]
}

// UntrackConnectionsForParent removes all Connections that reference the specified parent.
func (c *ConnectionsTracker) UntrackConnectionsForParent(parent types.NamespacedName) {
	c.lock.Lock()
	defer c.lock.Unlock()

	for key, conn := range c.connections {
		if conn.Parent == parent {
			delete(c.connections, key)
		}
	}
}

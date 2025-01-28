package grpc

import (
	"sync"

	"k8s.io/apimachinery/pkg/types"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . ConnectionsTracker

// ConnectionsTracker defines an interface to track all connections between the control plane
// and nginx agents.
type ConnectionsTracker interface {
	Track(key string, conn Connection)
	GetConnection(key string) Connection
	SetInstanceID(key, id string)
	UntrackConnectionsForParent(parent types.NamespacedName)
}

// Connection contains the data about a single nginx agent connection.
type Connection struct {
	PodName    string
	InstanceID string
	Parent     types.NamespacedName
}

// Ready returns if the connection is ready to be used. In other words, agent
// has registered itself and an nginx instance with the control plane.
func (c *Connection) Ready() bool {
	return c.InstanceID != ""
}

// AgentConnectionsTracker keeps track of all connections between the control plane and nginx agents.
type AgentConnectionsTracker struct {
	// connections contains a map of all IP addresses that have connected and their connection info.
	connections map[string]Connection

	lock sync.RWMutex
}

// NewConnectionsTracker returns a new AgentConnectionsTracker instance.
func NewConnectionsTracker() ConnectionsTracker {
	return &AgentConnectionsTracker{
		connections: make(map[string]Connection),
	}
}

// Track adds a connection to the tracking map.
// TODO(sberman): we need to handle the case when the token expires (once we support the token).
// This likely involves setting a callback to cancel a context when the token expires, which triggers
// the connection to be removed from the tracking list.
func (c *AgentConnectionsTracker) Track(key string, conn Connection) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.connections[key] = conn
}

// GetConnection returns the requested connection.
func (c *AgentConnectionsTracker) GetConnection(key string) Connection {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.connections[key]
}

// SetInstanceID sets the nginx instanceID for a connection.
func (c *AgentConnectionsTracker) SetInstanceID(key, id string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if conn, ok := c.connections[key]; ok {
		conn.InstanceID = id
		c.connections[key] = conn
	}
}

// UntrackConnectionsForParent removes all Connections that reference the specified parent.
func (c *AgentConnectionsTracker) UntrackConnectionsForParent(parent types.NamespacedName) {
	c.lock.Lock()
	defer c.lock.Unlock()

	for key, conn := range c.connections {
		if conn.Parent == parent {
			delete(c.connections, key)
		}
	}
}

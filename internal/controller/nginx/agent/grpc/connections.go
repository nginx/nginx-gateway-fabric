package grpc

import (
	"sync"

	"k8s.io/apimachinery/pkg/types"
)

//go:generate go tool counterfeiter -generate

//counterfeiter:generate . ConnectionsTracker

// ConnectionsTracker defines an interface to track all connections between the control plane
// and nginx agents.
type ConnectionsTracker interface {
	Track(key string, conn Connection) uint64
	GetConnection(key string) Connection
	Generation(key string) uint64
	SetInstanceID(key, id string)
	RemoveConnection(key string, generation uint64)
}

// Connection contains the data about a single nginx agent connection.
type Connection struct {
	InstanceID string
	ParentType string
	ParentName types.NamespacedName
}

// Ready returns if the connection is ready to be used. In other words, agent
// has registered itself and an nginx instance with the control plane.
func (c *Connection) Ready() bool {
	return c.InstanceID != ""
}

// trackedConnection wraps a Connection with a generation counter so stale removals don't evict live connections.
type trackedConnection struct {
	Connection
	generation uint64
}

// AgentConnectionsTracker keeps track of all connections between the control plane and nginx agents.
type AgentConnectionsTracker struct {
	// connections contains a map of all IP addresses that have connected and their connection info.
	connections map[string]trackedConnection

	lock       sync.RWMutex
	generation uint64
}

// NewConnectionsTracker returns a new AgentConnectionsTracker instance.
func NewConnectionsTracker() ConnectionsTracker {
	return &AgentConnectionsTracker{
		connections: make(map[string]trackedConnection),
	}
}

// Track adds a connection to the tracking map and returns the generation it was tracked under.
func (c *AgentConnectionsTracker) Track(key string, conn Connection) uint64 {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.generation++
	c.connections[key] = trackedConnection{Connection: conn, generation: c.generation}

	return c.generation
}

// GetConnection returns the requested connection.
func (c *AgentConnectionsTracker) GetConnection(key string) Connection {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.connections[key].Connection
}

// Generation returns the generation the connection is currently tracked under, 0 if untracked.
func (c *AgentConnectionsTracker) Generation(key string) uint64 {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.connections[key].generation
}

// SetInstanceID sets the nginx instanceID for a connection.
func (c *AgentConnectionsTracker) SetInstanceID(key, id string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if tc, ok := c.connections[key]; ok {
		tc.InstanceID = id
		c.connections[key] = tc
	}
}

// RemoveConnection removes a connection only if its generation matches, preventing a stale stream
// from evicting a connection re-established under the same key.
func (c *AgentConnectionsTracker) RemoveConnection(key string, generation uint64) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if tc, ok := c.connections[key]; ok && tc.generation == generation {
		delete(c.connections, key)
	}
}

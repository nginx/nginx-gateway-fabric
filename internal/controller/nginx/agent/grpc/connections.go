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
	Track(key string, conn Connection)
	GetConnection(key string) Connection
	SetInstanceID(key, id string)
	RemoveConnection(key string)
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
// If a connection already exists for the given key and has a valid InstanceID,
// and the new connection does not, the existing InstanceID is preserved.
// This prevents a reconnecting agent (which may not yet have rediscovered
// its nginx instance) from temporarily invalidating in-flight config apply
// operations on the existing Subscribe stream.
func (c *AgentConnectionsTracker) Track(key string, conn Connection) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if existing, ok := c.connections[key]; ok && conn.InstanceID == "" && existing.InstanceID != "" {
		conn.InstanceID = existing.InstanceID
	}

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

// RemoveConnection removes a connection from the tracking map.
func (c *AgentConnectionsTracker) RemoveConnection(key string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	delete(c.connections, key)
}

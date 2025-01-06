package grpc

import (
	"sync"
)

// ConnectionsTracker keeps track of all connections between the control plane and nginx agents.
type ConnectionsTracker struct {
	// connections contains a map of all IP addresses that have connected and their associated pod names.
	// TODO(sberman): we'll likely need to create a channel for each connection that can be stored in this map.
	// Then the Subscription listens on the channel for its connection, while the nginxUpdater sends the config
	// for the pod over that channel.
	connections map[string]string

	lock sync.Mutex
}

// NewConnectionsTracker returns a new ConnectionsTracker instance.
func NewConnectionsTracker() *ConnectionsTracker {
	return &ConnectionsTracker{
		connections: make(map[string]string),
	}
}

// Track adds a connection to the tracking map.
// TODO(sberman): we need to handle the case when the token expires (once we support the token).
// This likely involves setting a callback to cancel a context when the token expires, which triggers
// the connection to be removed from the tracking list.
func (c *ConnectionsTracker) Track(address, hostname string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.connections[address] = hostname
}

// GetConnections returns all connections that are currently tracked.
func (c *ConnectionsTracker) GetConnections() map[string]string {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.connections
}

// GetConnection returns the hostname of the requested connection.
func (c *ConnectionsTracker) GetConnection(address string) string {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.connections[address]
}

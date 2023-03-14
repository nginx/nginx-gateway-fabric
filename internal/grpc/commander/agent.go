package commander

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . AgentManager
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Agent

// Agent represents a connected Agent.
// This interface is used for testing purposes because it allows easy mocking of an agent.
type Agent interface {
	// ID returns the unique ID of the Agent.
	ID() string
	// State returns the State of the Agent.
	State() State
}

// AgentManager manages all the connected agents.
// The commander uses the AgentManager to track all the connected Agents.
type AgentManager interface {
	// AddAgent adds an Agent to the manager.
	AddAgent(agent Agent)
	// RemoveAgent removes the Agent with the provided ID from the manager.
	RemoveAgent(id string)
	// GetAgent returns the Agent with the provided ID.
	GetAgent(id string) Agent
}

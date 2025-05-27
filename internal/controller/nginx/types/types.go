package types

const (
	// Nginx503Server is used as a backend for services that cannot be resolved (have no IP address).
	Nginx503Server = "unix:/var/run/nginx/nginx-503-server.sock"
)

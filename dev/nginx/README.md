# Deploy NGINX locally for testing

This guide provides an easy way to deploy and experiment with NGINX configs locally

## Quick Start

```bash
# Deploy nginx and nginx-hello server
make deploy

# Port-forward nginx pod (in separate terminal)
make port-forward

# Test it works
make test

# Clean up
make cleanup
```

## Available Commands

- `make deploy` - Deploy nginx and nginx-hello server to Kubernetes
- `make port-forward` - Port forward nginx pod to localhost:8080
- `make test` - Test the setup via curl (assuming port-forward is running)
- `make update` - Update config and restart pods
- `make logs` - View nginx logs
- `make cleanup` - Delete everything

## How it works

1. NGINX pod proxies all requests to nginx-hello server pod
2. Use port-forward to access nginx on localhost:8080
3. Edit `nginx.conf` directly and run `make update`

## URLs (after port-forward)

- http://localhost:8080/health - NGINX health check
- http://localhost:8080/ - Proxied to nginx-hello server

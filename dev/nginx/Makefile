PORT ?= 8080

.PHONY: help deploy cleanup logs update port-forward test create-license-secret

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'
	@echo "\nVariables:"
	@echo "  PORT=8080    Port for port-forwarding (default: 8080)"
	@echo "  LICENSE_FILE=/apth/to/license.jwt   Path to NGINX Plus license token file"

deploy: ## Deploy nginx and nginx-hello server to Kubernetes
	kubectl create configmap nginx-config --from-file=nginx.conf --dry-run=client -o yaml | kubectl apply -f -
	kubectl apply -f nginx.yaml
	@echo "Waiting for pods to be ready..."
	kubectl wait --for=condition=ready pod -l app=nginx-proxy --timeout=60s
	kubectl wait --for=condition=ready pod -l app=nginx-hello --timeout=60s

deploy-plus : ## Deploy nginx with NGINX Plus license (requires LICENSE_FILE=/abs/path/license.jwt)
	@if [ -z "$(LICENSE_FILE)" ]; then echo "ERROR: LICENSE_FILE not set. Usage: make deploy-plus LICENSE_FILE=/abs/path/license.jwt"; exit 1; fi
	make create-license-secret LICENSE_FILE=$(LICENSE_FILE) || true
	make deploy

cleanup: ## Delete everything from Kubernetes
	kubectl delete configmap nginx-config --ignore-not-found=true
	kubectl delete -f nginx.yaml --ignore-not-found=true

logs: ## Show nginx logs
	kubectl logs -l app=nginx-proxy -f

update: ## Update nginx config and restart pods
	kubectl create configmap nginx-config --from-file=nginx.conf --dry-run=client -o yaml | kubectl apply -f -
	kubectl rollout restart deployment/nginx-proxy

port-forward: ## Port forward nginx pod to localhost:PORT (default: 8080)
	kubectl port-forward deployment/nginx-proxy $(PORT):80

test: ## Test the setup via port-forward (run 'make port-forward' first)
	@echo "Testing nginx health..."
	@curl -s http://localhost:$(PORT)/health
	@echo "\nTesting nginx-hello backend..."
	@curl -s http://localhost:$(PORT)/ | head -5

create-license-secret: ## Create the nginx-plus-license secret (requires LICENSE_FILE=/abs/path/license.jwt)
	@if [ -z "$(LICENSE_FILE)" ]; then echo "ERROR: LICENSE_FILE not set. Usage: make create-license-secret LICENSE_FILE=/abs/path/license.jwt"; exit 1; fi
	kubectl -n default create secret generic nginx-plus-license --from-file=license.jwt=$(LICENSE_FILE) --dry-run=client -o yaml | kubectl apply -f -

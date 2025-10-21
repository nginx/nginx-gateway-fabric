# Makefile to build and run the NGINX + Rust EPP POC locally with Docker
# Requires: Docker
#
# Workflow:
#   make up         # build image, start backend and nginx containers
#   make curl-test  # send a request that includes simulated EPP header
#   make logs       # tail nginx logs
#   make stop       # stop containers
#   make clean      # stop + remove image

IMAGE         ?= ngf-rust-epp:dev
NET           ?= ngf-epp-net
BACKEND_NAME  ?= backend1
BACKEND_PORT  ?= 8080
NGINX_CONT    ?= ngf-rust-epp
NGINX_PORT    ?= 8080

.PHONY: docker-build net-create backend-run nginx-run up curl-test logs stop clean

docker-build:
	docker build -f dev/rust-nginx-epp/Dockerfile -t $(IMAGE) .

net-create:
	@docker network inspect $(NET) >/dev/null 2>&1 || docker network create $(NET)

backend-run: net-create
	-@docker rm -f $(BACKEND_NAME) >/dev/null 2>&1 || true
	# Simple backend that serves HTTP on $(BACKEND_PORT)
	 docker run -d --name $(BACKEND_NAME) --network $(NET) -p $(BACKEND_PORT):$(BACKEND_PORT) \
	  python:3-alpine sh -c "python -m http.server $(BACKEND_PORT) -b 0.0.0.0"

nginx-run: net-create
	-@docker rm -f $(NGINX_CONT) >/dev/null 2>&1 || true
	 docker run -d --name $(NGINX_CONT) --network $(NET) -p $(NGINX_PORT):80 $(IMAGE)

up: docker-build backend-run nginx-run
	@echo "Containers are up:"
	@docker ps --filter "name=$(BACKEND_NAME)" --filter "name=$(NGINX_CONT)"

curl-test:
	# Simulate EPP selection with special header understood by the POC stub
	 curl -v -H "test-epp-endpoint-selection: $(BACKEND_NAME):$(BACKEND_PORT)" \
	  "http://localhost:$(NGINX_PORT)/inference/"

logs:
	docker logs -f $(NGINX_CONT)

stop:
	-@docker rm -f $(NGINX_CONT) >/dev/null 2>&1 || true
	-@docker rm -f $(BACKEND_NAME) >/dev/null 2>&1 || true

clean: stop
	-@docker rmi $(IMAGE) >/dev/null 2>&1 || true

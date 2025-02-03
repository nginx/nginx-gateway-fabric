# variables that should not be overridden by the user
VERSION = edge
SELF_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
CHART_DIR = $(SELF_DIR)charts/nginx-gateway-fabric
NGINX_CONF_DIR = internal/mode/static/nginx/conf
NJS_DIR = internal/mode/static/nginx/modules/src
KIND_CONFIG_FILE = $(SELF_DIR)config/cluster/kind-cluster.yaml
NGINX_DOCKER_BUILD_PLUS_ARGS = --secret id=nginx-repo.crt,src=$(SELF_DIR)nginx-repo.crt --secret id=nginx-repo.key,src=$(SELF_DIR)nginx-repo.key
BUILD_AGENT = local

PROD_TELEMETRY_ENDPOINT = oss.edge.df.f5.com:443
# the telemetry related variables below are also configured in goreleaser.yml
TELEMETRY_REPORT_PERIOD = 24h
TELEMETRY_ENDPOINT=# if empty, NGF will report telemetry in its logs at debug level.
TELEMETRY_ENDPOINT_INSECURE = false

ENABLE_EXPERIMENTAL ?= false

# go build flags - should not be overridden by the user
GO_LINKER_FlAGS_VARS = -X main.version=${VERSION} -X main.telemetryReportPeriod=${TELEMETRY_REPORT_PERIOD} -X main.telemetryEndpoint=${TELEMETRY_ENDPOINT} -X main.telemetryEndpointInsecure=${TELEMETRY_ENDPOINT_INSECURE}
GO_LINKER_FLAGS_OPTIMIZATIONS = -s -w
GO_LINKER_FLAGS = $(GO_LINKER_FLAGS_OPTIMIZATIONS) $(GO_LINKER_FlAGS_VARS)

# tools versions
# renovate: datasource=github-tags depName=golangci/golangci-lint
GOLANGCI_LINT_VERSION = v1.63.4
# renovate: datasource=docker depName=kindest/node
KIND_K8S_VERSION = v1.32.0
# renovate: datasource=github-tags depName=norwoodj/helm-docs
HELM_DOCS_VERSION = v1.14.2
# renovate: datasource=github-tags depName=ahmetb/gen-crd-api-reference-docs
GEN_CRD_API_REFERENCE_DOCS_VERSION = v0.3.0
# renovate: datasource=go depName=sigs.k8s.io/controller-tools
CONTROLLER_TOOLS_VERSION = v0.17.1
# renovate: datasource=docker depName=node
NODE_VERSION = 22
# renovate: datasource=docker depName=quay.io/helmpack/chart-testing
CHART_TESTING_VERSION = v3.12.0
# renovate: datasource=github-tags depName=dadav/helm-schema
HELM_SCHEMA_VERSION = 0.18.1

# variables that can be overridden by the user
PREFIX ?= nginx-gateway-fabric## The name of the NGF image. For example, nginx-gateway-fabric
NGINX_PREFIX ?= $(PREFIX)/nginx## The name of the nginx image. For example: nginx-gateway-fabric/nginx
NGINX_PLUS_PREFIX ?= $(PREFIX)/nginx-plus## The name of the nginx plus image. For example: nginx-gateway-fabric/nginx-plus
TAG ?= $(VERSION:v%=%)## The tag of the image. For example, 1.1.0
TARGET ?= local## The target of the build. Possible values: local and container
OUT_DIR ?= build/out## The folder where the binary will be stored
GOARCH ?= amd64## The architecture of the image and/or binary. For example: amd64 or arm64
GOOS ?= linux## The OS of the image and/or binary. For example: linux or darwin
PLUS_ENABLED ?= false
PLUS_LICENSE_FILE ?= $(SELF_DIR)license.jwt
PLUS_USAGE_ENDPOINT ?=## The N+ usage endpoint. For development, please set to the N1 staging endpoint.

override NGINX_DOCKER_BUILD_OPTIONS += --build-arg NJS_DIR=$(NJS_DIR) --build-arg NGINX_CONF_DIR=$(NGINX_CONF_DIR) --build-arg BUILD_AGENT=$(BUILD_AGENT)

.DEFAULT_GOAL := help

ifneq (,$(findstring plus,$(MAKECMDGOALS)))
   PLUS_ENABLED = true
endif

.PHONY: help
help: Makefile ## Display this help
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "; printf "Usage:\n\n	make \033[36m<target>\033[0m [VARIABLE=value...]\n\nTargets:\n\n"}; {printf "	 \033[36m%-30s\033[0m %s\n", $$1, $$2}'
	@grep -hE '^(override )?[a-zA-Z_-]+ \??\+?= .*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = " \\??\\+?= .*?## "; printf "\nVariables:\n\n"}; {gsub(/override /, "", $$1); printf "	 \033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: build-prod-images
build-prod-images: build-prod-ngf-image build-prod-nginx-image ## Build the NGF and nginx docker images for production

.PHONY: build-prod-images-with-plus
build-prod-images-with-plus: build-prod-ngf-image build-prod-nginx-plus-image ## Build the NGF and NGINX Plus docker images for production

.PHONY: build-images
build-images: build-ngf-image build-nginx-image ## Build the NGF and nginx docker images

.PHONY: build-images-with-plus
build-images-with-plus: build-ngf-image build-nginx-plus-image ## Build the NGF and NGINX Plus docker images

.PHONY: build-prod-ngf-image
build-prod-ngf-image: TELEMETRY_ENDPOINT=$(PROD_TELEMETRY_ENDPOINT)
build-prod-ngf-image: build-ngf-image ## Build the NGF docker image for production

.PHONY: build-ngf-image
build-ngf-image: check-for-docker build ## Build the NGF docker image
	docker build --platform linux/$(GOARCH) --build-arg BUILD_AGENT=$(BUILD_AGENT) --target $(strip $(TARGET)) -f $(SELF_DIR)build/Dockerfile -t $(strip $(PREFIX)):$(strip $(TAG)) $(strip $(SELF_DIR))

.PHONY: build-prod-nginx-image
build-prod-nginx-image: build-nginx-image ## Build the custom nginx image for production

.PHONY: build-nginx-image
build-nginx-image: check-for-docker ## Build the custom nginx image
	docker build --platform linux/$(GOARCH) $(strip $(NGINX_DOCKER_BUILD_OPTIONS)) -f $(SELF_DIR)build/Dockerfile.nginx -t $(strip $(NGINX_PREFIX)):$(strip $(TAG)) $(strip $(SELF_DIR))

.PHONY: build-prod-nginx-plus-image
build-prod-nginx-plus-image: build-nginx-plus-image ## Build the custom nginx plus image for production

.PHONY: build-nginx-plus-image
build-nginx-plus-image: check-for-docker ## Build the custom nginx plus image
	docker build --platform linux/$(GOARCH) $(strip $(NGINX_DOCKER_BUILD_OPTIONS)) $(strip $(NGINX_DOCKER_BUILD_PLUS_ARGS))  -f $(SELF_DIR)build/Dockerfile.nginxplus -t $(strip $(NGINX_PLUS_PREFIX)):$(strip $(TAG)) $(strip $(SELF_DIR))

.PHONY: check-for-docker
check-for-docker: ## Check if Docker is installed
	@docker -v || (code=$$?; printf "\033[0;31mError\033[0m: there was a problem with Docker\n"; exit $$code)

.PHONY: build
build: ## Build the binary
ifeq (${TARGET},local)
	@go version || (code=$$?; printf "\033[0;31mError\033[0m: unable to build locally\n"; exit $$code)
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -C $(SELF_DIR) -trimpath -a -ldflags "$(GO_LINKER_FLAGS)" $(ADDITIONAL_GO_BUILD_FLAGS) -o $(OUT_DIR)/gateway github.com/nginx/nginx-gateway-fabric/cmd/gateway
endif

.PHONY: build-goreleaser
build-goreleaser: ## Build the binary using GoReleaser
	@goreleaser -v || (code=$$?; printf "\033[0;31mError\033[0m: there was a problem with GoReleaser. Follow the docs to install it https://goreleaser.com/install\n"; exit $$code)
	GOOS=linux GOPATH=$(shell go env GOPATH) GOARCH=$(GOARCH) goreleaser build --clean --snapshot --single-target

.PHONY: generate
generate: ## Run go generate
	go generate ./...

.PHONY: generate-crds
generate-crds: ## Generate CRDs and Go types using kubebuilder
	go run sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION) crd object paths=./apis/... output:crd:artifacts:config=config/crd/bases
	kubectl kustomize config/crd >deploy/crds.yaml

.PHONY: install-crds
install-crds: ## Install CRDs
	kubectl kustomize $(SELF_DIR)config/crd | kubectl apply -f -

.PHONY: install-gateway-crds
install-gateway-crds: ## Install Gateway API CRDs
	kubectl kustomize $(SELF_DIR)config/crd/gateway-api/$(if $(filter true,$(ENABLE_EXPERIMENTAL)),experimental,standard) | kubectl apply -f -

.PHONY: uninstall-gateway-crds
uninstall-gateway-crds: ## Uninstall Gateway API CRDs
	kubectl kustomize $(SELF_DIR)config/crd/gateway-api/$(if $(filter true,$(ENABLE_EXPERIMENTAL)),experimental,standard) | kubectl delete -f -

.PHONY: generate-manifests
generate-manifests: ## Generate manifests using Helm.
	./scripts/generate-manifests.sh

generate-api-docs: ## Generate API docs
	go run github.com/ahmetb/gen-crd-api-reference-docs@$(GEN_CRD_API_REFERENCE_DOCS_VERSION) -config site/config/api/config.json -template-dir site/config/api -out-file site/content/reference/api.md -api-dir "github.com/nginx/nginx-gateway-fabric/apis"

.PHONY: generate-helm-docs
generate-helm-docs: ## Generate the Helm chart documentation
	go run github.com/norwoodj/helm-docs/cmd/helm-docs@$(HELM_DOCS_VERSION) --chart-search-root=charts --template-files _templates.gotmpl --template-files README.md.gotmpl

.PHONY: generate-helm-schema
generate-helm-schema: ## Generate the Helm chart schema
	go run github.com/dadav/helm-schema/cmd/helm-schema@$(HELM_SCHEMA_VERSION) --chart-search-root=charts --add-schema-reference "--skip-auto-generation=required,additionalProperties" --append-newline

.PHONY: generate-all
generate-all: generate generate-crds generate-helm-schema generate-manifests generate-api-docs generate-helm-docs ## Generate all the necessary files

.PHONY: clean
clean: ## Clean the build
	-rm -r $(OUT_DIR)

.PHONY: clean-go-cache
clean-go-cache: ## Clean go cache
	@go clean -modcache

.PHONY: deps
deps: ## Add missing and remove unused modules, verify deps and download them to local cache
	@go mod tidy && go mod verify && go mod download

.PHONY: create-kind-cluster
create-kind-cluster: ## Create a kind cluster
	@kind version || (code=$$?; printf "\033[0;31mError\033[0m: there was a problem with kind. Follow the docs to install it https://kind.sigs.k8s.io/docs/user/quick-start/\n"; exit $$code)
	kind create cluster --image kindest/node:$(KIND_K8S_VERSION) --config $(KIND_CONFIG_FILE)

.PHONY: delete-kind-cluster
delete-kind-cluster: ## Delete kind cluster
	kind delete cluster

.PHONY: fmt
fmt: ## Run go fmt against code
	go fmt ./...

.PHONY: njs-fmt
njs-fmt: ## Run prettier against the njs httpmatches module
	docker run --rm -w /modules \
		-v $(CURDIR)/internal/nginx/modules/:/modules/ \
		node:${NODE_VERSION} \
		/bin/bash -c "npm ci && npm run format"

.PHONY: vet
vet: ## Run go vet against code
	go vet ./...

.PHONY: lint
lint: ## Run golangci-lint against code
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run --fix

.PHONY: unit-test
unit-test: ## Run unit tests for the go code
	go test ./cmd/... ./internal/... -buildvcs -race -shuffle=on -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o cover.html

.PHONY: njs-unit-test
njs-unit-test: ## Run unit tests for the njs httpmatches module
	docker run --rm -w /modules \
		-v $(CURDIR)/internal/mode/static/nginx/modules:/modules/ \
		node:${NODE_VERSION} \
		/bin/bash -c "npm ci && npm test && npm run clean"

.PHONY: lint-helm
lint-helm: ## Run the helm chart linter
	docker run --pull always --rm -v $(CURDIR):/nginx-gateway-fabric -w /nginx-gateway-fabric quay.io/helmpack/chart-testing:$(CHART_TESTING_VERSION) ct lint --config .ct.yaml

.PHONY: load-images
load-images: ## Load NGF and NGINX images on configured kind cluster.
	kind load docker-image $(PREFIX):$(TAG) $(NGINX_PREFIX):$(TAG)

.PHONY: load-images-with-plus
load-images-with-plus: ## Load NGF and NGINX Plus images on configured kind cluster.
	kind load docker-image $(PREFIX):$(TAG) $(NGINX_PLUS_PREFIX):$(TAG)

.PHONY: install-ngf-local-build
install-ngf-local-build: build-images load-images helm-install-local ## Install NGF from local build on configured kind cluster.

.PHONY: install-ngf-local-build-with-plus
install-ngf-local-build-with-plus: check-for-plus-usage-endpoint build-images-with-plus load-images-with-plus helm-install-local-with-plus ## Install NGF with NGINX Plus from local build on configured kind cluster.

.PHONY: helm-install-local
helm-install-local: install-gateway-crds ## Helm install NGF on configured kind cluster with local images. To build, load, and install with helm run make install-ngf-local-build.
	helm install nginx-gateway $(CHART_DIR) --set nginx.image.repository=$(NGINX_PREFIX) --create-namespace --wait --set nginxGateway.image.pullPolicy=Never --set service.type=NodePort --set nginxGateway.image.repository=$(PREFIX) --set nginxGateway.image.tag=$(TAG) --set nginx.image.tag=$(TAG) --set nginx.image.pullPolicy=Never --set nginxGateway.gwAPIExperimentalFeatures.enable=$(ENABLE_EXPERIMENTAL) -n nginx-gateway $(HELM_PARAMETERS)

.PHONY: helm-install-local-with-plus
helm-install-local-with-plus: check-for-plus-usage-endpoint install-gateway-crds ## Helm install NGF with NGINX Plus on configured kind cluster with local images. To build, load, and install with helm run make install-ngf-local-build-with-plus.
	kubectl create namespace nginx-gateway || true
	kubectl -n nginx-gateway create secret generic nplus-license --from-file $(PLUS_LICENSE_FILE) || true
	helm install nginx-gateway $(CHART_DIR) --set nginx.image.repository=$(NGINX_PLUS_PREFIX) --wait --set nginxGateway.image.pullPolicy=Never --set service.type=NodePort --set nginxGateway.image.repository=$(PREFIX) --set nginxGateway.image.tag=$(TAG) --set nginx.image.tag=$(TAG) --set nginx.image.pullPolicy=Never --set nginxGateway.gwAPIExperimentalFeatures.enable=$(ENABLE_EXPERIMENTAL) -n nginx-gateway --set nginx.plus=true --set nginx.usage.endpoint=$(PLUS_USAGE_ENDPOINT) $(HELM_PARAMETERS)

.PHONY: check-for-plus-usage-endpoint
check-for-plus-usage-endpoint: ## Checks that the PLUS_USAGE_ENDPOINT is set in the environment. This env var is required when deploying or testing with N+.
ifndef PLUS_USAGE_ENDPOINT
	$(error PLUS_USAGE_ENDPOINT must be defined in your environment)
endif

# Debug Targets
.PHONY: debug-build
debug-build: GO_LINKER_FLAGS=$(GO_LINKER_FlAGS_VARS)
debug-build: ADDITIONAL_GO_BUILD_FLAGS=-gcflags "all=-N -l"
debug-build: build ## Build binary with debug info, symbols, and no optimizations

.PHONY: debug-build-dlv-image
debug-build-dlv-image: check-for-docker ## Build the dlv debugger image.
	docker build --platform linux/$(GOARCH) -f debug/Dockerfile -t dlv-debug:edge .

.PHONY: debug-build-images
debug-build-images: debug-build build-ngf-image build-nginx-image debug-build-dlv-image ## Build all images used in debugging.

.PHONY: debug-build-images-with-plus
debug-build-images-with-plus: debug-build build-ngf-image build-nginx-plus-image debug-build-dlv-image ## Build all images with NGINX plus used in debugging.

.PHONY: debug-load-images
debug-load-images: load-images ## Load all images used in debugging to kind cluster.
	kind load docker-image dlv-debug:edge

.PHONY: debug-load-images-with-plus
debug-load-images-with-plus: load-images-with-plus ## Load all images with NGINX Plus used in debugging to kind cluster.
	kind load docker-image dlv-debug:edge

.PHONY: debug-install-local-build
debug-install-local-build: debug-build-images debug-load-images helm-install-local ## Install NGF from local build using debug NGF binary on configured kind cluster.

.PHONY: debug-install-local-build-with-plus
debug-install-local-build-with-plus: debug-build-images-with-plus debug-load-images-with-plus helm-install-local-with-plus ## Install NGF with NGINX Plus from local build using debug NGF binary on configured kind cluster.

.PHONY: dev-all
dev-all: deps fmt njs-fmt vet lint unit-test njs-unit-test ## Run all the development checks

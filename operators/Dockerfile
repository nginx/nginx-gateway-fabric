# syntax=docker/dockerfile:1.27
FROM --platform=$BUILDPLATFORM golang:1.27 AS builder
ARG TARGETARCH
ARG TARGETOS

WORKDIR /workspace

ARG HELM_OPERATOR_COMMIT=299157a814e674f8f40d1b91ee77d64a90e850de
# Fetch the release tags along with the commit so Go stamps the binary with a real
# version (v1.42.x). A tagless shallow fetch stamps v0.0.0-..., which scanners
# flag as vulnerable to GO-2025-3852 (fixed in v0.15.2).
RUN git init . && \
    git remote add origin https://github.com/operator-framework/operator-sdk.git && \
    git fetch --filter=tree:0 origin ${HELM_OPERATOR_COMMIT} 'refs/tags/v1.*:refs/tags/v1.*' && \
    git checkout ${HELM_OPERATOR_COMMIT}
# Upstream lags on dependency updates, so bump the modules that carry fixed CVEs.
# Drop entries once upstream catches up.
RUN go get \
        github.com/containerd/containerd@v1.7.36 \
        github.com/google/cel-go@v0.30.0 \
        github.com/klauspost/compress@v1.18.7 \
        go.opentelemetry.io/otel@v1.45.0 \
        go.opentelemetry.io/otel/sdk@v1.45.0 \
        go.opentelemetry.io/otel/exporters/otlp/otlptrace@v1.45.0 \
        go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc@v1.45.0 \
        golang.org/x/crypto@v0.57.0 \
        golang.org/x/net@v0.60.0 \
        golang.org/x/text@v0.42.0 \
        google.golang.org/grpc@v1.83.2 \
        helm.sh/helm/v3@v3.20.2 \
        oras.land/oras-go/v2@v2.6.2 && \
    go mod tidy
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH make build/helm-operator

FROM registry.access.redhat.com/ubi10/ubi-minimal:10.2

# Update system packages to fix vulnerabilities
USER root
RUN microdnf update -y --refresh --nobest && microdnf clean all

ENV HOME=/opt/helm \
    USER_NAME=helm \
    USER_UID=1001

RUN echo "${USER_NAME}:x:${USER_UID}:0:${USER_NAME} user:${HOME}:/sbin/nologin" >> /etc/passwd

WORKDIR ${HOME}
USER ${USER_UID}

COPY --from=builder /workspace/build/helm-operator /usr/local/bin/helm-operator

COPY LICENSE /licenses/LICENSE

LABEL name="nginx-gateway-fabric-operator" \
      maintainer="kubernetes@nginx.com" \
      vendor="F5 NGINX" \
      version="edge" \
      release="1" \
      summary="NGINX Gateway Fabric Operator" \
      description="Helm-based operator for NGINX Gateway Fabric"

COPY operators/watches.yaml ${HOME}/watches.yaml
COPY charts ${HOME}/helm-charts
COPY config ${HOME}/config

ENTRYPOINT ["/usr/local/bin/helm-operator", "run", "--watches-file=./watches.yaml"]

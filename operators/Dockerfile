# syntax=docker/dockerfile:1.23
FROM --platform=$BUILDPLATFORM golang:1.26 AS builder
ARG TARGETARCH
ARG TARGETOS

WORKDIR /workspace

ARG HELM_OPERATOR_COMMIT=dc231827cd920fdd15533bc134b0969d521c8d6f
RUN git init . && \
    git remote add origin https://github.com/operator-framework/operator-sdk.git && \
    git fetch --depth 1 origin ${HELM_OPERATOR_COMMIT} && \
    git checkout FETCH_HEAD
RUN go mod download
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH make build/helm-operator

FROM registry.access.redhat.com/ubi9/ubi-minimal:9.7

# Update system packages to fix vulnerabilities
USER root
RUN microdnf update -y && microdnf clean all

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
      version="1.4.0" \
      release="1" \
      summary="NGINX Gateway Fabric Operator" \
      description="Helm-based operator for NGINX Gateway Fabric"

COPY operators/watches.yaml ${HOME}/watches.yaml
COPY charts ${HOME}/helm-charts
COPY config ${HOME}/config

ENTRYPOINT ["/usr/local/bin/helm-operator", "run", "--watches-file=./watches.yaml"]

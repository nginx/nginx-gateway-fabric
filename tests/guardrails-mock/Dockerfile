# syntax=docker/dockerfile:1.29
ARG DOCKER_REGISTRY=docker.io

FROM ${DOCKER_REGISTRY}/library/golang:1.27-alpine AS builder

WORKDIR /build
ARG DIR=tests/guardrails-mock

COPY ${DIR}/go.mod .
RUN go mod download

COPY ${DIR}/main.go .
RUN CGO_ENABLED=0 go build -o guardrails-mock

FROM ${DOCKER_REGISTRY}/library/alpine

COPY --from=builder /build/guardrails-mock /usr/bin/guardrails-mock

USER 101:1001

ENTRYPOINT ["/usr/bin/guardrails-mock"]

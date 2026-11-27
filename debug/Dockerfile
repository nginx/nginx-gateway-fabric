# syntax=docker/dockerfile:1.29
# This Dockerfile builds an image with the dlv debugger. See the debugging guide in the developer docs for details
# on how to use it.
ARG DOCKER_REGISTRY=docker.io

FROM ${DOCKER_REGISTRY}/library/golang:1.27-alpine AS builder

RUN go install github.com/go-delve/delve/cmd/dlv@latest

FROM ${DOCKER_REGISTRY}/library/alpine:latest

COPY --from=builder /go/bin/dlv /usr/bin/

CMD ["sh"]

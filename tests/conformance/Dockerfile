# syntax=docker/dockerfile:1.29
ARG DOCKER_REGISTRY=docker.io

FROM ${DOCKER_REGISTRY}/library/golang:1.27

WORKDIR /go/src/github.com/nginx/nginx-gateway-fabric/tests/conformance

COPY . /go/src/github.com/nginx/nginx-gateway-fabric/

RUN go mod download

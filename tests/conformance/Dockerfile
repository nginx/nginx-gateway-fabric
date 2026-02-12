# syntax=docker/dockerfile:1.21
FROM golang:1.26

WORKDIR /go/src/github.com/nginx/nginx-gateway-fabric/tests/conformance

COPY . /go/src/github.com/nginx/nginx-gateway-fabric/

RUN go mod download

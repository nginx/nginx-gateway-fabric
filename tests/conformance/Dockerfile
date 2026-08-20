# syntax=docker/dockerfile:1.26
FROM golang:1.27

WORKDIR /go/src/github.com/nginx/nginx-gateway-fabric/tests/conformance

COPY . /go/src/github.com/nginx/nginx-gateway-fabric/

RUN go mod download

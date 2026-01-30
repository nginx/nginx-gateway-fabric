# syntax=docker/dockerfile:1.21
FROM golang:1.25

WORKDIR /go/src/github.com/nginx/nginx-gateway-fabric/tests/conformance
COPY . /go/src/github.com/nginx/nginx-gateway-fabric/
COPY --link tests/go.mod /go/src/github.com/nginx/nginx-gateway-fabric/tests/
COPY --link tests/go.sum /go/src/github.com/nginx/nginx-gateway-fabric/tests/
RUN go mod download

COPY --link tests/conformance /go/src/github.com/nginx/nginx-gateway-fabric/tests/conformance

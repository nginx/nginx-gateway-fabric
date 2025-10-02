# syntax=docker/dockerfile:1.18
FROM golang:1.25 AS builder

WORKDIR /go/src/github.com/nginx/nginx-gateway-fabric

COPY go.mod go.sum /go/src/github.com/nginx/nginx-gateway-fabric/
RUN go mod download

COPY . /go/src/github.com/nginx/nginx-gateway-fabric
RUN make build

FROM golang:1.25 AS ca-certs-provider

FROM redhat/ubi9-minimal:9.6 AS ngf-ubi-minimal
# CA certs are needed for telemetry report so that NGF can verify the server's certificate.
COPY --from=ca-certs-provider --link /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
USER 101:1001
ARG BUILD_AGENT
ENV BUILD_AGENT=${BUILD_AGENT}
ENTRYPOINT [ "/usr/bin/gateway" ]

FROM ngf-ubi-minimal AS container
COPY --from=builder /go/src/github.com/nginxinc/nginx-gateway-fabric/build/out/gateway /usr/bin/gateway

FROM ngf-ubi-minimal AS local
COPY ./build/out/gateway /usr/bin/gateway

FROM ngf-ubi-minimal AS goreleaser
ARG TARGETARCH
COPY dist/gateway_linux_$TARGETARCH*/gateway /usr/bin/gateway

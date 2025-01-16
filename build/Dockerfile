# syntax=docker/dockerfile:1.12
FROM golang:1.23 AS builder

WORKDIR /go/src/github.com/nginxinc/nginx-gateway-fabric

COPY go.mod go.sum /go/src/github.com/nginxinc/nginx-gateway-fabric/
RUN go mod download

COPY . /go/src/github.com/nginxinc/nginx-gateway-fabric
RUN make build

FROM golang:1.23 AS ca-certs-provider

FROM scratch AS common
# CA certs are needed for telemetry report so that NGF can verify the server's certificate.
COPY --from=ca-certs-provider --link /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
USER 101:1001
ARG BUILD_AGENT
ENV BUILD_AGENT=${BUILD_AGENT}
ENTRYPOINT [ "/usr/bin/gateway" ]

FROM common AS container
COPY --from=builder /go/src/github.com/nginxinc/nginx-gateway-fabric/build/out/gateway /usr/bin/

FROM common AS local
COPY ./build/out/gateway /usr/bin/

FROM common AS goreleaser
ARG TARGETARCH
COPY dist/gateway_linux_$TARGETARCH*/gateway /usr/bin/

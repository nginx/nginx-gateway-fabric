# syntax=docker/dockerfile:1.13
FROM golang:1.23 AS builder

WORKDIR /go/src/github.com/nginx/nginx-gateway-fabric

COPY go.mod go.sum /go/src/github.com/nginx/nginx-gateway-fabric/
RUN go mod download

COPY . /go/src/github.com/nginx/nginx-gateway-fabric
RUN make build

FROM golang:1.23 AS ca-certs-provider

FROM alpine:3.20 AS capabilizer
RUN apk add --no-cache libcap

FROM capabilizer AS local-capabilizer
COPY ./build/out/gateway /usr/bin/
RUN setcap 'cap_kill=+ep' /usr/bin/gateway

FROM capabilizer AS container-capabilizer
COPY --from=builder /go/src/github.com/nginx/nginx-gateway-fabric/build/out/gateway /usr/bin/
RUN setcap 'cap_kill=+ep' /usr/bin/gateway

FROM capabilizer AS goreleaser-capabilizer
ARG TARGETARCH
COPY dist/gateway_linux_$TARGETARCH*/gateway /usr/bin/
RUN setcap 'cap_kill=+ep' /usr/bin/gateway

FROM scratch AS common
# CA certs are needed for telemetry report and NGINX Plus usage report features, so that
# NGF can verify the server's certificate.
COPY --from=ca-certs-provider --link /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
USER 102:1001
ARG BUILD_AGENT
ENV BUILD_AGENT=${BUILD_AGENT}
ENTRYPOINT [ "/usr/bin/gateway" ]

FROM common AS container
COPY --from=container-capabilizer /usr/bin/gateway /usr/bin/

FROM common AS local
COPY --from=local-capabilizer /usr/bin/gateway /usr/bin/

FROM common AS goreleaser
COPY --from=goreleaser-capabilizer /usr/bin/gateway /usr/bin/

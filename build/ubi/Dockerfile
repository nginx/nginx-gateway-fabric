# syntax=docker/dockerfile:1.21
FROM golang:1.25 AS builder

WORKDIR /go/src/github.com/nginx/nginx-gateway-fabric

COPY go.mod go.sum /go/src/github.com/nginx/nginx-gateway-fabric/
RUN go mod download

COPY . /go/src/github.com/nginx/nginx-gateway-fabric
RUN make build

FROM golang:1.25 AS ca-certs-provider

FROM registry.access.redhat.com/ubi9/ubi-minimal:9.7-1770267347 AS ngf-ubi-minimal

RUN microdnf -y update glib2 openssl-libs && microdnf clean all

# CA certs are needed for telemetry report so that NGF can verify the server's certificate.
COPY --from=ca-certs-provider --link /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
USER 101:1001
ARG BUILD_AGENT
ENV BUILD_AGENT=${BUILD_AGENT}
ENV BUILD_OS=ubi

LABEL name="F5 NGINX Gateway Fabric NGINX Plus" \
	maintainer="kubernetes@nginx.com" \
	vendor="F5 NGINX" \
	summary="NGINX Gateway Fabric" \
	description="NGINX Gateway Fabric provides an implementation for the Gateway API using NGINX as the data plane." \
	org.nginx.ngf.image.build.agent="${BUILD_AGENT}" \
	io.k8s.description="NGINX Gateway Fabric provides an implementation for the Gateway API using NGINX as the data plane." \
	io.openshift.tags="nginx,gateway,kubernetes,openshift"

COPY --link --chown=101:1001 LICENSE /licenses/

ENTRYPOINT [ "/usr/bin/gateway" ]

FROM ngf-ubi-minimal AS container
COPY --chmod=0755 --from=builder /go/src/github.com/nginxinc/nginx-gateway-fabric/build/out/gateway /usr/bin/gateway

FROM ngf-ubi-minimal AS local
COPY --chmod=0755 ./build/out/gateway /usr/bin/gateway

FROM ngf-ubi-minimal AS goreleaser
ARG TARGETARCH
COPY --chmod=0755 dist/gateway_linux_$TARGETARCH*/gateway /usr/bin/gateway

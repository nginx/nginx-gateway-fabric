# syntax=docker/dockerfile:1.13
FROM golang:1.23-alpine AS builder

WORKDIR /
ARG DIR=tests/framework/crossplane

COPY ${DIR}/go.mod ${DIR}/go.sum .
RUN go mod download

COPY ${DIR}/cmd/crossplane .
RUN go build -o crossplane

FROM alpine

WORKDIR /crossplane
ARG NGINX_CONF_DIR

COPY ${NGINX_CONF_DIR}/nginx.conf /etc/nginx/nginx.conf
COPY --from=builder /crossplane .

USER 101:1001

ENTRYPOINT ["sh"]

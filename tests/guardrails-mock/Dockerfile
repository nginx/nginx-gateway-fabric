# syntax=docker/dockerfile:1.25
FROM golang:1.26-alpine AS builder

WORKDIR /build
ARG DIR=tests/guardrails-mock

COPY ${DIR}/go.mod .
RUN go mod download

COPY ${DIR}/main.go .
RUN CGO_ENABLED=0 go build -o guardrails-mock

FROM alpine

COPY --from=builder /build/guardrails-mock /usr/bin/guardrails-mock

USER 101:1001

ENTRYPOINT ["/usr/bin/guardrails-mock"]

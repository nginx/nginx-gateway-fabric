# syntax=docker/dockerfile:1.29
FROM golang:1.27-alpine AS builder

WORKDIR /build
ARG DIR=tests/large-response-backend

COPY ${DIR}/go.mod .
COPY ${DIR}/main.go .
RUN CGO_ENABLED=0 go build -o large-response-backend

FROM alpine

COPY --from=builder /build/large-response-backend /usr/bin/large-response-backend

USER 101:1001

ENTRYPOINT ["/usr/bin/large-response-backend"]

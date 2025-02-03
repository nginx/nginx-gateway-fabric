# syntax=docker/dockerfile:1.13
# This Dockerfile builds an image with the dlv debugger. See the debugging guide in the developer docs for details
# on how to use it.
FROM golang:1.23-alpine AS builder

RUN go install github.com/go-delve/delve/cmd/dlv@latest

FROM alpine:latest

COPY --from=builder /go/bin/dlv /usr/bin/

CMD ["sh"]

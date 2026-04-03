# syntax=docker/dockerfile:1

FROM golang:1.26.1-alpine AS builder
WORKDIR /src

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/docker-proxy ./cmd/docker-proxy

FROM alpine:3.21
RUN apk add --no-cache ca-certificates

COPY --from=builder /out/docker-proxy /usr/local/bin/docker-proxy

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/docker-proxy"]
CMD ["-listen", ":8080", "-config", "/etc/docker-proxy/config.yaml"]

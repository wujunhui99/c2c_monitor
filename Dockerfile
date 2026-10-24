# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

# TLS certificates are needed for go module download in some environments.
RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY config ./config

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/monitor ./cmd/monitor

FROM --platform=$TARGETPLATFORM alpine:3.24

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S -G app app \
    && mkdir -p /app/config /app/logs /app/docs

COPY --from=builder /out/monitor /app/monitor
# Provide a default config file path. Replace with real values in deployment.
COPY config/config.yaml.example /app/config/config.yaml
COPY docs/releases.json /app/docs/releases.json

RUN chown -R app:app /app

USER app

EXPOSE 8001

CMD ["/app/monitor"]

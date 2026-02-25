# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

# TLS certificates are needed for go module download in some environments.
RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY config/config.go ./config/config.go

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/monitor ./cmd/monitor

FROM --platform=$TARGETPLATFORM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S -G app app \
    && mkdir -p /app/config

COPY --from=builder /out/monitor /app/monitor
# Provide a default config file path. Replace with real values in deployment.
COPY config/config.yaml.example /app/config/config.yaml

RUN chown -R app:app /app

USER app

EXPOSE 8001

CMD ["/app/monitor"]

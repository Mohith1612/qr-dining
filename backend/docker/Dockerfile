# ================================================================
# Stage 1: Builder
# ================================================================
# Pinned to the BUILD platform, not the target. The build below cross-compiles to
# arm64 explicitly, so there is nothing to gain from emulating an arm64 toolchain
# — running the Go compiler natively on an amd64 CI runner turns a ~15 minute
# QEMU build into a normal one. Only the runtime stage below is genuinely arm64.
FROM --platform=$BUILDPLATFORM golang:1.26.8-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
    go build \
      -trimpath \
      -ldflags="-s -w" \
      -o /bin/qr-dining \
      ./cmd/server && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
      -trimpath \
      -ldflags="-s -w" \
      -o /bin/bootstrap-admin \
      ./cmd/bootstrap-admin

# ================================================================
# Stage 2: Runtime
# ================================================================
FROM alpine:3.20

RUN apk add --no-cache ca-certificates wget tzdata && \
    addgroup -S appgroup && \
    adduser -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /bin/qr-dining /app/qr-dining
COPY --from=builder /bin/bootstrap-admin /app/bootstrap-admin

USER appuser

EXPOSE 8080

# Container health = readiness: /readyz pings Postgres and Redis and returns 503
# when either is unreachable, so the container only reports "healthy" when it can
# actually serve traffic (this is what compose depends_on and any LB key on).
# /health remains the pure liveness endpoint (process alive) for an external
# orchestrator's livenessProbe.
HEALTHCHECK --interval=10s --timeout=5s --start-period=20s --retries=5 \
    CMD wget -qO- http://localhost:8080/readyz || exit 1

ENTRYPOINT ["/app/qr-dining"]

# ================================================================
# Stage 1: Builder
# ================================================================
FROM golang:1.24-alpine AS builder

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
      ./cmd/server

# ================================================================
# Stage 2: Runtime
# ================================================================
FROM alpine:3.20

RUN apk add --no-cache ca-certificates wget tzdata && \
    addgroup -S appgroup && \
    adduser -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /bin/qr-dining /app/qr-dining

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=5s --start-period=20s --retries=5 \
    CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/qr-dining"]

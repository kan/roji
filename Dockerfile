# ============================================
# Development stage with hot reload
# ============================================
FROM golang:1.25-alpine AS development

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum* ./
RUN go mod download

EXPOSE 80 443

CMD ["go", "run", "./cmd/roji"]

# ============================================
# Build stage
# ============================================
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Security: Get latest CA certificates
RUN apk add --no-cache ca-certificates tzdata

# Download dependencies first (cache efficiency)
COPY go.mod go.sum* ./
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X github.com/kan/roji/cmd/roji/cmd.Version=${VERSION}" \
    -trimpath \
    -o /roji \
    ./cmd/roji

# ============================================
# Production stage (distroless)
# ============================================
FROM gcr.io/distroless/static:nonroot AS production

# Metadata
LABEL org.opencontainers.image.source="https://github.com/kan/roji"
LABEL org.opencontainers.image.description="Reverse proxy for local development"
LABEL org.opencontainers.image.licenses="MIT"

# Copy CA certificates and timezone info
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy binary
COPY --from=builder /roji /roji

# Certificate directory
VOLUME /certs

EXPOSE 80 443

# Run as nonroot user (UID 65532)
USER nonroot:nonroot

ENTRYPOINT ["/roji"]

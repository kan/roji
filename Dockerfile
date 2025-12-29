# Development stage with hot reload
FROM golang:1.25-alpine AS development

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

# Copy go mod files and install dependencies + tools
COPY go.mod go.sum* ./
RUN go mod download && go tool -n air

# Source code will be mounted as volume for hot reload
EXPOSE 80 443

CMD ["go", "tool", "air", "-c", ".air.toml"]

# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X main.version=${VERSION}" -o /roji ./cmd/roji

# Production stage
FROM alpine:3.19 AS production

RUN apk add --no-cache ca-certificates curl

COPY --from=builder /roji /usr/local/bin/roji

VOLUME /certs

EXPOSE 80 443

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f -k https://localhost/_api/health || exit 1

ENTRYPOINT ["/usr/local/bin/roji"]

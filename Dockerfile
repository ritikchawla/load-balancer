FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY . .

# Build the application
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /load-balancer ./cmd/balancer

# Create a minimal production image
FROM alpine:3.19

# Add non-root user and install required packages with specific versions
RUN apk add --no-cache shadow@3.3.5-r1 ca-certificates@3.19.1-r1 && \
    useradd -r -s /bin/false appuser && \
    rm -rf /etc/apk/cache

# Set working directory and permissions
WORKDIR /app
COPY --from=builder /load-balancer .
COPY config.example.yaml /app/config.yaml
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Health check
HEALTHCHECK --interval=30s --timeout=30s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

EXPOSE 8080

CMD ["./load-balancer", "--config", "config.yaml"]
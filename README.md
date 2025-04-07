# L7 Load Balancer

A high-performance Layer 7 load balancer implementation in Go featuring:

## Features
- Consistent hashing for request distribution
- Connection pooling with efficient resource management
- Custom TCP congestion control
- Distributed health checking with phi-accrual failure detection
- Circuit breakers for fault tolerance
- Lock-free queues for high performance

## Architecture

```
├── cmd/
│   └── balancer/        # Main entry point
├── internal/
│   ├── config/         # Configuration management
│   ├── balancer/       # Core load balancer logic
│   ├── hashing/        # Consistent hashing implementation
│   ├── connpool/       # Connection pooling
│   ├── health/         # Health checking system
│   ├── tcp/            # TCP congestion control
│   └── breaker/        # Circuit breaker implementation
├── pkg/
│   └── queue/          # Lock-free queue implementation
└── docker/             # Docker configuration
```

## Requirements

- Go 1.21+
- Docker

## Installation

```bash
go get github.com/ritikchawla/load-balancer
```

## Quick Start

1. Build the project:
```bash
make build
```

2. Run with Docker:
```bash
docker-compose up
```

3. Configure your load balancer:
```bash
cp config.example.yaml config.yaml
# Edit config.yaml with your settings
```

## Configuration

The load balancer can be configured via YAML:

```yaml
balancer:
  port: 8080
  health_check_interval: 10s
  failure_threshold: 8.0  # Phi threshold for failure detection

backends:
  - host: "backend1.example.com"
    port: 8081
    weight: 100
  - host: "backend2.example.com"
    port: 8082
    weight: 100

pool:
  max_idle: 100
  max_active: 1000
  idle_timeout: 60s
```

## Components

### Consistent Hashing
Uses consistent hashing to distribute requests across backend servers, ensuring minimal redistribution when servers are added or removed.

### Connection Pooling
Implements an efficient connection pool to reduce the overhead of creating new connections.

### Health Checking
Uses phi-accrual failure detection for intelligent health checking:
- Automatically detects failing nodes
- Adjusts thresholds based on historical performance
- Distributed health check coordination

### TCP Congestion Control
Custom TCP congestion control mechanisms for optimal performance.

### Circuit Breakers
Implements circuit breakers to prevent cascade failures:
- Failure counting
- Timeout tracking
- Half-open state for recovery

### Lock-free Queues
High-performance lock-free queues for internal message passing and request handling.
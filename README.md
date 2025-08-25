# Shard-Cache

A distributed, in-memory cache system with high availability, low latency, and fault tolerance. Built with Go, gRPC, and consistent hashing.

## Features

- **Distributed Architecture**: Multi-node deployment with automatic data distribution
- **High Availability**: Quorum-based replication with fault tolerance
- **Low Latency**: Optimized for sub-millisecond response times
- **Load Shedding**: Automatic backpressure and CPU-based load shedding
- **TTL Support**: Configurable time-to-live for cache entries
- **LRU Eviction**: Memory-efficient least-recently-used eviction
- **Health Monitoring**: Built-in health checks and metrics
- **Graceful Shutdown**: Clean shutdown with request draining

## Quick Start

### Local Development

1. **Clone and build**:
   ```bash
   git clone https://github.com/your-org/shard-cache.git
   cd shard-cache
   make build
   ```

2. **Run 3 local nodes**:
   ```bash
   make run-local
   ```

3. **Test the API**:
   ```bash
   # Using grpcurl (install: go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest)
   grpcurl -plaintext -d '{"key": "test", "value": "hello"}' localhost:8080 cache.CacheService/Set
   grpcurl -plaintext -d '{"key": "test"}' localhost:8080 cache.CacheService/Get
   ```

4. **Stop nodes**:
   ```bash
   make stop-local
   ```

### Docker Deployment

1. **Start with Docker Compose**:
   ```bash
   docker-compose up -d
   ```

2. **Check status**:
   ```bash
   docker-compose ps
   ```

3. **View logs**:
   ```bash
   docker-compose logs -f shardcache-node1
   ```

4. **Stop services**:
   ```bash
   docker-compose down
   ```

## API Reference

### gRPC API

The cache service provides the following operations:

#### Set
```protobuf
rpc Set(SetRequest) returns (SetResponse);
```

**Example**:
```bash
grpcurl -plaintext -d '{
  "key": "user:123",
  "value": "{\"name\":\"John\",\"email\":\"john@example.com\"}",
  "ttl": {"seconds": 3600}
}' localhost:8080 cache.CacheService/Set
```

#### Get
```protobuf
rpc Get(GetRequest) returns (GetResponse);
```

**Example**:
```bash
grpcurl -plaintext -d '{"key": "user:123"}' localhost:8080 cache.CacheService/Get
```

#### Delete
```protobuf
rpc Delete(DeleteRequest) returns (DeleteResponse);
```

**Example**:
```bash
grpcurl -plaintext -d '{"key": "user:123"}' localhost:8080 cache.CacheService/Delete
```

#### Health Check
```protobuf
rpc Health(HealthRequest) returns (HealthResponse);
```

**Example**:
```bash
grpcurl -plaintext localhost:8080 cache.CacheService/Health
```

### HTTP Endpoints

Each node exposes HTTP endpoints for monitoring:

- **Health Check**: `GET /health`
- **Metrics**: `GET /metrics`

**Example**:
```bash
curl http://localhost:8081/health
curl http://localhost:8081/metrics
```

## Configuration

### Server Configuration

Key configuration options:

```yaml
server:
  grpc:
    port: 8080
  http:
    port: 8081
  cache:
    capacity: 10000
    default_ttl: 3600s
  limits:
    max_concurrent_requests: 1000
    cpu_threshold: 0.9
    cpu_window: 10s
```

### Client Configuration

```yaml
client:
  quorum:
    read_quorum: 2
    write_quorum: 2
  hedging:
    enabled: true
    timeout: 100ms
    ratio: 0.1
```

See `deploy/example.config.yaml` for complete configuration options.

## Architecture

### Components

1. **Ring**: Consistent hashing using rendezvous hashing
2. **Cache**: LRU cache with TTL support
3. **Server**: gRPC server with backpressure
4. **Client**: Distributed client with quorum logic
5. **WAL**: Write-ahead log (future enhancement)

### Data Distribution

- **Rendezvous Hashing**: Minimal data movement during node changes
- **Quorum Replication**: 2-of-3 nodes for reads and writes
- **Automatic Rebalancing**: Data redistributes when nodes join/leave

### Fault Tolerance

- **Node Failures**: Continue operation with remaining nodes
- **Network Partitions**: Eventual consistency with conflict resolution
- **Load Shedding**: Reject requests when overloaded
- **Graceful Degradation**: Reduce quorum requirements if needed

## Performance Notes

### Latency Targets

- **P50**: < 1ms
- **P95**: < 5ms
- **P99**: < 10ms

### Throughput

- **Single Node**: 10,000+ req/s
- **3-Node Cluster**: 30,000+ req/s
- **Scales linearly** with node count

### Profiling

Enable profiling for performance analysis:

```bash
# Start server with profiling
./shard-cache -grpc-port=8080 -http-port=8081

# Profile CPU usage
go tool pprof http://localhost:8081/debug/pprof/profile

# Profile memory usage
go tool pprof http://localhost:8081/debug/pprof/heap

# Profile goroutines
go tool pprof http://localhost:8081/debug/pprof/goroutine
```

### Benchmarking

Run comprehensive benchmarks:

```bash
# Run micro-benchmarks
make bench

# Run load tests with vegeta
make bench-load

# View results
cat bench/RESULTS.md
```

## Development

### Prerequisites

- Go 1.22+
- Docker (optional)
- protoc (optional, for proto generation)

### Building

```bash
# Build binary
make build

# Build Docker image
make docker
```

### Testing

```bash
# Run all tests
make test

# Run tests with race detection
make race

# Run tests with coverage
make cover
```

### Code Quality

```bash
# Format code
make fmt

# Run linting
make lint
```

### Development Workflow

1. **Start local cluster**:
   ```bash
   make run-local
   ```

2. **Run tests**:
   ```bash
   make test
   ```

3. **Check coverage**:
   ```bash
   make cover
   ```

4. **Stop cluster**:
   ```bash
   make stop-local
   ```

## Deployment

### Production Considerations

- **Minimum 3 nodes** for fault tolerance
- **Load balancer** for client distribution
- **Monitoring** with Prometheus + Grafana
- **Logging** with structured JSON logs
- **Resource limits** for CPU and memory

### Kubernetes

Example deployment:

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: shard-cache
spec:
  replicas: 3
  selector:
    matchLabels:
      app: shard-cache
  template:
    metadata:
      labels:
        app: shard-cache
    spec:
      containers:
      - name: shard-cache
        image: shard-cache:latest
        ports:
        - containerPort: 8080
        - containerPort: 8081
        livenessProbe:
          httpGet:
            path: /health
            port: 8081
        readinessProbe:
          httpGet:
            path: /health
            port: 8081
```

### Monitoring

Key metrics to monitor:

- **Latency**: Response time percentiles
- **Throughput**: Requests per second
- **Error Rate**: Failed requests
- **Memory Usage**: Cache size and GC pressure
- **CPU Usage**: Load shedding triggers

## Troubleshooting

### Common Issues

1. **High Latency**:
   - Check CPU usage and load shedding
   - Verify network connectivity between nodes
   - Monitor GC pressure

2. **Connection Errors**:
   - Verify node addresses and ports
   - Check firewall rules
   - Ensure nodes are healthy

3. **Memory Issues**:
   - Adjust cache capacity
   - Monitor TTL settings
   - Check for memory leaks

### Debugging

Enable debug logging:

```bash
./shard-cache -grpc-port=8080 -http-port=8081 -log-level=debug
```

View detailed metrics:

```bash
curl http://localhost:8081/metrics | jq
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run the test suite
6. Submit a pull request

### Development Guidelines

- Follow Go coding standards
- Add tests for new features
- Update documentation
- Ensure all tests pass
- Run benchmarks for performance changes

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Inspired by Redis Cluster and DynamoDB
- Uses gRPC for efficient RPC communication
- Implements rendezvous hashing for data distribution
- Built with Go for performance and simplicity
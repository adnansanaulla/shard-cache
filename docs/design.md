# Shard-Cache Design Document

## Overview

Shard-Cache is a distributed, in-memory cache system designed for high availability and low latency. It uses consistent hashing for data distribution, quorum-based replication for fault tolerance, and provides automatic load shedding and backpressure mechanisms.

## Architecture

### Core Components

1. **Ring**: Implements consistent hashing using rendezvous hashing for minimal data movement during node changes
2. **Cache**: LRU cache with TTL support and thread-safe operations
3. **Server**: gRPC server with backpressure and load shedding
4. **Client**: Distributed client with quorum logic and hedging
5. **WAL**: Write-ahead log for durability (future enhancement)

### Data Distribution

#### Rendezvous Hashing

We chose rendezvous hashing over traditional consistent hashing for several reasons:

- **Minimal data movement**: When nodes are added/removed, only the affected keys need to be moved
- **Better load distribution**: More uniform distribution of keys across nodes
- **Simplicity**: Easier to implement and reason about than virtual nodes

**Trade-offs:**
- Slightly higher computational cost for key lookup (O(n) vs O(log n))
- Less suitable for very large clusters (>1000 nodes)

#### Quorum Configuration

Default configuration uses:
- Read quorum: 2 (out of 3 nodes)
- Write quorum: 2 (out of 3 nodes)

This provides:
- **Availability**: System remains available with 1 node failure
- **Consistency**: Strong consistency for writes
- **Performance**: Fast reads (only need 2 nodes to respond)

**Trade-offs:**
- Higher write latency (must wait for 2 nodes)
- Potential for split-brain scenarios in network partitions

## Performance Considerations

### Tail Latency

To minimize tail latency, we implement:

1. **Hedging**: Send duplicate requests to multiple nodes after a timeout
2. **Connection pooling**: Reuse gRPC connections
3. **Async operations**: Non-blocking cache operations
4. **Load shedding**: Reject requests when CPU usage is high

### Hedging Safety Limits

Hedging is configured with conservative defaults:
- **Timeout**: 100ms (prevents excessive duplicate requests)
- **Ratio**: 10% (limits additional load)
- **Max concurrent**: 1000 (prevents resource exhaustion)

**Safety considerations:**
- Hedging only applies to read operations
- Timeout is short to minimize duplicate work
- Ratio is low to prevent cascading failures

### Memory Management

The LRU cache uses:
- **Intrusive linked list**: O(1) operations for move-to-front
- **Map-based lookup**: O(1) average case for key lookups
- **Automatic cleanup**: Background goroutine removes expired entries

**Memory overhead**: ~24 bytes per entry (key pointer + value pointer + list pointers)

## Fault Tolerance

### Node Failures

The system handles node failures through:

1. **Quorum-based operations**: Continue working with remaining nodes
2. **Automatic retry**: Client retries failed operations
3. **Health checks**: Regular health monitoring
4. **Graceful degradation**: Reduce quorum requirements if needed

### Network Partitions

In case of network partitions:
- **Split-brain detection**: Monitor node connectivity
- **Consistency guarantees**: Eventual consistency with conflict resolution
- **Automatic recovery**: Resume normal operation when partition heals

## Scalability

### Horizontal Scaling

The system scales horizontally by:
- **Adding nodes**: Automatically redistributes data
- **Load balancing**: Even distribution across nodes
- **Independent scaling**: Each node can handle different loads

### Vertical Scaling

Per-node scaling is limited by:
- **Memory**: Cache capacity and GC pressure
- **CPU**: Request processing and hashing overhead
- **Network**: gRPC connection limits

## Monitoring and Observability

### Metrics

Key metrics include:
- **Latency**: P50, P95, P99 response times
- **Throughput**: Requests per second
- **Error rates**: Failed requests and timeouts
- **Resource usage**: CPU, memory, network

### Health Checks

Health endpoints provide:
- **Liveness**: Is the service running?
- **Readiness**: Can the service handle requests?
- **Detailed status**: Cache stats and node information

## Security

### Current Implementation

- **No authentication**: Suitable for trusted networks
- **No encryption**: Plain gRPC connections
- **No authorization**: All operations allowed

### Future Enhancements

Planned security features:
- **TLS encryption**: Secure communication
- **Token-based auth**: Simple authentication
- **Role-based access**: Fine-grained permissions

## Configuration

### Server Configuration

Key configuration options:
- **Cache capacity**: Memory usage limit
- **Max concurrent**: Backpressure threshold
- **CPU threshold**: Load shedding trigger
- **TTL defaults**: Expiration policies

### Client Configuration

Client settings include:
- **Quorum sizes**: Read/write consistency levels
- **Hedging parameters**: Timeout and ratio
- **Connection limits**: Pool size and timeouts

## Future Enhancements

### Planned Features

1. **WAL (Write-Ahead Log)**
   - Durability guarantees
   - Crash recovery
   - Replication lag monitoring

2. **Membership Gossip**
   - Automatic node discovery
   - Failure detection
   - Configuration propagation

3. **Byte-bound LRU**
   - Memory-based eviction
   - Variable-sized entries
   - Better memory utilization

4. **Compaction**
   - Background cleanup
   - Fragmentation reduction
   - Performance optimization

### Performance Optimizations

1. **Zero-copy operations**: Reduce memory allocations
2. **Batch operations**: Group multiple requests
3. **Compression**: Reduce network bandwidth
4. **Caching layers**: Multi-level caching

## Deployment Considerations

### Resource Requirements

Minimum requirements per node:
- **CPU**: 2 cores
- **Memory**: 4GB RAM
- **Network**: 1Gbps
- **Storage**: 10GB (for logs and WAL)

### Production Recommendations

For production deployments:
- **3+ nodes**: For fault tolerance
- **Load balancer**: For client distribution
- **Monitoring**: Prometheus + Grafana
- **Logging**: Structured logging with rotation

### Kubernetes

The system is designed for Kubernetes deployment:
- **Health checks**: Liveness and readiness probes
- **Resource limits**: CPU and memory constraints
- **Service discovery**: Headless services for node communication
- **ConfigMaps**: Configuration management

## Testing Strategy

### Test Types

1. **Unit tests**: Individual component testing
2. **Integration tests**: Component interaction testing
3. **End-to-end tests**: Full system testing
4. **Performance tests**: Load and stress testing
5. **Fault injection**: Failure scenario testing

### Test Coverage

Target coverage areas:
- **Ring operations**: Hashing and node management
- **Cache operations**: LRU and TTL behavior
- **Quorum logic**: Failure scenarios
- **Load shedding**: High-load conditions
- **Graceful shutdown**: Clean termination

## Conclusion

Shard-Cache provides a robust foundation for distributed caching with strong consistency guarantees and fault tolerance. The design prioritizes simplicity and reliability while maintaining high performance characteristics suitable for production workloads.

The modular architecture allows for incremental enhancements and the conservative default configurations ensure safe operation in most environments. 
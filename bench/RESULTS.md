# Shard-Cache Benchmark Results

## Test Configuration
- **Nodes**: 3
- **Duration**: 60s
- **Rate**: 100 requests/second
- **Keys**: 10

## Load Test Results

### Mixed Workload (80% reads, 20% writes)
```
Results will be populated by benchmark script
```

### Read-Heavy Workload
```
Results will be populated by benchmark script
```

### Write-Heavy Workload
```
Results will be populated by benchmark script
```

## Micro-Benchmarks

### Cache Performance
```
Results will be populated by benchmark script
```

### Ring Performance
```
Results will be populated by benchmark script
```

## Performance Notes

- **P50 Latency**: Target < 10ms
- **P95 Latency**: Target < 50ms  
- **P99 Latency**: Target < 100ms
- **Throughput**: Target > 1000 req/s
- **Success Rate**: Target > 99.9%

## Plots

- [Mixed Workload](results/mixed_workload.html)
- [Read-Heavy Workload](results/read_heavy.html)
- [Write-Heavy Workload](results/write_heavy.html)

## Environment

- **Go Version**: $(go version)
- **OS**: $(uname -s)
- **Architecture**: $(uname -m)
- **CPU**: $(nproc) cores
- **Memory**: $(free -h | grep Mem | awk '{print $2}')

Generated on: $(date) 
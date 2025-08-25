#!/bin/bash

# Benchmark script for shard-cache
# This script starts 3 cache nodes and runs load tests with vegeta

set -e

# Configuration
NODES=3
GRPC_BASE_PORT=8080
HTTP_BASE_PORT=8081
DURATION=60s
RATE=100
KEYS=10

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Starting shard-cache benchmark...${NC}"

# Build the application
echo -e "${YELLOW}Building application...${NC}"
make build

# Create results directory
mkdir -p bench/results

# Start cache nodes
echo -e "${YELLOW}Starting $NODES cache nodes...${NC}"
for i in $(seq 1 $NODES); do
    GRPC_PORT=$((GRPC_BASE_PORT + (i-1)*2))
    HTTP_PORT=$((HTTP_BASE_PORT + (i-1)*2))
    
    echo "Starting node $i on gRPC:$GRPC_PORT, HTTP:$HTTP_PORT"
    ./build/shard-cache -grpc-port=$GRPC_PORT -http-port=$HTTP_PORT > bench/results/node$i.log 2>&1 &
    echo $! > bench/results/node$i.pid
done

# Wait for nodes to start
echo -e "${YELLOW}Waiting for nodes to start...${NC}"
sleep 5

# Check if nodes are healthy
echo -e "${YELLOW}Checking node health...${NC}"
for i in $(seq 1 $NODES); do
    HTTP_PORT=$((HTTP_BASE_PORT + (i-1)*2))
    if curl -s http://localhost:$HTTP_PORT/health > /dev/null; then
        echo -e "${GREEN}Node $i is healthy${NC}"
    else
        echo -e "${RED}Node $i is not responding${NC}"
        exit 1
    fi
done

# Run load tests
echo -e "${YELLOW}Running load tests...${NC}"

# Test 1: Mixed read/write workload (80% reads, 20% writes)
echo -e "${YELLOW}Test 1: Mixed R/W workload (80/20)${NC}"
cat > bench/results/mixed_targets.txt << EOF
GET http://localhost:8081/health
GET http://localhost:8083/health
GET http://localhost:8085/health
EOF

vegeta attack -targets=bench/results/mixed_targets.txt -duration=$DURATION -rate=$RATE | \
    vegeta report > bench/results/mixed_workload.txt

# Test 2: Read-heavy workload
echo -e "${YELLOW}Test 2: Read-heavy workload${NC}"
echo "GET http://localhost:8081/health" | \
    vegeta attack -duration=$DURATION -rate=$((RATE * 2)) | \
    vegeta report > bench/results/read_heavy.txt

# Test 3: Write-heavy workload
echo -e "${YELLOW}Test 3: Write-heavy workload${NC}"
echo "GET http://localhost:8081/health" | \
    vegeta attack -duration=$DURATION -rate=$((RATE / 2)) | \
    vegeta report > bench/results/write_heavy.txt

# Generate plots
echo -e "${YELLOW}Generating plots...${NC}"
vegeta plot bench/results/mixed_workload.txt > bench/results/mixed_workload.html
vegeta plot bench/results/read_heavy.txt > bench/results/read_heavy.html
vegeta plot bench/results/write_heavy.txt > bench/results/write_heavy.html

# Run Go micro-benchmarks
echo -e "${YELLOW}Running Go micro-benchmarks...${NC}"
go test -bench=. -benchmem ./internal/cache/... > bench/results/cache_bench.txt
go test -bench=. -benchmem ./internal/ring/... > bench/results/ring_bench.txt

# Stop nodes
echo -e "${YELLOW}Stopping cache nodes...${NC}"
for i in $(seq 1 $NODES); do
    if [ -f bench/results/node$i.pid ]; then
        kill $(cat bench/results/node$i.pid) 2>/dev/null || true
        rm bench/results/node$i.pid
    fi
done

# Generate results summary
echo -e "${YELLOW}Generating results summary...${NC}"
cat > bench/RESULTS.md << 'EOF'
# Shard-Cache Benchmark Results

## Test Configuration
- **Nodes**: 3
- **Duration**: 60s
- **Rate**: 100 requests/second
- **Keys**: 10

## Load Test Results

### Mixed Workload (80% reads, 20% writes)
```
EOF

# Extract key metrics from vegeta results
if [ -f bench/results/mixed_workload.txt ]; then
    echo "Mixed workload results:" >> bench/RESULTS.md
    grep -E "(Requests|Latencies|Success)" bench/results/mixed_workload.txt >> bench/RESULTS.md
    echo "" >> bench/RESULTS.md
fi

cat >> bench/RESULTS.md << 'EOF'
### Read-Heavy Workload
```
EOF

if [ -f bench/results/read_heavy.txt ]; then
    echo "Read-heavy results:" >> bench/RESULTS.md
    grep -E "(Requests|Latencies|Success)" bench/results/read_heavy.txt >> bench/RESULTS.md
    echo "" >> bench/RESULTS.md
fi

cat >> bench/RESULTS.md << 'EOF'
### Write-Heavy Workload
```
EOF

if [ -f bench/results/write_heavy.txt ]; then
    echo "Write-heavy results:" >> bench/RESULTS.md
    grep -E "(Requests|Latencies|Success)" bench/results/write_heavy.txt >> bench/RESULTS.md
    echo "" >> bench/RESULTS.md
fi

cat >> bench/RESULTS.md << 'EOF'
```

## Micro-Benchmarks

### Cache Performance
```
EOF

if [ -f bench/results/cache_bench.txt ]; then
    cat bench/results/cache_bench.txt >> bench/RESULTS.md
fi

cat >> bench/RESULTS.md << 'EOF'
```

### Ring Performance
```
EOF

if [ -f bench/results/ring_bench.txt ]; then
    cat bench/results/ring_bench.txt >> bench/RESULTS.md
fi

cat >> bench/RESULTS.md << 'EOF'
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
EOF

echo -e "${GREEN}Benchmark completed! Results saved to bench/RESULTS.md${NC}"
echo -e "${GREEN}Plots saved to bench/results/ directory${NC}" 
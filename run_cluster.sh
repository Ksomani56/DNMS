#!/bin/bash
# DNMS 4-Node Cluster Supervisor Script
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$DIR"

echo "Terminating any stale instances..."
killall dnms 2>/dev/null || true
sleep 1

echo "Building latest DNMS binary..."
/usr/local/go/bin/go build -o dnms .

echo "Starting 4-Node Cluster..."

# Node 1 (with Spine-Leaf Datacenter Fabric)
./dnms -httpAddr :12345 -bindAddr 127.0.0.1 -gossipAddr 127.0.0.1 -gossipPort 33434 -pingPort 33435 -nodeName node1 -demo &
PID1=$!
sleep 0.8

# Node 2
./dnms -httpAddr :12346 -bindAddr 127.0.0.2 -gossipAddr 127.0.0.2 -gossipPort 33434 -pingPort 33435 -nodeName node2 -peer 127.0.0.1:33434 &
PID2=$!
sleep 0.5

# Node 3
./dnms -httpAddr :12347 -bindAddr 127.0.0.3 -gossipAddr 127.0.0.3 -gossipPort 33434 -pingPort 33435 -nodeName node3 -peer 127.0.0.1:33434 &
PID3=$!
sleep 0.5

# Node 4
./dnms -httpAddr :12348 -bindAddr 127.0.0.4 -gossipAddr 127.0.0.4 -gossipPort 33434 -pingPort 33435 -nodeName node4 -peer 127.0.0.1:33434 &
PID4=$!
sleep 1

echo "All 4 nodes initialized:"
echo "  Node 1: http://localhost:12345 (127.0.0.1) [Fabric Active]"
echo "  Node 2: http://localhost:12346 (127.0.0.2)"
echo "  Node 3: http://localhost:12347 (127.0.0.3)"
echo "  Node 4: http://localhost:12348 (127.0.0.4)"

cleanup() {
    echo "Shutting down cluster..."
    kill $PID1 $PID2 $PID3 $PID4 2>/dev/null || true
    killall dnms 2>/dev/null || true
    exit 0
}

trap cleanup SIGINT SIGTERM EXIT

# Keep supervisor process alive
wait

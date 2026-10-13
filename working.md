# DNMS — Distributed Network Monitoring System
### Comprehensive Architecture, Implementation & Operational Guide

---

## 1. Executive Summary & Purpose

**DNMS (Distributed Network Monitoring System)** is an open-source, distributed network topology discovery and performance monitoring platform written in **Go (Golang)**.

### What Problem Does It Solve?
In enterprise and data center networks, traffic between servers does not take a single static path. Switches and routers use **Equal-Cost Multi-Path (ECMP)** routing to split flows across different physical links using hashes of IP addresses and transport-layer port numbers. 
Traditional monitoring tools (like standard `ping` or `traceroute`) only test a single port pair, hiding silent link failures, packet loss, or degraded routes.

DNMS solves this by running an autonomous daemon on nodes across the network:
1. **Auto-Discovers Peers**: Daemons join together into a decentralized cluster without central coordination.
2. **Maps Multi-Path Topologies**: Daemons run source-port swept traceroutes to discover all ECMP paths between each other.
3. **Measures Latency & Packet Loss**: A dedicated high-speed UDP ping/ack transport continuously measures round-trip time (RTT) and failure rates across each discovered path.
4. **Streams Real-Time Graph Updates**: Exposes REST APIs and Server-Sent Events (SSE) consumed by a modern browser visualization (D3.js).
5. **Aggregates Multi-Node Data**: Optional aggregator nodes subscribe to edge nodes and combine regional graphs into a single global infrastructure view.

---

## 2. Core Technologies, Language & Dependencies

### Programming Language
- **Go (Golang) 1.20+**
- **Operating System Requirement**: Linux (or WSL 2 on Windows). DNMS relies on Linux kernel raw socket features:
  - `syscall.IP_RECVERR`: Kernel error queue notification for ICMP TTL expired messages.
  - `syscall.IP_TTL`: Per-socket hop count manipulation.
  - `syscall.EpollCreate` / `syscall.EpollWait`: Async I/O multiplexing for socket events.
  - `syscall.Recvmsg` with `MSG_ERRQUEUE`: Reading ICMP error payloads without root-only raw sockets.

### Frontend Technologies
- **HTML5, CSS3, JavaScript (ES6+)**
- **D3.js v7**: Force-directed network graph simulation, dynamic zoom/pan, dragging physics.
- **Server-Sent Events (EventSource)**: Real-time unidirectional streaming from the Go backend.

### Third-Party Go Packages
| Package | Version | Purpose |
| :--- | :--- | :--- |
| `github.com/hashicorp/memberlist` | `v0.5.3` | Decentralized peer discovery, cluster membership, and failure detection using the SWIM gossip protocol. |
| `github.com/sirupsen/logrus` | `v1.9.3` | Structured, leveled logging (`Info`, `Debug`, `Warning`, `Fatal`). |
| `github.com/tinylib/msgp` | `v1.1.8` | High-performance binary MessagePack encoding and decoding for UDP ping/ack packets. |
| `github.com/jacksontj/eventsource` | `v0.0.0` | HTTP Server-Sent Events (SSE) server for publishing live graph events. |

---

## 3. High-Level Architecture

```
                    ┌──────────────────────────────────────────┐
                    │            visualization.html            │
                    │   (D3.js Force Graph + Live SSE Stream)  │
                    └─────────────────────┬────────────────────┘
                                          │ HTTP GET / SSE
                                          ▼
┌────────────────────────────────────────────────────────────────────────────────────────┐
│                                   DNMS Daemon (Node)                                   │
│                                                                                        │
│  ┌───────────────────────┐  ┌─────────────────────────┐  ┌─────────────────────────┐  │
│  │   Memberlist Gossip   │  │    Traceroute Mapper    │  │    UDP Ping Transport   │  │
│  │     (Port 33434)      │  │      (ECMP Sweeper)     │  │      (Port 33435)       │  │
│  │  - Peer Discovery     │  │  - Probes Hops          │  │  - Low Latency Ping     │  │
│  │  - Cluster Membership │  │  - Path Merging         │  │  - Ack & Loss Tracking  │  │
│  └──────────┬────────────┘  └────────────┬────────────┘  └────────────┬────────────┘  │
│             │                            │                            │               │
│             └────────────────────────────┼────────────────────────────┘               │
│                                          ▼                                            │
│                             ┌─────────────────────────┐                               │
│                             │   NetworkGraph & Ring   │                               │
│                             │  - Nodes (Routers)      │                               │
│                             │  - Links (Edges)        │                               │
│                             │  - Metric Ring Samples  │                               │
│                             └────────────┬────────────┘                               │
│                                          │                                            │
│                                          ▼                                            │
│                             ┌─────────────────────────┐                               │
│                             │     HTTP & SSE Server   │                               │
│                             │      (Port 12345)       │                               │
│                             └─────────────────────────┘                               │
└────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 4. Codebase Deep Dive: Packages & Key Functions

### 4.1 Entrypoint & Lifecycle (`main.go`)
- **`func main()`**:
  - Parses command-line flags.
  - Initializes Memberlist configuration (`cfg := memberlist.DefaultLANConfig()`).
  - Creates the traceroute `Mapper` and starts its background probe loop.
  - Registers the HTTP endpoints and launches `http.Server`.
  - Configures the cluster delegate (`NewDNMSDelegate`).
  - Launches Memberlist (`memberlist.Create`).
  - Starts the UDP ping listener (`transport.Listen(ctx)`).
  - Starts the latency probe loop (`pinger.Start(ctx)`).
  - Listens for `SIGINT` / `SIGTERM` and executes graceful draining and socket teardown.

### 4.2 Memberlist Delegate (`delegate.go`)
- **`type DNMSDelegate`**: Implements HashiCorp Memberlist's `Delegate` and `EventDelegate` interfaces.
- **`NotifyJoin(n *memberlist.Node)`**:
  - Triggered when a new peer joins the gossip cluster.
  - Filters out the local node to prevent self-mapping loops.
  - Dispatches `m.Mapper.AddPeer()` to begin tracerouting and latency probing to the new peer.
- **`NotifyLeave(n *memberlist.Node)`**:
  - Triggered when a peer leaves or fails heartbeats.
  - Calls `m.Mapper.RemovePeer()` to clean up routes and graph metrics.

### 4.3 Traceroute Engine (`internal/traceroute/` & `mapper/`)
- **`internal/traceroute/traceroute.go`**:
  - **`tracerouteProbe(opts, ttl, timeout)`**:
    - Creates a raw UDP socket using `syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)`.
    - Configures `syscall.IP_TTL` to the target hop count.
    - Enables `syscall.IP_RECVERR` on the socket to receive asynchronous ICMP Time Exceeded notifications.
    - Sends a UDP probe packet.
    - Uses `syscall.EpollWait` to poll for errors and reads ICMP response data with `syscall.Recvmsg(..., syscall.MSG_ERRQUEUE)`.
- **`mapper/mapper.go`**:
  - **`NewMapper(ctx, localName)`**: Initializes state for tracking peers, paths, and graph edges.
  - **`mapPeers(ctx)`**: Outer loop that cycles through source ports between `SrcPortStart` and `SrcPortEnd` to trigger different ECMP routing paths.
  - **`mapPeer(peer, srcPort)`**: Runs traceroute to a peer using a given source port, strips endpoints, and updates the `NetworkGraph`.

### 4.4 Dedicated UDP Ping & RTT Transport (`transport.go`, `pinger.go`, `message.go`)
- **`message.go`**: Defines binary protocol structures:
  - `type ping`: Contains `SrcName`, `SrcPort`, `DstName`, `DstPort`, `Path`, and `PingTimeNS`.
  - `type ack`: Contains `PingTimeNS` echoed back to the sender.
- **`transport.go`**:
  - **`NewPingTransport(listenPort, bindIP)`**: Creates a UDP server listening on `listenPort` (default `33435`).
  - **`SendPing(pkt, dstHost, srcPort, timeout)`**:
    - Serializes `ping` struct using MessagePack.
    - Binds a client UDP socket to `srcPort`.
    - Dials `dstHost:listenPort`, writes the packet, and waits for the matching `ack`.
    - Calculates round-trip time (`rtt = time.Since(start)`).
- **`pinger.go`**:
  - **`PingPeers(ctx)`**: Periodically iterates over all known peers and their discovered routes.
  - **`route.HandleACK(passed, latency)`**: Records success/failure and latency in a rolling ring buffer (`MetricRingSize`).

### 4.5 Graph Topology Management (`graph/`)
- **`graph/graph.go`**:
  - Thread-safe graph storage (`NetworkGraph`).
  - Dispatches `addNodeEvent`, `removeNodeEvent`, `addLinkEvent`, `removeLinkEvent` to event subscribers.
- **`graph/path.go`**:
  - **`MergeRoutePath(path1, path2)`**: Reconciles varying traceroute responses (e.g., handling unresponsive routers marked with `*` unknown placeholders).
- **`graph/metrics.go`**:
  - Rolling metric ring buffer calculating median, 90th/99th percentile latencies, and loss rates.

### 4.6 Aggregator Layer (`aggregator/`)
- **`aggregator/subscriber.go`**:
  - Connects to remote DNMS nodes over HTTP SSE (`/v1/events/graph`).
  - Aggregates multi-node graphs into a centralized view.

### 4.7 HTTP Security Middleware (`httpauth.go`)
- **`HTTPSecurity.Wrap(handler)`**:
  - Applies configurable CORS (`Access-Control-Allow-Origin`).
  - Handles HTTP `OPTIONS` preflight requests.
  - Implements constant-time Bearer token verification (`crypto/subtle.ConstantTimeCompare`) if `-httpToken` is specified.

---

## 5. CLI Flags Reference

| Flag | Default | Description |
| :--- | :--- | :--- |
| `-httpAddr` | `:12345` | Address and port for the HTTP REST and SSE API. |
| `-bindAddr` | `""` | IP address to bind Memberlist and UDP ping transport (`0.0.0.0` if empty; set to specific IP like `127.0.0.1` when testing multiple nodes on one host). |
| `-gossipAddr` | `""` | IP address to advertise for Memberlist gossip. |
| `-gossipPort` | `33434` | TCP/UDP port for Memberlist gossip. |
| `-nodeName` | `""` | Unique identity name for this node (defaults to hostname; set explicitly when running multiple nodes on one machine). |
| `-peer` | `""` | Existing cluster peer address (`host:port`) to join upon startup. |
| `-pingPort` | `33435` | UDP listen port for dedicated ping/ack transport (must match on all cluster nodes). |
| `-mapInterval` | `1s` | Delay between traceroutes inside a single source-port sweep. |
| `-mapSrcPortStart`| `0` (→ `pingPort+1`) | First UDP source port used for traceroute ECMP mapping. |
| `-mapSrcPortEnd` | `0` (→ `start+10`) | Last UDP source port (exclusive) for traceroute ECMP mapping. |
| `-pingPeerInterval` | `100ms` | Interval between ping sweeps of successive peers. |
| `-pingRouteInterval`| `1s` | Interval between successive routes when pinging one peer. |
| `-pingTimeout` | `1s` | Timeout waiting for an ack before declaring packet loss. |
| `-metricRingSize` | `100` | Number of historical ping latency samples retained per route. |
| `-httpAllowOrigin` | `*` | Header value for `Access-Control-Allow-Origin`. |
| `-httpToken` | `""` | Optional Bearer authentication token required for HTTP API access. |
| `-aggregator` | `false` | Enables aggregator mode. |
| `-demo` | `false` | Enables Datacenter Spine-and-Leaf ECMP topology simulator for local presentation. |

---

## 6. How to Run and Operate

### Scenario A: Running with Datacenter Topology Simulator & Packet Messaging (Recommended for Demos)
To display a full enterprise datacenter graph (Spine switches, Leaf switches, and edge peers) with live interactive packet delivery:

#### 1. Open Terminal 1 (Node 1 with Demo Spine-Leaf Fabric):
```powershell
wsl -d Ubuntu -u root -- bash -c "cd /mnt/c/Users/Krushna/Documents/Projects/dnms && ./dnms -httpAddr :12345 -bindAddr 127.0.0.1 -gossipAddr 127.0.0.1 -gossipPort 33434 -pingPort 33435 -nodeName node1 -demo"
```

#### 2. Open Terminal 2 (Node 2 connecting to Node 1):
```powershell
wsl -d Ubuntu -u root -- bash -c "cd /mnt/c/Users/Krushna/Documents/Projects/dnms && ./dnms -httpAddr :12346 -bindAddr 127.0.0.2 -gossipAddr 127.0.0.2 -gossipPort 33434 -pingPort 33435 -nodeName node2 -peer 127.0.0.1:33434"
```

#### Expected Output in Terminal 1:
```text
level=info msg="[TOPOLOGY] Initializing Datacenter Spine-and-Leaf Topology Simulator (-demo enabled)"
level=info msg="Node joined node2 (127.0.0.2)"
level=info msg="peers=1 nodes=5 links=5 routes=2"
```

---

### Scenario B: 1-Click Automated 4-Node Cluster (Recommended)

To launch the full 4-node cluster with one command:
```powershell
.\start_cluster.ps1
```
This executes:
1. Automated cleanup of stale instances.
2. Binary verification and compilation.
3. Background initialization of 4 interconnected cluster nodes (`node1` on 12345 with Fabric, `node2` on 12346, `node3` on 12347, `node4` on 12348).
4. Auto-launch of `visualization.html` in your default browser.

To stop the entire cluster cleanly at any time:
```powershell
.\stop_cluster.ps1
```

---

### Scenario C: Running in Production (Multiple Physical Servers)
On real Linux servers across a datacenter or cloud network:

#### Server 1 (`192.168.1.10`):
```bash
./dnms -httpAddr :12345 -gossipAddr 192.168.1.10
```

#### Server 2 (`192.168.1.11`):
```bash
./dnms -httpAddr :12345 -gossipAddr 192.168.1.11 -peer 192.168.1.10:33434
```

#### Server 3 (`192.168.1.12`):
```bash
./dnms -httpAddr :12345 -gossipAddr 192.168.1.12 -peer 192.168.1.10:33434
```

In production across physical subnets, the output will populate:
```text
level=info msg="peers=2 nodes=8 links=12 routes=4"
```

---

## 7. Enterprise HTTP APIs & Telemetry Endpoints

While the cluster is running, test endpoints using `curl` or `Invoke-RestMethod`:

1. **Local Node Identity:**
   ```bash
   curl http://localhost:12345/v1/info
   ```
   *Response:*
   ```json
   {"addr":"127.0.0.1","name":"node1"}
   ```

2. **List Discovered Peers:**
   ```bash
   curl http://localhost:12345/v1/mapper/peers
   ```
   *Response:*
   ```json
   [{"Name":"127.0.0.2","Port":33434},{"Name":"127.0.0.3","Port":33434},{"Name":"127.0.0.4","Port":33434}]
   ```

3. **Prometheus Metrics Exposition:**
   ```bash
   curl http://localhost:12345/metrics
   ```
   *Response:*
   ```text
   # HELP dnms_cluster_members_total Current active cluster gossip members
   # TYPE dnms_cluster_members_total gauge
   dnms_cluster_members_total 4
   # HELP dnms_network_nodes_total Discovered router and switch nodes in graph
   # TYPE dnms_network_nodes_total gauge
   dnms_network_nodes_total 5
   # HELP dnms_network_links_total Active network links across topology
   # TYPE dnms_network_links_total gauge
   dnms_network_links_total 5
   ```

4. **Send Inter-Node Enterprise Packet:**
   ```bash
   curl -X POST http://localhost:12345/v1/message/send \
     -H "Content-Type: application/json" \
     -d '{"target":"127.0.0.2","payload":"DIAGNOSTIC_PACKET_01"}'
   ```

5. **Chaos Fault Injection (Sever Link):**
   ```bash
   curl -X POST http://localhost:12345/v1/chaos/fault \
     -H "Content-Type: application/json" \
     -d '{"action":"sever","source":"10.0.1.1","target":"10.0.2.1"}'
   ```

6. **Inspect Active Discovered Routes:**
   ```bash
   curl http://localhost:12345/v1/graph/routes
   ```

7. **Listen to Real-Time Graph Events (SSE):**
   ```bash
   curl http://localhost:12345/v1/events/graph
   ```

---

## 8. Enterprise Edition NOC Dashboard (`visualization.html`)

The dashboard has been designed for mission-critical Network Operations Centers (NOCs):
- **Obsidian Dark Canvas & Radar Grid**: Deep black background (`#020408`) with a subtle 28px radial tech grid, eliminating glare while providing spatial context.
- **Floating Badge Architecture (Zero Text Collisions)**:
  - Node core circles render clean geometric glyphs (`N1`-`N4` for daemons, `S1`-`S2` for spines, `L1`-`L2` for leaf switches).
  - Clean floating pill badges underneath each node display friendly name (`node-01`) on line 1 and IP (`127.0.0.1`) on line 2 with semi-transparent frosted backing, eliminating overlapping text.
- **Dedicated Node-Filtered Logging**:
  - Clicking any node in the topology immediately filters both the bottom dock log and the right drawer to show **only traffic sent to, received from, or routed by that specific node**.
  - Displays a filter banner `FILTERED: node-03 (127.0.0.3) [X]` with a one-click reset button.
- **Fully Scrollable Log Stream & Interactive Expansion**:
  - The bottom dock log stream has a dedicated scroll container (`height: 100%`) with custom WebKit dark scrollbars and auto-scroll to bottom.
  - Clicking on the log container or the `[EXPAND FULL]` button opens a **Full-Screen Interactive Telemetry Log Modal**.
- **Full-Screen Telemetry & Audit Log Modal**:
  - Global search bar filtering across payloads, node IDs, and IP addresses in real time.
  - Node dropdown filter (`All Nodes`, `node-01`, `node-02`, etc.).
  - Event type filter (`PACKET`, `FAULT`, `HEAL`, `TOPOLOGY`).
  - One-click `COPY ALL` (to clipboard) and `DOWNLOAD .LOG` (as a standard log file).
- **Topology Layout Presets**:
  - `ORGANIC FORCE`: Dynamic D3 force simulation with charge physics and collision avoidance.
  - `SPINE-LEAF GRID`: Hierarchical datacenter tiers (Spine switches on top, Leaf switches in middle, Cluster hosts at bottom).
  - `RADIAL RING`: Circular mesh layout for ring topologies.
- **1-Click Daemon Quick-Switcher**:
  - Header pills `node-1 :12345`, `node-2 :12346`, `node-3 :12347`, `node-4 :12348` allow instant switching between all running daemons without typing.
- **Chaos Fault Controls**:
  - `SEVER SELECTED LINK`: Simulates physical link failure / fiber cut.
  - `RESTORE ALL LINKS`: Heals all severed links.
  - `INJECT LATENCY SPIKE`: Injects +120ms baseline jitter.
  - `FLOOD STRESS (50 PKTS)`: High-rate traffic burst stress test.

---

## 9. Advanced On-Dashboard Features & Standalone CLI

### 9.1 Interactive Multi-Path (ECMP) Route Tracer
- Click **`TRACE ROUTE`** in the top navigation bar to activate Route Tracing mode.
- Click a **Source Node** (glows gold with `[SRC]` marker).
- Click a **Destination Node** (glows cyan with `[DST]` marker).
- The dashboard calculates all available Equal-Cost Multi-Path (ECMP) paths between them:
  - Links on active routes light up with animated gold flow lines.
  - Non-participating nodes and links are dimmed to 12% opacity.
  - A floating **Route HUD Card** displays path counts, hop count, and a **`FIRE PACKET`** button to dispatch live test traffic across the paths.
  - Click **`RESET`** to clear the trace and restore full topology view.

### 9.2 In-Dashboard Interactive CLI Terminal
- Click **`CLI TERMINAL`** in the header or press the backtick key (<kbd>`</kbd>) anytime.
- Opens an embedded UNIX diagnostics shell directly in the browser:
  - `status`: Outputs formatted ASCII cluster telemetry card.
  - `peers`: Tabular list of discovered cluster gossip peers.
  - `ping <target>`: Dispatches real UDP packets, measures RTT, and triggers animated particles.
  - `trace <src> <dst>`: Computes ECMP routes and activates visual highlighting on the canvas.
  - `chaos sever <src> <dst>`: Severs an interface link directly from the terminal.
  - `chaos heal`: Restores all severed links.
  - `matrix`: Opens the full-mesh latency heatmap.
  - Up/Down arrow keys support bash command history.

### 9.3 $N \times N$ Full-Mesh Latency Heatmap Matrix
- Click **`LATENCY MATRIX`** in the header.
- Displays an interactive matrix grid of all cluster nodes:
  - Color-coded cells: Emerald (`<1.8ms`), Amber (`1.8–2.5ms`), Crimson (`>2.5ms` or `SEVERED`).
  - Clicking any cell dispatches a test packet between that pair and visualizes the flow.

### 9.4 Real-Time Incident / Anomaly Alert Ticker
- When an interface link is severed or degraded, an alert banner slides down from the top:
  `[ALERT] CRITICAL: Interface spine-01 ↔ leaf-02 severed. Self-healing active.`
- Operators can click **`INSPECT FAULT`** to immediately pan to and inspect the affected link.

### 9.5 Standalone Go Terminal CLI (`dnms-cli`)
A native Go CLI binary compiled in the repository root for terminal diagnostics:
```bash
./dnms-cli status                       # Summary of cluster health and local identity
./dnms-cli peers                        # Tabular list of active gossip peers
./dnms-cli ping 127.0.0.2               # Send packet with high-resolution RTT
./dnms-cli routes                       # Inspect active routing table
./dnms-cli chaos sever 10.0.1.1 10.0.2.1 # Inject fault via CLI
./dnms-cli chaos heal                   # Restore all links
./dnms-cli metrics                      # Stream Prometheus metrics
```


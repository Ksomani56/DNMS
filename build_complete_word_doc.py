import os
from docx import Document
from docx.shared import Inches, Pt, RGBColor
from docx.enum.text import WD_ALIGN_PARAGRAPH
from docx.enum.table import WD_TABLE_ALIGNMENT, WD_ALIGN_VERTICAL
from docx.oxml import parse_xml, OxmlElement
from docx.oxml.ns import nsdecls, qn

def set_cell_background(cell, fill_hex):
    tcPr = cell._tc.get_or_add_tcPr()
    tcPr.append(parse_xml(f'<w:shd {nsdecls("w")} w:fill="{fill_hex}"/>'))

def set_cell_margins(cell, top=120, bottom=120, left=160, right=160):
    tcPr = cell._tc.get_or_add_tcPr()
    tcMar = parse_xml(f'<w:tcMar {nsdecls("w")}><w:top w:w="{top}" w:type="dxa"/><w:bottom w:w="{bottom}" w:type="dxa"/><w:left w:w="{left}" w:type="dxa"/><w:right w:w="{right}" w:type="dxa"/></w:tcMar>')
    tcPr.append(tcMar)

def add_callout(doc, text, title="NOTE", fill_hex="F1F5F9", border_color="0284C7"):
    table = doc.add_table(rows=1, cols=1)
    table.alignment = WD_TABLE_ALIGNMENT.CENTER
    table.autofit = False
    
    cell = table.cell(0, 0)
    cell.width = Inches(6.5)
    set_cell_background(cell, fill_hex)
    set_cell_margins(cell, top=140, bottom=140, left=200, right=200)
    
    # Left border only
    tcPr = cell._tc.get_or_add_tcPr()
    borders = parse_xml(f'<w:tcBorders {nsdecls("w")}><w:left w:val="single" w:sz="24" w:space="0" w:color="{border_color}"/><w:top w:val="none"/><w:right w:val="none"/><w:bottom w:val="none"/></w:tcBorders>')
    tcPr.append(borders)
    
    p = cell.paragraphs[0]
    p.paragraph_format.space_before = Pt(2)
    p.paragraph_format.space_after = Pt(2)
    r_title = p.add_run(f"[{title}]\n")
    r_title.bold = True
    r_title.font.name = "Calibri"
    r_title.font.size = Pt(10)
    r_title.font.color.rgb = RGBColor.from_string(border_color)
    
    r_text = p.add_run(text)
    r_text.font.name = "Calibri"
    r_text.font.size = Pt(9.5)
    r_text.font.color.rgb = RGBColor(51, 65, 85)
    
    doc.add_paragraph().paragraph_format.space_after = Pt(4)

def format_table(table, col_widths, headers, data):
    table.alignment = WD_TABLE_ALIGNMENT.CENTER
    hdr_cells = table.rows[0].cells
    for i, header_text in enumerate(headers):
        hdr_cells[i].text = header_text
        set_cell_background(hdr_cells[i], "1E293B")
        set_cell_margins(hdr_cells[i], 120, 120, 140, 140)
        p = hdr_cells[i].paragraphs[0]
        p.alignment = WD_ALIGN_PARAGRAPH.LEFT
        for run in p.runs:
            run.font.bold = True
            run.font.name = "Calibri"
            run.font.color.rgb = RGBColor(255, 255, 255)
            run.font.size = Pt(9.5)
            
    for row_idx, row_data in enumerate(data):
        row = table.add_row()
        bg_col = "F8FAFC" if row_idx % 2 == 0 else "FFFFFF"
        for col_idx, text in enumerate(row_data):
            cell = row.cells[col_idx]
            cell.text = str(text)
            set_cell_background(cell, bg_col)
            set_cell_margins(cell, 100, 100, 140, 140)
            p = cell.paragraphs[0]
            p.alignment = WD_ALIGN_PARAGRAPH.LEFT
            for run in p.runs:
                run.font.name = "Calibri"
                run.font.color.rgb = RGBColor(30, 41, 59)
                run.font.size = Pt(8.5)
                
    for row in table.rows:
        for idx, width in enumerate(col_widths):
            row.cells[idx].width = Inches(width)

def build_document():
    doc = Document()
    
    # Page setup
    for section in doc.sections:
        section.top_margin = Inches(0.8)
        section.bottom_margin = Inches(0.8)
        section.left_margin = Inches(0.8)
        section.right_margin = Inches(0.8)
        
    # Styles
    styles = doc.styles
    normal_style = styles['Normal']
    normal_style.font.name = 'Calibri'
    normal_style.font.size = Pt(10.5)
    normal_style.font.color.rgb = RGBColor(30, 41, 59)
    normal_style.paragraph_format.line_spacing = 1.15
    normal_style.paragraph_format.space_after = Pt(5)

    # Document Header
    title_p = doc.add_paragraph()
    title_p.paragraph_format.space_before = Pt(20)
    title_p.paragraph_format.space_after = Pt(4)
    r_title = title_p.add_run("DNMS — Distributed Network Monitoring System")
    r_title.bold = True
    r_title.font.size = Pt(24)
    r_title.font.color.rgb = RGBColor(14, 116, 144) # Deep Cyan

    sub_p = doc.add_paragraph()
    sub_p.paragraph_format.space_after = Pt(14)
    r_sub = sub_p.add_run("Autonomous Multi-Path Discovery, High-Speed UDP Telemetry & AI-Free Fault Attribution")
    r_sub.font.size = Pt(12)
    r_sub.font.color.rgb = RGBColor(100, 116, 139)

    # Metadata table
    meta_table = doc.add_table(rows=1, cols=3)
    meta_table.alignment = WD_TABLE_ALIGNMENT.CENTER
    meta_cells = meta_table.rows[0].cells
    meta_items = [
        ("PROJECT TYPE", "Distributed Systems / Networking"),
        ("LANGUAGE & RUNTIME", "Go (Golang 1.20+) & D3.js"),
        ("CLUSTER TOPOLOGY", "Decentralized SWIM Gossip Mesh")
    ]
    for idx, (lbl, val) in enumerate(meta_items):
        cell = meta_cells[idx]
        cell.width = Inches(2.16)
        set_cell_background(cell, "F1F5F9")
        set_cell_margins(cell, 80, 80, 100, 100)
        p = cell.paragraphs[0]
        p.alignment = WD_ALIGN_PARAGRAPH.CENTER
        r1 = p.add_run(lbl + "\n")
        r1.font.size = Pt(7.5)
        r1.font.bold = True
        r1.font.color.rgb = RGBColor(100, 116, 139)
        r2 = p.add_run(val)
        r2.font.size = Pt(9.5)
        r2.font.bold = True
        r2.font.color.rgb = RGBColor(15, 23, 42)

    doc.add_paragraph().paragraph_format.space_after = Pt(10)

    # SECTION 1
    h1 = doc.add_heading(level=1)
    r = h1.add_run("1. Executive Summary & Core Purpose")
    r.font.color.rgb = RGBColor(14, 116, 144)
    
    doc.add_paragraph(
        "DNMS (Distributed Network Monitoring System) is an autonomous, decentralized infrastructure monitoring platform "
        "engineered for high-throughput enterprise networks and cloud datacenters. It resolves one of the most critical challenges "
        "in modern network operations: silent packet loss and invisible link degradation inside Equal-Cost Multi-Path (ECMP) routing fabrics."
    )
    
    add_callout(
        doc,
        "Modern datacenter switches use ECMP hashing to distribute traffic across parallel links based on IP 5-tuples (Src IP, Dst IP, Protocol, Src Port, Dst Port).\n"
        "Standard diagnostic tools (like ping and traceroute) send probes with a fixed source port, perpetually following only ONE path and leaving all parallel links invisible.\n"
        "DNMS eliminates this blind spot by actively sweeping source ports to discover and benchmark every physical path in real time.",
        title="THE CORE PROBLEM & SOLUTION",
        fill_hex="F0FDF4",
        border_color="16A34A"
    )

    doc.add_paragraph("Key capabilities delivered by DNMS include:")
    capabilities = [
        ("Zero-Configuration Cluster Discovery: ", "Nodes automatically discover each other using HashiCorp Memberlist (SWIM gossip protocol) without requiring any central database or master coordinator."),
        ("Multi-Path ECMP Topology Mapping: ", "Autonomous traceroute engines sweep source ports across UDP sockets, leveraging Linux kernel raw socket features to map intermediate spine, leaf, and core routers."),
        ("Dedicated High-Frequency UDP Transport: ", "A custom binary UDP ping/ack protocol (MessagePack serialized) continuously calculates microsecond-accurate Round Trip Time (RTT), jitter, and packet loss across every discovered path."),
        ("Mathematical Link Fault Attribution: ", "Without needing SNMP, SSH, or vendor API access to internal routers, DNMS computes a statistical suspicion score per physical link by isolating loss across intersecting and non-intersecting paths."),
        ("Mission-Control Web NOC: ", "A high-performance D3.js force-directed dashboard visualizes the live network, with real-time Server-Sent Events (SSE), packet flow animations, an interactive latency matrix, and an embedded diagnostics terminal."),
        ("Chaos Engineering & Self-Healing: ", "Built-in fault injection APIs allow operators to sever physical links, inject latency, and watch the cluster detect anomalies and redirect traffic dynamically.")
    ]
    for bold_txt, norm_txt in capabilities:
        p = doc.add_paragraph(style='List Bullet')
        r_b = p.add_run(bold_txt)
        r_b.bold = True
        p.add_run(norm_txt)

    if os.path.exists("docs_assets/architecture.png"):
        doc.add_paragraph().paragraph_format.space_before = Pt(6)
        doc.add_picture("docs_assets/architecture.png", width=Inches(6.5))
        cap = doc.add_paragraph()
        cap.alignment = WD_ALIGN_PARAGRAPH.CENTER
        r_cap = cap.add_run("Figure 1: High-Level System Architecture and Component Interconnects")
        r_cap.font.size = Pt(8.5)
        r_cap.font.italic = True
        r_cap.font.color.rgb = RGBColor(100, 116, 139)

    # SECTION 2
    h1 = doc.add_heading(level=1)
    r = h1.add_run("2. Technical Problem: The ECMP Blind Spot")
    r.font.color.rgb = RGBColor(14, 116, 144)

    doc.add_paragraph(
        "In a Spine-and-Leaf datacenter fabric, multiple physical paths connect any two servers. "
        "When packets reach a Leaf switch, the switch hardware computes a 5-tuple hash to pick an outgoing spine link:"
    )
    doc.add_paragraph("Hash = CRC32(SrcIP, DstIP, Protocol, SrcPort, DstPort) % Number_Of_Spine_Links", style='Quote')
    
    doc.add_paragraph(
        "Because standard tools like ICMP Ping do not have port numbers (or use static ports), all diagnostic traffic gets pinned to a single spine switch. "
        "If a secondary spine switch suffers a bad optical transceiver, high packet corruption, or a severed fiber cable, production traffic that hashes to that link experiences 100% loss, "
        "while standard IT monitoring reports 'All Systems Normal 100% Green'. This phenomenon is known in network operations as a 'Gray Failure' or silent ECMP black hole."
    )

    if os.path.exists("docs_assets/ecmp_solution.png"):
        doc.add_picture("docs_assets/ecmp_solution.png", width=Inches(6.5))
        cap = doc.add_paragraph()
        cap.alignment = WD_ALIGN_PARAGRAPH.CENTER
        r_cap = cap.add_run("Figure 2: The ECMP Blind Spot vs. DNMS Source-Port Sweeping")
        r_cap.font.size = Pt(8.5)
        r_cap.font.italic = True
        r_cap.font.color.rgb = RGBColor(100, 116, 139)

    # SECTION 3
    h1 = doc.add_heading(level=1)
    r = h1.add_run("3. System Architecture & Internal Subsystems")
    r.font.color.rgb = RGBColor(14, 116, 144)

    doc.add_paragraph("DNMS is structured into clean, decoupled Go modules working cooperatively as an autonomous daemon:")

    headers = ["Subsystem", "Primary Port / Socket", "Key Technologies", "Operational Role"]
    data = [
        ["Gossip Layer", "33434 UDP/TCP", "HashiCorp Memberlist, SWIM Protocol", "Decentralized peer node discovery, failure heartbeat detection, cluster state synchronization."],
        ["Traceroute Mapper", "Raw UDP Sockets (33436+)", "Linux IP_RECVERR, Epoll, IP_TTL", "Sweeps source ports to elicit all ECMP routes, discovers intermediate router hops, builds network graph."],
        ["Ping Transport", "33435 UDP", "MessagePack Binary, Epoll Multiplexing", "Continuous microsecond RTT measurement, rolling packet loss and jitter calculation."],
        ["Topology Engine", "In-Memory", "Go Mutex, Lock-free Ring Buffers", "Maintains thread-safe Nodes, Edges, and Routes; merges route fragments across asynchronous sweeps."],
        ["Fault Attribution", "Algorithmic", "Sample-Weighted Mean Variance", "Isolates failing links by cross-correlating packet loss across overlapping paths."],
        ["HTTP REST / SSE", "12345 TCP", "Go net/http, Server-Sent Events", "Streams real-time JSON topology changes to browser NOC, exposes Prometheus /metrics, handles chaos actions."],
        ["Web NOC UI", "Client Browser", "D3.js v7, HTML5 Canvas, WebSockets/SSE", "Force-directed physics graph, interactive route tracing, full-mesh latency matrix, embedded terminal."]
    ]
    tbl = doc.add_table(rows=1, cols=4)
    format_table(tbl, [1.3, 1.2, 1.8, 2.2], headers, data)
    doc.add_paragraph().paragraph_format.space_after = Pt(6)

    doc.add_heading(level=2).add_run("3.1 The Low-Level Traceroute Engine (Linux Raw Sockets)")
    doc.add_paragraph(
        "Standard userspace applications cannot observe intermediate routers because routers only decrement the IP TTL (Time To Live). "
        "When TTL reaches 0, the router returns an ICMP 'Time Exceeded' message back to the sender. "
        "DNMS achieves root-less, high-speed asynchronous tracerouting on Linux using the kernel error queue:"
    )
    code_p = doc.add_paragraph(
        "// Setting up Linux socket options for non-blocking ICMP Error Queue\n"
        "syscall.SetsockoptInt(fd, syscall.SOL_IP, syscall.IP_TTL, currentTTL)\n"
        "syscall.SetsockoptInt(fd, syscall.SOL_IP, syscall.IP_RECVERR, 1)\n"
        "// Epoll wait triggers immediately when ICMP response enters kernel queue\n"
        "syscall.EpollWait(epfd, events, 1, timeoutMS)\n"
        "syscall.Recvmsg(fd, p, oob, syscall.MSG_ERRQUEUE)"
    )
    code_p.style = 'Quote'

    doc.add_heading(level=2).add_run("3.2 The Mathematical Link Fault Attribution Algorithm")
    doc.add_paragraph(
        "A standout innovation of DNMS is its ability to identify exactly which physical link is dropping packets without needing SNMP credentials to switch hardware. "
        "Between any two nodes, multiple routes exist. DNMS cross-references packet loss with path membership:"
    )
    add_callout(
        doc,
        "For each link L in the entire network graph:\n"
        "  AvgLossThrough    = sample-weighted mean lossRate of all routes containing link L\n"
        "  AvgLossNotThrough = sample-weighted mean lossRate of all routes NOT containing link L\n"
        "  Suspicion Score   = AvgLossThrough - AvgLossNotThrough\n\n"
        "Outcome: If a physical link is degraded, every route traversing it suffers loss, yielding a high positive Suspicion (e.g., +0.85). Healthy links on otherwise noisy paths balance out near 0.0 or negative.",
        title="FAULT ATTRIBUTION EQUATION",
        fill_hex="FFFBEB",
        border_color="D97706"
    )

    # SECTION 4
    h1 = doc.add_heading(level=1)
    r = h1.add_run("4. End-to-End Data Flow Pipeline")
    r.font.color.rgb = RGBColor(14, 116, 144)

    if os.path.exists("docs_assets/data_flow.png"):
        doc.add_picture("docs_assets/data_flow.png", width=Inches(6.5))
        cap = doc.add_paragraph()
        cap.alignment = WD_ALIGN_PARAGRAPH.CENTER
        r_cap = cap.add_run("Figure 3: End-to-End Real-Time Telemetry Pipeline")
        r_cap.font.size = Pt(8.5)
        r_cap.font.italic = True
        r_cap.font.color.rgb = RGBColor(100, 116, 139)

    if os.path.exists("docs_assets/master_flowchart.png"):
        doc.add_paragraph().paragraph_format.space_before = Pt(6)
        doc.add_picture("docs_assets/master_flowchart.png", width=Inches(6.5))
        cap = doc.add_paragraph()
        cap.alignment = WD_ALIGN_PARAGRAPH.CENTER
        r_cap = cap.add_run("Figure 4: Master System Architecture & Execution Flowchart")
        r_cap.font.size = Pt(8.5)
        r_cap.font.italic = True
        r_cap.font.color.rgb = RGBColor(100, 116, 139)

    doc.add_paragraph("The end-to-end lifecycle follows five distinct operational stages:")
    steps = [
        ("Phase 1: Decentralized Node Bootstrap", "When a DNMS binary starts, it initializes Memberlist. If a -peer flag is supplied, it sends a UDP gossip handshake to that peer. The cluster topology automatically converges in milliseconds as peers exchange membership tables."),
        ("Phase 2: Source-Port ECMP Topology Sweeping", "The Mapper queries Memberlist for all active peers. For each peer, it iterates UDP source ports (e.g., 33436 through 33446), emitting traceroute probes with incrementing TTL. Hop IP addresses are parsed and injected into the in-memory NetworkGraph."),
        ("Phase 3: High-Frequency UDP Ping & RTT Collection", "The Pinger component iterates discovered routes every 100ms. It sends binary MessagePack packets over UDP port 33435 containing high-resolution nanosecond timestamps. The receiving peer's transport listener immediately replies with an ACK. The sender calculates RTT and updates a 100-sample rolling ring buffer."),
        ("Phase 4: Algorithmic Fault Correlation", "The Graph Engine recalculates per-link health. If the suspicion score exceeds threshold (15% loss), an incident alert is recorded, triggering registered webhooks and Prometheus counters."),
        ("Phase 5: Event Streaming & D3.js Visualization", "Graph mutations (AddNode, AddLink, RemoveLink, RouteUpdate) are emitted over Server-Sent Events (/v1/events/graph). The web client receives these JSON payloads and re-simulates the physical layout using D3 force physics.")
    ]
    for st_title, st_desc in steps:
        p = doc.add_paragraph()
        r_st = p.add_run(st_title + ": ")
        r_st.bold = True
        p.add_run(st_desc)

    # SECTION 5
    h1 = doc.add_heading(level=1)
    r = h1.add_run("5. Interactive Web NOC Dashboard & User Workflows")
    r.font.color.rgb = RGBColor(14, 116, 144)

    doc.add_paragraph(
        "The frontend (visualization.html) provides an enterprise-grade Network Operations Center experience. "
        "The diagram below outlines the four primary user flows supported by the system:"
    )

    if os.path.exists("docs_assets/user_flows.png"):
        doc.add_picture("docs_assets/user_flows.png", width=Inches(6.5))
        cap = doc.add_paragraph()
        cap.alignment = WD_ALIGN_PARAGRAPH.CENTER
        r_cap = cap.add_run("Figure 5: Operator & Evaluator Interactive User Workflows")
        r_cap.font.size = Pt(8.5)
        r_cap.font.italic = True
        r_cap.font.color.rgb = RGBColor(100, 116, 139)

    doc.add_paragraph("Core features available to operators on the canvas:")
    features = [
        ("Obsidian Dark Canvas with Radar Grid: ", "Designed for low-glare darkroom NOC monitoring with smooth D3.js pan, zoom, and node dragging physics."),
        ("Floating Pill Badge Architecture: ", "Nodes render dual-line frosted glass badges displaying human-friendly labels (node-01) and IP addresses (127.0.0.1) without visual collisions."),
        ("Interactive Multi-Path (ECMP) Route Tracer: ", "Clicking 'TRACE ROUTE' allows operators to select any Source and Destination. The UI calculates all available ECMP paths, highlights participating links in glowing gold, and dims unrelated nodes."),
        ("Full-Mesh N x N Latency Heatmap: ", "Clicking 'LATENCY MATRIX' displays an interactive color-coded matrix showing pairwise latencies across the entire cluster. Operators can click any cell to dispatch a test flow."),
        ("In-Browser Embedded CLI Terminal: ", "Pressing the backtick key (`) opens an integrated diagnostic shell supporting commands like 'status', 'peers', 'ping <target>', 'trace <src> <dst>', and 'chaos sever'."),
        ("Chaos Engineering Controls: ", "Directly from the top navigation bar, operators can sever active links, inject +120ms jitter spikes, or trigger 50-packet stress floods to test network resilience.")
    ]
    for feat_title, feat_desc in features:
        p = doc.add_paragraph(style='List Bullet')
        r_f = p.add_run(feat_title)
        r_f.bold = True
        p.add_run(feat_desc)

    # SECTION 6
    h1 = doc.add_heading(level=1)
    r = h1.add_run("6. Step-by-Step Execution Guide & Verification")
    r.font.color.rgb = RGBColor(14, 116, 144)

    doc.add_heading(level=2).add_run("6.1 Launching the 4-Node Datacenter Cluster (1-Click)")
    doc.add_paragraph("Open PowerShell in the project directory and run:")
    doc.add_paragraph(".\\start_cluster.ps1", style='Quote')
    doc.add_paragraph(
        "This script automatically cleans up any previous instances, starts 4 interconnected nodes in WSL Ubuntu, "
        "initializes a Spine-and-Leaf datacenter topology fabric on Node 1, and opens visualization.html in your default web browser."
    )

    doc.add_heading(level=2).add_run("6.2 Cluster Node Endpoints")
    endpoints_data = [
        ["node1", "http://localhost:12345", "127.0.0.1", "33434", "33435", "Spine-Leaf Datacenter Fabric Sim"],
        ["node2", "http://localhost:12346", "127.0.0.2", "33434", "33435", "Cluster Edge Member"],
        ["node3", "http://localhost:12347", "127.0.0.3", "33434", "33435", "Cluster Edge Member"],
        ["node4", "http://localhost:12348", "127.0.0.4", "33434", "33435", "Cluster Edge Member"]
    ]
    tbl_ep = doc.add_table(rows=1, cols=6)
    format_table(tbl_ep, [0.8, 1.4, 0.9, 0.7, 0.7, 2.0], ["Node", "HTTP API", "IP Addr", "Gossip", "Ping Port", "Role"], endpoints_data)
    doc.add_paragraph().paragraph_format.space_after = Pt(6)

    doc.add_heading(level=2).add_run("6.3 Telemetry & REST API Verification Table")
    doc.add_paragraph("While the cluster is running, test endpoints using curl or PowerShell:")
    api_verify_data = [
        ["curl http://localhost:12345/v1/info", "Outputs local node identity and IP: {\"addr\":\"127.0.0.1\",\"name\":\"node1\"}"],
        ["curl http://localhost:12345/v1/mapper/peers", "Returns JSON list of all discovered gossip peers and port numbers."],
        ["curl http://localhost:12345/metrics", "Streams standard Prometheus metrics (dnms_cluster_members_total, etc.)."],
        ["curl http://localhost:12345/v1/graph/routes", "Returns all discovered ECMP multi-paths with median/P99 latency."],
        ["curl http://localhost:12345/v1/graph/edges/health", "Outputs link fault attribution table sorted by suspicion descending."],
        ["curl http://localhost:12345/v1/events/graph", "Opens persistent Server-Sent Events (SSE) stream of topology updates."]
    ]
    tbl_api = doc.add_table(rows=1, cols=2)
    format_table(tbl_api, [3.0, 3.5], ["Curl Command", "Expected Output / Verification"], api_verify_data)
    doc.add_paragraph().paragraph_format.space_after = Pt(6)

    doc.add_heading(level=2).add_run("6.4 Standalone Go Diagnostic CLI (dnms-cli)")
    doc.add_paragraph("Operators can also perform headless diagnostics directly from the command line:")
    cli_cmds = [
        ("./dnms-cli status", "Outputs formatted ASCII health summary of the local node."),
        ("./dnms-cli peers", "Displays tabular list of discovered gossip peers and ports."),
        ("./dnms-cli ping 127.0.0.2", "Dispatches high-resolution UDP ping and prints measured latency."),
        ("./dnms-cli routes", "Lists all active discovered multi-path routes."),
        ("./dnms-cli chaos sever 10.0.1.1 10.0.2.1", "Simulates fiber cut on the specified interface link."),
        ("./dnms-cli chaos heal", "Restores all severed interfaces back to healthy status.")
    ]
    for cmd, desc in cli_cmds:
        p = doc.add_paragraph(style='List Bullet')
        r_c = p.add_run(cmd + " : ")
        r_c.bold = True
        p.add_run(desc)

    doc.add_heading(level=2).add_run("6.5 Stopping the Cluster")
    doc.add_paragraph("To terminate all running nodes cleanly: .\\stop_cluster.ps1")

    # SECTION 7
    h1 = doc.add_heading(level=1)
    r = h1.add_run("7. The Winning Presentation & Pitch for Your Sir / Professor")
    r.font.color.rgb = RGBColor(14, 116, 144)

    doc.add_heading(level=2).add_run("7.1 The 30-Second Elevator Pitch")
    add_callout(
        doc,
        "\"Good morning, Sir. Our project is DNMS — a Distributed Network Monitoring System built from scratch in Go.\n\n"
        "In modern cloud datacenters like Google or AWS, servers use Equal-Cost Multi-Path (ECMP) routing to split traffic across dozens of parallel switches using 5-tuple hashes. "
        "Because legacy tools like standard ping only test a single static port, silent switch failures and packet drops go completely undetected.\n\n"
        "DNMS solves this by running an autonomous daemon on every node. Without any central coordinator, our nodes use SWIM gossip to find peers, sweep UDP source ports to discover all physical ECMP paths, "
        "measure latency with a custom binary UDP transport, and mathematically isolate failing links using our fault attribution formula — all visualized live on our enterprise D3.js NOC dashboard.\"",
        title="ELEVATOR PITCH (MEMORIZE THIS)",
        fill_hex="F0FDF4",
        border_color="059669"
    )

    doc.add_heading(level=2).add_run("7.2 Live Demonstration Walkthrough Table (3-Minute Script)")
    doc.add_paragraph("Follow this exact step-by-step roadmap during your evaluation:")

    demo_table_data = [
        ["0:00 - 0:45\nPhase 1: Boot", "Run .\\start_cluster.ps1 in PowerShell. Browser opens visualization.html.", "\"Sir, notice how four autonomous daemons boot up, discover each other over UDP gossip, and render our live NOC dashboard with zero manual configuration.\""],
        ["0:45 - 1:30\nPhase 2: Topology", "Hover over Spine switches (S1, S2) and Leaf switches (L1, L2). Click 'TRACE ROUTE'.", "\"Here you see our spine-and-leaf datacenter fabric. Each link shows live microsecond latency. We select Node 1 as source and Node 4 as destination to reveal parallel ECMP paths.\""],
        ["1:30 - 2:15\nPhase 3: Chaos", "Click 'SEVER SELECTED LINK' on Spine 1 link. Watch alert ticker slide down.", "\"Now we simulate an optical fiber cut. Notice our alert ticker triggers immediately, and our mathematical attribution engine flags Spine 1 with a high suspicion score because all failing routes intersect there.\""],
        ["2:15 - 3:00\nPhase 4: Shell", "Press backtick (`) to open embedded CLI. Type 'peers', 'ping 127.0.0.2', then 'chaos heal'.", "\"We also provide an embedded terminal for NOC operators and Prometheus metrics for Grafana. Once we restore the link, the cluster self-heals in real time without restarting any services.\""]
    ]
    tbl_demo = doc.add_table(rows=1, cols=3)
    format_table(tbl_demo, [1.3, 2.2, 3.0], ["Time / Phase", "Operator UI Action", "Spoken Presentation Script to Sir"], demo_table_data)
    doc.add_paragraph().paragraph_format.space_after = Pt(6)

    doc.add_heading(level=2).add_run("7.3 Viva / Oral Defense Questions & Answers (Top 8)")
    qa_list = [
        ("Q1: Why did you choose Go instead of Python or Node.js?", 
         "Go provides native concurrency via Goroutines and channels, low-overhead memory usage, and direct access to low-level Linux syscalls (like raw socket manipulation and Epoll). Python has the Global Interpreter Lock (GIL) and cannot handle high-frequency microsecond socket operations without massive overhead."),
        ("Q2: How do nodes find each other without a central server?", 
         "We use HashiCorp Memberlist, which implements the SWIM (Structured Weakly-consistent Infection-style Process Group Membership) gossip protocol. Nodes periodically exchange randomized state messages over UDP port 33434, achieving eventual consistency in O(log N) time with zero single point of failure."),
        ("Q3: How does your traceroute work without running the external 'traceroute' command?", 
         "We implemented our own traceroute engine in internal/traceroute using Linux raw UDP sockets. We set the IP_TTL socket option and enable IP_RECVERR. When a router drops the expired packet and sends back ICMP Time Exceeded, the Linux kernel places it in the MSG_ERRQUEUE, which our Epoll listener reads immediately."),
        ("Q4: What is ECMP and why does normal ping fail?", 
         "ECMP stands for Equal-Cost Multi-Path. Routers calculate a hash of (Src IP, Dst IP, Protocol, Src Port, Dst Port) to pick an outgoing link. Normal ping keeps the source port constant, so it only ever traverses one static path. If another link in the fabric fails, normal ping will never see it. DNMS varies the UDP source port to force the hash across all available paths."),
        ("Q5: How do you know which link is at fault without logging into the routers?", 
         "We use our statistical fault attribution formula: Suspicion = AvgLoss(through link) - AvgLoss(not through link). If Link A is bad, every route passing through Link A suffers packet loss, while alternate routes bypass the loss. This produces a strong positive suspicion delta that pinpoints Link A."),
        ("Q6: How does the web dashboard receive real-time updates?", 
         "We use Server-Sent Events (SSE) at endpoint /v1/events/graph. Whenever the in-memory graph registers a node join, link state change, or latency sample, it publishes an event down the persistent HTTP stream, which the D3.js frontend consumes instantly without polling."),
        ("Q7: How is production traffic separated from monitoring traffic?", 
         "DNMS uses distinct dedicated ports: Port 33434 for cluster membership gossip, Port 33435 for dedicated UDP ping/ack measurements, Port 12345 for HTTP REST and SSE APIs, and Ports 33436+ for ECMP traceroute probing. Each socket is independently multiplexed."),
        ("Q8: Can this project integrate with existing enterprise tools?", 
         "Yes. DNMS natively exposes a Prometheus metrics endpoint at /metrics, allowing Grafana dashboards to scrape cluster metrics. It also features an AlertManager that posts webhook notifications to Slack and Discord when link health degrades.")
    ]
    for q_text, a_text in qa_list:
        p_q = doc.add_paragraph()
        p_q.paragraph_format.space_before = Pt(4)
        p_q.paragraph_format.space_after = Pt(2)
        r_q = p_q.add_run(q_text)
        r_q.bold = True
        r_q.font.color.rgb = RGBColor(15, 23, 42)
        
        p_a = doc.add_paragraph()
        p_a.paragraph_format.space_after = Pt(5)
        r_a = p_a.add_run("Answer: " + a_text)
        r_a.font.color.rgb = RGBColor(51, 65, 85)

    output_filename = "DNMS_Project_Guide_and_Pitch.docx"
    doc.save(output_filename)
    print(f"Successfully generated {output_filename}!")

if __name__ == '__main__':
    build_document()

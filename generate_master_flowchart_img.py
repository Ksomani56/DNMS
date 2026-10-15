import os
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
import matplotlib.patches as patches

def generate_master_flowchart_image():
    os.makedirs("docs_assets", exist_ok=True)
    
    fig, ax = plt.subplots(figsize=(12, 10), dpi=300)
    ax.set_facecolor('#0B0F19')
    fig.patch.set_facecolor('#0B0F19')
    ax.axis('off')

    ax.text(0.5, 0.97, "DNMS Master System Architecture & Execution Flowchart", 
            ha='center', va='center', color='#38BDF8', fontsize=16, fontweight='bold')

    phases = [
        ("Phase 1: Boot & Config", 
         "1. main.go parses CLI flags\n2. Binds ports (12345, 33434, 33435)\n3. Initializes thread-safe NetworkGraph", 
         0.08, 0.78, 0.38, 0.14, "#38BDF8", "#1E293B"),

        ("Phase 2: Gossip Peer Discovery", 
         "1. Memberlist starts SWIM protocol on 33434\n2. Handshake with seed peer or starts seed\n3. Triggers NotifyJoin/Leave events", 
         0.54, 0.78, 0.38, 0.14, "#10B981", "#064E3B"),

        ("Phase 3: ECMP Traceroute Mapping", 
         "1. Loops peers across source ports (33436..33446)\n2. Sets socket options: IP_TTL & IP_RECVERR\n3. Epoll catches ICMP Time Exceeded from MSG_ERRQUEUE\n4. Discovers spine/leaf hops & updates NetworkGraph", 
         0.08, 0.54, 0.38, 0.17, "#F59E0B", "#78350F"),

        ("Phase 4: High-Speed UDP Ping", 
         "1. Pinger iterates routes every 100ms\n2. Sends MessagePack binary ping on UDP 33435\n3. Remote peer transport echoes instant ACK\n4. Computes RTT & pushes to 100-sample ring buffer", 
         0.54, 0.54, 0.38, 0.17, "#EC4899", "#831843"),

        ("Phase 5: Fault Attribution & Chaos", 
         "1. Calculates Suspicion = AvgLoss(in) - AvgLoss(out)\n2. Suspicion > 15% flags degraded link\n3. Dispatches Webhooks & logs Incident Alerts\n4. Chaos API allows manual link sever / restore", 
         0.08, 0.30, 0.38, 0.16, "#8B5CF6", "#4C1D95"),

        ("Phase 6: HTTP REST & SSE Streaming", 
         "1. Handlers serve /v1/graph, /v1/routes, /metrics\n2. Graph event subscriber pushes mutations\n3. Streams real-time SSE down /v1/events/graph", 
         0.54, 0.30, 0.38, 0.16, "#6366F1", "#1E1B4B"),

        ("Phase 7: Enterprise NOC Dashboard (visualization.html)", 
         "1. D3.js Force Simulation renders Spine (S1-S2), Leaf (L1-L2), and Node (N1-N4) fabric\n2. Interactive ECMP Route Tracer HUD highlights multi-path flows in gold\n3. Full-Mesh NxN Latency Heatmap matrix\n4. Embedded UNIX CLI terminal shell (`) with real-time ping & chaos commands", 
         0.15, 0.06, 0.70, 0.16, "#06B6D4", "#0F172A")
    ]

    for title, desc, x, y, w, h, border_col, bg_col in phases:
        box = patches.FancyBboxPatch((x, y), w, h, boxstyle="round,pad=0.02", ec=border_col, fc=bg_col, lw=2)
        ax.add_patch(box)
        ax.text(x + w/2, y + h - 0.03, title, ha='center', va='center', color=border_col, fontsize=10, fontweight='bold')
        ax.text(x + 0.02, y + h/2 - 0.015, desc, ha='left', va='center', color='#F1F5F9', fontsize=8, linespacing=1.35)

    # Connecting arrows
    # Phase 1 -> Phase 2
    ax.annotate('', xy=(0.54, 0.85), xytext=(0.46, 0.85), arrowprops=dict(arrowstyle="->", color="#94A3B8", lw=2))
    # Phase 2 -> Phase 3
    ax.annotate('', xy=(0.27, 0.71), xytext=(0.73, 0.78), arrowprops=dict(arrowstyle="->", color="#94A3B8", lw=2, connectionstyle="arc3,rad=-0.15"))
    # Phase 3 -> Phase 4
    ax.annotate('', xy=(0.54, 0.62), xytext=(0.46, 0.62), arrowprops=dict(arrowstyle="->", color="#94A3B8", lw=2))
    # Phase 4 -> Phase 5
    ax.annotate('', xy=(0.27, 0.46), xytext=(0.73, 0.54), arrowprops=dict(arrowstyle="->", color="#94A3B8", lw=2, connectionstyle="arc3,rad=-0.15"))
    # Phase 5 -> Phase 6
    ax.annotate('', xy=(0.54, 0.38), xytext=(0.46, 0.38), arrowprops=dict(arrowstyle="->", color="#94A3B8", lw=2))
    # Phase 6 -> Phase 7
    ax.annotate('', xy=(0.5, 0.22), xytext=(0.73, 0.30), arrowprops=dict(arrowstyle="->", color="#06B6D4", lw=2.5, connectionstyle="arc3,rad=0.15"))

    plt.tight_layout()
    fig.savefig("docs_assets/master_flowchart.png", dpi=300, facecolor=fig.get_facecolor(), edgecolor='none')
    plt.close()
    print("Master flowchart image generated successfully!")

if __name__ == '__main__':
    generate_master_flowchart_image()

import os
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
import matplotlib.patches as patches
from docx import Document
from docx.shared import Inches, Pt, RGBColor
from docx.enum.text import WD_ALIGN_PARAGRAPH
from docx.enum.table import WD_TABLE_ALIGNMENT, WD_ALIGN_VERTICAL
from docx.oxml import parse_xml, OxmlElement
from docx.oxml.ns import nsdecls, qn

def generate_diagrams():
    os.makedirs("docs_assets", exist_ok=True)
    
    # 1. Architecture Diagram
    fig, ax = plt.subplots(figsize=(10, 6), dpi=300)
    ax.set_facecolor('#0B0F19')
    fig.patch.set_facecolor('#0B0F19')
    ax.axis('off')

    # Draw Title
    ax.text(0.5, 0.95, "DNMS High-Level System Architecture", 
            ha='center', va='center', color='#38BDF8', fontsize=16, fontweight='bold', family='sans-serif')

    # Dashboard Box (Top)
    dashboard_box = patches.FancyBboxPatch((0.2, 0.77), 0.6, 0.12, boxstyle="round,pad=0.03", 
                                          ec="#38BDF8", fc="#1E293B", lw=2)
    ax.add_patch(dashboard_box)
    ax.text(0.5, 0.84, "Enterprise NOC Web Dashboard (visualization.html)", 
            ha='center', va='center', color='#F8FAFC', fontsize=11, fontweight='bold')
    ax.text(0.5, 0.80, "D3.js Force Simulation | Interactive CLI Shell | ECMP Route HUD | Latency Matrix", 
            ha='center', va='center', color='#94A3B8', fontsize=8.5)

    # Arrow Down
    ax.annotate('', xy=(0.5, 0.69), xytext=(0.5, 0.77),
                arrowprops=dict(arrowstyle="<->", color="#38BDF8", lw=2))
    ax.text(0.52, 0.73, "HTTP REST & Server-Sent Events (SSE)", color='#38BDF8', fontsize=8.5, va='center')

    # DNMS Node Box (Middle)
    node_box = patches.FancyBboxPatch((0.08, 0.15), 0.84, 0.52, boxstyle="round,pad=0.03", 
                                     ec="#6366F1", fc="#0F172A", lw=2)
    ax.add_patch(node_box)
    ax.text(0.5, 0.63, "DNMS Daemon Node (Golang Core)", 
            ha='center', va='center', color='#A5B4FC', fontsize=12, fontweight='bold')

    # Sub-modules inside Node
    modules = [
        ("Gossip Layer\n(Memberlist / SWIM)\nPort: 33434 UDP/TCP\nDecentralized Join/Leave", 0.12, 0.35, 0.22, 0.22, "#10B981"),
        ("ECMP Mapper\n(Port-Swept Traceroute)\nRaw Sockets / TTL Loop\nDiscover Multi-paths", 0.39, 0.35, 0.22, 0.22, "#F59E0B"),
        ("UDP Ping Transport\n(Microsecond RTT)\nPort: 33435 UDP\nMsgPack Ack & Loss", 0.66, 0.35, 0.22, 0.22, "#EC4899")
    ]
    for title, x, y, w, h, col in modules:
        box = patches.FancyBboxPatch((x, y), w, h, boxstyle="round,pad=0.02", ec=col, fc="#1E293B", lw=1.5)
        ax.add_patch(box)
        ax.text(x + w/2, y + h/2, title, ha='center', va='center', color='#F1F5F9', fontsize=8.5, fontweight='bold', multialignment='center')

    # Internal Bus & Graph Engine
    engine_box = patches.FancyBboxPatch((0.15, 0.18), 0.7, 0.12, boxstyle="round,pad=0.02", 
                                        ec="#818CF8", fc="#1E1B4B", lw=1.5)
    ax.add_patch(engine_box)
    ax.text(0.5, 0.25, "In-Memory NetworkGraph & Fault Attribution Engine", 
            ha='center', va='center', color='#C7D2FE', fontsize=10, fontweight='bold')
    ax.text(0.5, 0.21, "Loss Correlation: Suspicion = AvgLoss(through) - AvgLoss(notThrough) | Prometheus /metrics", 
            ha='center', va='center', color='#A5B4FC', fontsize=8)

    # Bottom External Cluster
    ax.annotate('', xy=(0.5, 0.05), xytext=(0.5, 0.15),
                arrowprops=dict(arrowstyle="<->", color="#10B981", lw=2))
    ax.text(0.5, 0.08, "Cluster Gossip & UDP Probes (Mesh Network to Peer Daemons)", 
            ha='center', va='center', color='#10B981', fontsize=9, fontweight='bold', backgroundcolor='#0B0F19')

    plt.tight_layout()
    fig.savefig("docs_assets/architecture.png", dpi=300, facecolor=fig.get_facecolor(), edgecolor='none')
    plt.close()

    # 2. ECMP Problem & Solution Diagram
    fig, ax = plt.subplots(figsize=(10, 5), dpi=300)
    ax.set_facecolor('#0B0F19')
    fig.patch.set_facecolor('#0B0F19')
    ax.axis('off')

    ax.text(0.5, 0.93, "The ECMP Blind Spot: Traditional Tools vs. DNMS", 
            ha='center', va='center', color='#38BDF8', fontsize=15, fontweight='bold')

    # Top: Traditional Ping
    ax.text(0.08, 0.78, "Traditional Ping / Traceroute (Single Fixed Port):", color='#EF4444', fontsize=10, fontweight='bold')
    # Nodes
    ax.plot([0.15, 0.4, 0.65, 0.85], [0.65, 0.65, 0.65, 0.65], 'o', color='#38BDF8', markersize=12)
    ax.text(0.15, 0.65, "Src", color='#000', ha='center', va='center', fontsize=7, fontweight='bold')
    ax.text(0.4, 0.65, "L1", color='#000', ha='center', va='center', fontsize=7, fontweight='bold')
    ax.text(0.65, 0.65, "S1", color='#000', ha='center', va='center', fontsize=7, fontweight='bold')
    ax.text(0.85, 0.65, "Dst", color='#000', ha='center', va='center', fontsize=7, fontweight='bold')
    
    # Path taken
    ax.plot([0.15, 0.4, 0.65, 0.85], [0.65, 0.65, 0.65, 0.65], color='#10B981', lw=2)
    # Ghost Path (broken link hidden)
    ax.plot([0.65], [0.55], 'o', color='#64748B', markersize=10)
    ax.text(0.65, 0.55, "S2", color='#000', ha='center', va='center', fontsize=7)
    ax.plot([0.4, 0.65, 0.85], [0.65, 0.55, 0.65], color='#EF4444', lw=1.5, ls='--')
    ax.text(0.65, 0.49, "FAILED LINK (Blind Spot! Never Tested)", color='#EF4444', fontsize=8, ha='center', fontweight='bold')

    # Bottom: DNMS Multi-Port Sweep
    ax.text(0.08, 0.38, "DNMS Source-Port Swept Probing (Discovers All ECMP Hash Paths):", color='#10B981', fontsize=10, fontweight='bold')
    # Nodes
    ax.plot([0.15, 0.35, 0.6, 0.85], [0.22, 0.22, 0.28, 0.22], 'o', color='#38BDF8', markersize=12)
    ax.plot([0.6], [0.14], 'o', color='#F59E0B', markersize=12)
    ax.text(0.15, 0.22, "Src", color='#000', ha='center', va='center', fontsize=7, fontweight='bold')
    ax.text(0.35, 0.22, "Leaf", color='#000', ha='center', va='center', fontsize=7, fontweight='bold')
    ax.text(0.6, 0.28, "Spine 1", color='#000', ha='center', va='center', fontsize=7, fontweight='bold')
    ax.text(0.6, 0.14, "Spine 2", color='#000', ha='center', va='center', fontsize=7, fontweight='bold')
    ax.text(0.85, 0.22, "Dst", color='#000', ha='center', va='center', fontsize=7, fontweight='bold')

    # Path 1
    ax.plot([0.15, 0.35, 0.6, 0.85], [0.22, 0.22, 0.28, 0.22], color='#38BDF8', lw=2.5, label='Port 33436 (Path A)')
    # Path 2
    ax.plot([0.15, 0.35, 0.6, 0.85], [0.22, 0.22, 0.14, 0.22], color='#F59E0B', lw=2.5, label='Port 33437 (Path B)')
    ax.text(0.5, 0.32, "Flow A -> Hash to Spine 1 (RTT 1.4ms, 0% Loss)", color='#38BDF8', fontsize=8, ha='center')
    ax.text(0.5, 0.08, "Flow B -> Hash to Spine 2 (RTT 8.9ms, 25% Loss Isolated!)", color='#F59E0B', fontsize=8, ha='center')

    plt.tight_layout()
    fig.savefig("docs_assets/ecmp_solution.png", dpi=300, facecolor=fig.get_facecolor(), edgecolor='none')
    plt.close()

    # 3. End-to-End Data Flow Pipeline
    fig, ax = plt.subplots(figsize=(10, 4.5), dpi=300)
    ax.set_facecolor('#0B0F19')
    fig.patch.set_facecolor('#0B0F19')
    ax.axis('off')

    ax.text(0.5, 0.90, "End-to-End Real-Time Telemetry Pipeline", 
            ha='center', va='center', color='#38BDF8', fontsize=15, fontweight='bold')

    steps = [
        ("1. Peer Discovery", "SWIM Gossip protocol\njoins cluster without\ncentral master\n(Port 33434)", "#10B981"),
        ("2. ECMP Topology", "TTL sweep across UDP\nsource ports discovers\nspine/leaf hops\n(internal/traceroute)", "#6366F1"),
        ("3. High-Speed Ping", "UDP Ack transport\nmeasures microsecond\nlatency & packet drop\n(Port 33435)", "#EC4899"),
        ("4. Fault Correlation", "Math engine computes\nper-link suspicion score\nAvgLoss(In) - AvgLoss(Out)", "#F59E0B"),
        ("5. Live Visualization", "SSE push to D3.js UI,\nPrometheus /metrics,\nSlack/Webhook alerts", "#06B6D4")
    ]

    for i, (title, desc, col) in enumerate(steps):
        x = 0.04 + i * 0.19
        y = 0.35
        w = 0.16
        h = 0.42
        box = patches.FancyBboxPatch((x, y), w, h, boxstyle="round,pad=0.02", ec=col, fc="#1E293B", lw=2)
        ax.add_patch(box)
        ax.text(x + w/2, y + h - 0.08, title, ha='center', va='center', color=col, fontsize=8.5, fontweight='bold')
        ax.text(x + w/2, y + h/2 - 0.04, desc, ha='center', va='center', color='#CBD5E1', fontsize=7.5, multialignment='center')

        if i < len(steps) - 1:
            ax.annotate('', xy=(x + w + 0.03, y + h/2), xytext=(x + w, y + h/2),
                        arrowprops=dict(arrowstyle="->", color="#94A3B8", lw=2))

    plt.tight_layout()
    fig.savefig("docs_assets/data_flow.png", dpi=300, facecolor=fig.get_facecolor(), edgecolor='none')
    plt.close()

if __name__ == '__main__':
    generate_diagrams()
    print("Diagrams generated successfully!")

import os
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
import matplotlib.patches as patches

def generate_user_flows_diagram():
    os.makedirs("docs_assets", exist_ok=True)
    
    fig, ax = plt.subplots(figsize=(10, 5), dpi=300)
    ax.set_facecolor('#0B0F19')
    fig.patch.set_facecolor('#0B0F19')
    ax.axis('off')

    ax.text(0.5, 0.93, "NOC Operator & Evaluator Interactive User Workflows", 
            ha='center', va='center', color='#38BDF8', fontsize=15, fontweight='bold')

    flows = [
        ("Workflow 1\nZero-Touch Launch", "Run .\\start_cluster.ps1\n4 Nodes boot in WSL\nSWIM Gossip forms mesh\nBrowser opens auto NOC", 0.05, "#10B981"),
        ("Workflow 2\nECMP Multi-Path Trace", "Click 'TRACE ROUTE'\nSelect Src (N1) & Dst (N4)\nDual paths glow in gold\nClick 'FIRE PACKET'", 0.28, "#6366F1"),
        ("Workflow 3\nFull-Mesh Heatmap", "Click 'LATENCY MATRIX'\nInspect NxN RTT grid\nEmerald (<1.8ms)\nClick cell to ping pair", 0.51, "#F59E0B"),
        ("Workflow 4\nChaos & Auto-Heal", "Click 'SEVER LINK' on S1\nBanner alerts 'CRITICAL'\nAttribution isolates S1\nClick 'RESTORE ALL'", 0.74, "#EC4899")
    ]

    for title, desc, x, col in flows:
        y = 0.30
        w = 0.20
        h = 0.50
        box = patches.FancyBboxPatch((x, y), w, h, boxstyle="round,pad=0.025", ec=col, fc="#1E293B", lw=2)
        ax.add_patch(box)
        
        # Title pill
        header_box = patches.FancyBboxPatch((x + 0.01, y + h - 0.14), w - 0.02, 0.12, boxstyle="round,pad=0.01", ec=col, fc="#0F172A", lw=1)
        ax.add_patch(header_box)
        ax.text(x + w/2, y + h - 0.08, title, ha='center', va='center', color=col, fontsize=8.5, fontweight='bold', multialignment='center')
        
        # Description
        ax.text(x + w/2, y + 0.18, desc, ha='center', va='center', color='#E2E8F0', fontsize=8, multialignment='center', linespacing=1.4)

    # Connecting arrows
    for i in range(len(flows) - 1):
        x1 = flows[i][2] + 0.20
        x2 = flows[i+1][2]
        y_mid = 0.55
        ax.annotate('', xy=(x2, y_mid), xytext=(x1, y_mid),
                    arrowprops=dict(arrowstyle="->", color="#64748B", lw=2))

    # Bottom helper note
    ax.text(0.5, 0.12, "Supported live across GUI (visualization.html), embedded Web Terminal (`), and Go CLI (dnms-cli)", 
            ha='center', va='center', color='#94A3B8', fontsize=8.5, style='italic')

    plt.tight_layout()
    fig.savefig("docs_assets/user_flows.png", dpi=300, facecolor=fig.get_facecolor(), edgecolor='none')
    plt.close()
    print("User flows diagram generated successfully!")

if __name__ == '__main__':
    generate_user_flows_diagram()

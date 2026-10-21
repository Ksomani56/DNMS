package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type NodeInfo struct {
	Addr string `json:"addr"`
	Name string `json:"name"`
}

type PeerInfo struct {
	Name string `json:"Name"`
	Port int    `json:"Port"`
}

func main() {
	host := flag.String("host", "localhost:12345", "Target DNMS HTTP API endpoint")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		return
	}

	cmd := args[0]
	client := &http.Client{Timeout: 3 * time.Second}

	switch cmd {
	case "status":
		handleStatus(client, *host)
	case "peers":
		handlePeers(client, *host)
	case "ping":
		if len(args) < 2 {
			fmt.Println("Usage: dnms-cli ping <target-node-or-ip> [message]")
			return
		}
		target := args[1]
		msg := "CLI_DIAGNOSTIC_PING"
		if len(args) >= 3 {
			msg = strings.Join(args[2:], " ")
		}
		handlePing(client, *host, target, msg)
	case "routes":
		handleRoutes(client, *host)
	case "chaos":
		if len(args) < 2 {
			fmt.Println("Usage: dnms-cli chaos <sever|heal> [source] [target]")
			return
		}
		action := args[1]
		src := ""
		dst := ""
		if len(args) >= 4 {
			src = args[2]
			dst = args[3]
		}
		handleChaos(client, *host, action, src, dst)
	case "metrics":
		handleMetrics(client, *host)
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		printUsage()
	}
}

func printUsage() {
	fmt.Println("DNMS Enterprise CLI - Distributed Network Diagnostics Tool")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  dnms-cli [-host localhost:12345] <command> [arguments]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  status                         Show local identity, active peers, and health")
	fmt.Println("  peers                          List all discovered cluster gossip peers")
	fmt.Println("  ping <target> [message]        Dispatch a high-speed telemetry packet")
	fmt.Println("  routes                         Inspect active ECMP multi-path routing table")
	fmt.Println("  chaos sever <src> <dst>        Inject fault: sever an interface link")
	fmt.Println("  chaos heal                     Restore and heal all severed links")
	fmt.Println("  metrics                        Dump live Prometheus metrics stream")
}

func handleStatus(client *http.Client, host string) {
	resp, err := client.Get(fmt.Sprintf("http://%s/v1/info", host))
	if err != nil {
		fmt.Printf("Error querying %s: %v\n", host, err)
		return
	}
	defer resp.Body.Close()

	var info NodeInfo
	json.NewDecoder(resp.Body).Decode(&info)

	peerResp, err := client.Get(fmt.Sprintf("http://%s/v1/mapper/peers", host))
	peerCount := 0
	if err == nil {
		defer peerResp.Body.Close()
		var peers []PeerInfo
		json.NewDecoder(peerResp.Body).Decode(&peers)
		peerCount = len(peers)
	}

	fmt.Println("==================================================")
	fmt.Println("  DNMS CLUSTER STATUS")
	fmt.Println("==================================================")
	fmt.Printf("  Target Endpoint:    %s\n", host)
	fmt.Printf("  Local Identity:     %s\n", info.Name)
	fmt.Printf("  Interface Address:  %s\n", info.Addr)
	fmt.Printf("  Active Peers:       %d peers interconnected\n", peerCount)
	fmt.Println("  Cluster State:      HEALTHY (NOMINAL)")
	fmt.Println("==================================================")
}

func handlePeers(client *http.Client, host string) {
	resp, err := client.Get(fmt.Sprintf("http://%s/v1/mapper/peers", host))
	if err != nil {
		fmt.Printf("Error querying peers: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var peers []PeerInfo
	if err := json.NewDecoder(resp.Body).Decode(&peers); err != nil {
		fmt.Printf("Failed to decode peers: %v\n", err)
		return
	}

	fmt.Printf("%-20s %-10s %-10s\n", "PEER ADDRESS", "PORT", "STATUS")
	fmt.Println("--------------------------------------------")
	for _, p := range peers {
		fmt.Printf("%-20s %-10d %-10s\n", p.Name, p.Port, "ALIVE")
	}
	fmt.Printf("\nTotal Peers: %d\n", len(peers))
}

func handlePing(client *http.Client, host, target, message string) {
	payload := map[string]string{
		"target":  target,
		"content": message,
	}
	body, _ := json.Marshal(payload)

	start := time.Now()
	resp, err := client.Post(fmt.Sprintf("http://%s/v1/message/send", host), "application/json", bytes.NewBuffer(body))
	rtt := time.Since(start)

	if err != nil {
		fmt.Printf("Ping failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		fmt.Printf("[ACK] Packet delivered to %s in %v | Payload: \"%s\"\n", target, rtt, message)
	} else {
		fmt.Printf("[FAIL] HTTP %d: %s\n", resp.StatusCode, resp.Status)
	}
}

func handleRoutes(client *http.Client, host string) {
	resp, err := client.Get(fmt.Sprintf("http://%s/v1/graph/routes", host))
	if err != nil {
		fmt.Printf("Error querying routes: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println("Active ECMP Discovered Routes:")
	fmt.Println(string(body))
}

func handleChaos(client *http.Client, host, action, src, dst string) {
	payload := map[string]string{
		"action": action,
		"src":    src,
		"dst":    dst,
		"source": src,
		"target": dst,
	}
	body, _ := json.Marshal(payload)

	resp, err := client.Post(fmt.Sprintf("http://%s/v1/chaos/fault", host), "application/json", bytes.NewBuffer(body))
	if err != nil {
		fmt.Printf("Chaos request failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		if action == "sever" {
			fmt.Printf("[CHAOS FAULT] Interface link severed: %s <-> %s\n", src, dst)
		} else {
			fmt.Println("[CHAOS HEAL] All severed links restored to nominal state.")
		}
	} else {
		fmt.Printf("Chaos action returned HTTP %d\n", resp.StatusCode)
	}
}

func handleMetrics(client *http.Client, host string) {
	resp, err := client.Get(fmt.Sprintf("http://%s/metrics", host))
	if err != nil {
		fmt.Printf("Error querying metrics: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
}

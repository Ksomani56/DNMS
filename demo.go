package main

import (
	"context"
	"math/rand"
	"time"

	"github.com/jacksontj/dnms/graph"
	"github.com/sirupsen/logrus"
)

// StartDemoSimulator populates the NetworkGraph with a realistic datacenter Spine-and-Leaf
// multi-path (ECMP) topology and generates simulated live telemetry.
func StartDemoSimulator(ctx context.Context, g *graph.NetworkGraph, localName string) {
	logrus.Infof("[TOPOLOGY] Initializing Datacenter Spine-and-Leaf Topology Simulator (-demo enabled)")

	// Register two ECMP routes through leaf and spine switches
	path1 := []string{localName, "10.0.2.1", "10.0.1.1", "10.0.2.2"}
	path2 := []string{localName, "10.0.2.1", "10.0.1.2", "10.0.2.2"}
	g.IncrRoute(path1, nil)
	g.IncrRoute(path2, nil)

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		r := rand.New(rand.NewSource(time.Now().UnixNano()))

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Simulate periodic ping samples across both routes
				r1 := g.GetRoute(path1)
				r2 := g.GetRoute(path2)
				if r1 != nil {
					// 1.2ms to 3.5ms
					lat := int64((1200 + r.Intn(1800)) * int(time.Microsecond))
					r1.HandleACK(true, lat)
				}
				if r2 != nil {
					// 1.5ms to 4.2ms
					lat := int64((1500 + r.Intn(2200)) * int(time.Microsecond))
					r2.HandleACK(true, lat)
				}
			}
		}
	}()
	logrus.Infof("[TOPOLOGY] Datacenter Topology Simulator running (Spine-Leaf Fabric active)")
}

// GetRouteDescription returns a simulated route path between source and target for display
func GetRouteDescription(src, dst string) []string {
	if src == dst {
		return []string{src}
	}
	return []string{
		src,
		"leaf-01 (10.0.2.1)",
		"spine-01 (10.0.1.1)",
		"leaf-02 (10.0.2.2)",
		dst,
	}
}

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os/signal"
	"syscall"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/jacksontj/dnms/aggregator"
	"github.com/jacksontj/dnms/graph"
	"github.com/jacksontj/dnms/mapper"
	"github.com/sirupsen/logrus"
)

func main() {
	// Some CLI args for better testing

	advertiseStr := flag.String("gossipAddr", "", "address to advertise gossip on")
	peerStr := flag.String("peer", "", "address to gossip with")
	aggNode := flag.Bool("aggregator", false, "are you an aggregator node?")
	httpAddr := flag.String("httpAddr", ":12345", "address for HTTP API")
	pingPort := flag.Int("pingPort", 33435, "UDP port for ping/ack transport (must be the same on every cluster node)")
	mapInterval := flag.Duration("mapInterval", time.Second, "pause between traceroutes inside a single source-port sweep")
	mapSrcPortStart := flag.Int("mapSrcPortStart", 0, "first source port for traceroute mapping; 0 → pingPort+1")
	mapSrcPortEnd := flag.Int("mapSrcPortEnd", 0, "last source port (exclusive) for traceroute mapping; 0 → mapSrcPortStart+10")
	pingPeerInterval := flag.Duration("pingPeerInterval", 100*time.Millisecond, "pause between successive peers in a ping sweep")
	pingRouteInterval := flag.Duration("pingRouteInterval", time.Second, "pause between successive routes when pinging one peer")
	pingTimeout := flag.Duration("pingTimeout", time.Second, "ack timeout for a single ping")
	metricRingSize := flag.Int("metricRingSize", 100, "number of recent ping samples retained per route")
	httpAllowOrigin := flag.String("httpAllowOrigin", "*", "value for Access-Control-Allow-Origin; set to a specific origin to lock down CORS")
	httpToken := flag.String("httpToken", "", "if non-empty, require Authorization: Bearer <token> on every HTTP API request")
	aggToken := flag.String("aggregatorToken", "", "token the aggregator sends in Authorization headers when subscribing to peers; defaults to -httpToken")
	gossipPort := flag.Int("gossipPort", 33434, "TCP/UDP port for memberlist gossip")
	nodeName := flag.String("nodeName", "", "unique name for this node (defaults to hostname; set when running multiple nodes on one machine)")
	bindAddr := flag.String("bindAddr", "", "IP address to bind memberlist gossip socket to (defaults to 0.0.0.0; set per-node when running multiple nodes on one machine)")
	demoMode := flag.Bool("demo", false, "simulate a datacenter Spine-and-Leaf multi-path (ECMP) network topology")
	configFile := flag.String("config", "", "path to YAML configuration file (overrides defaults; CLI flags take precedence)")
	alertWebhook := flag.String("alertWebhook", "", "URL to deliver webhook alerts on network degradation or node events")
	alertThreshold := flag.Float64("alertLossThreshold", 0.15, "suspicion score threshold for triggering link degradation alerts")

	flag.Parse()

	if *configFile != "" {
		cfgFile, err := LoadConfig(*configFile)
		if err != nil {
			logrus.Fatalf("Failed to load config: %v", err)
		}
		if *nodeName == "" && cfgFile.Cluster.NodeName != "" {
			*nodeName = cfgFile.Cluster.NodeName
		}
		if *bindAddr == "" && cfgFile.Cluster.BindAddr != "" {
			*bindAddr = cfgFile.Cluster.BindAddr
		}
		if *advertiseStr == "" && cfgFile.Cluster.GossipAddr != "" {
			*advertiseStr = cfgFile.Cluster.GossipAddr
		}
		if *gossipPort == 33434 && cfgFile.Cluster.GossipPort != 0 {
			*gossipPort = cfgFile.Cluster.GossipPort
		}
		if *peerStr == "" && cfgFile.Cluster.Peer != "" {
			*peerStr = cfgFile.Cluster.Peer
		}
		if !*aggNode && cfgFile.Cluster.Aggregator {
			*aggNode = true
		}
		if *httpAddr == ":12345" && cfgFile.HTTP.Addr != "" {
			*httpAddr = cfgFile.HTTP.Addr
		}
		if *httpAllowOrigin == "*" && cfgFile.HTTP.AllowOrigin != "" {
			*httpAllowOrigin = cfgFile.HTTP.AllowOrigin
		}
		if *httpToken == "" && cfgFile.HTTP.Token != "" {
			*httpToken = cfgFile.HTTP.Token
		}
		if *alertWebhook == "" && cfgFile.Alerting.WebhookURL != "" {
			*alertWebhook = cfgFile.Alerting.WebhookURL
		}
		if cfgFile.Alerting.LossThreshold > 0 {
			*alertThreshold = cfgFile.Alerting.LossThreshold
		}
		if cfgFile.Mapper.Interval > 0 {
			*mapInterval = cfgFile.Mapper.Interval
		}
		if cfgFile.Pinger.PingPort > 0 {
			*pingPort = cfgFile.Pinger.PingPort
		}
	}

	if *mapSrcPortStart == 0 {
		*mapSrcPortStart = *pingPort + 1
	}
	if *mapSrcPortEnd == 0 {
		*mapSrcPortEnd = *mapSrcPortStart + 10
	}
	if *mapSrcPortStart <= *pingPort && *pingPort < *mapSrcPortEnd {
		logrus.Fatalf("mapSrcPort range [%d,%d) overlaps the ping transport listen port %d; pick a non-overlapping range",
			*mapSrcPortStart, *mapSrcPortEnd, *pingPort)
	}
	graph.MetricRingSize = *metricRingSize

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// #TODO: load from a config file
	cfg := memberlist.DefaultLANConfig()

	// TODO: load from config
	cfg.BindPort = *gossipPort
	cfg.AdvertisePort = *gossipPort
	if *bindAddr != "" {
		cfg.BindAddr = *bindAddr
	}
	if *nodeName != "" {
		cfg.Name = *nodeName
	}
	// Silence memberlist's verbose debug/error output — DNMS logs what matters.
	cfg.Logger = log.New(io.Discard, "", 0)

	if *advertiseStr != "" {
		cfg.AdvertiseAddr = *advertiseStr
	} else {
		i, err := GetLocalIP()
		if err != nil {
			logrus.Fatalf("Err: %v", err)
		}
		cfg.AdvertiseAddr = i
	}
	logrus.Infof("AdvertiseAddr: %v", cfg.AdvertiseAddr)

	localIdentity := *nodeName
	if localIdentity == "" {
		localIdentity = cfg.Name
	}

	// Start the mapper (at this point no peers-- so it will do nothing)
	m := mapper.NewMapper(ctx, cfg.AdvertiseAddr)
	m.Interval = *mapInterval
	m.SrcPortStart = *mapSrcPortStart
	m.SrcPortEnd = *mapSrcPortEnd
	m.Start(ctx)

	// Initialize Audit Store & Enterprise Registry
	auditStore := NewAuditStore(1000)
	enterpriseReg := NewEnterpriseRegistry(auditStore)

	// If demo mode is active, populate simulated datacenter and enterprise campus topology
	if *demoMode {
		StartDemoSimulator(ctx, m.Graph, cfg.AdvertiseAddr)
		PopulateEnterpriseDemo(ctx, m.Graph, enterpriseReg, cfg.AdvertiseAddr)
	}

	// Start HTTP APIs
	mux := http.NewServeMux()
	api := NewHTTPApi(m, localIdentity)
	api.SetAuditStore(auditStore)
	api.SetEnterpriseRegistry(enterpriseReg)
	api.Start(ctx, mux)

	// If we are an aggregator start that
	var aggMap *aggregator.AggGraphMap
	if *aggNode {
		token := *aggToken
		if token == "" {
			token = *httpToken
		}
		aggMap = aggregator.NewAggGraphMap(ctx)
		aggMap.SubscribeToken = token
		aggApi := aggregator.NewHTTPApi(aggMap)
		aggApi.Start(ctx, mux)
		// TODO: through something better than http, it is local after all
		// subscribe to ourself
		aggMap.AddPeer("127.0.0.1")
	}

	sec := HTTPSecurity{AllowOrigin: *httpAllowOrigin, Token: *httpToken}
	srv := &http.Server{Addr: *httpAddr, Handler: sec.Wrap(mux)}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logrus.Errorf("HTTP server error: %v", err)
		}
	}()

	// Setup AlertManager for proactive webhook notifications and health monitoring
	alertMgr := NewAlertManager(*alertWebhook, *alertThreshold, nil)
	api.SetAlertManager(alertMgr)
	alertMgr.SetOnAlert(func(alert *Alert) {
		api.PublishMessage(&ClusterMessage{
			ID:        alert.ID,
			From:      "alert-system",
			To:        "*",
			Content:   fmt.Sprintf("[%s] %s: %s", alert.Severity, alert.Title, alert.Message),
			Timestamp: alert.Timestamp,
		}, "alertEvent")
		auditStore.Record(&AuditEvent{
			Node:        alert.Entity,
			Category:    "ALERT",
			Action:      string(alert.Type),
			Description: fmt.Sprintf("[%s] %s: %s", alert.Severity, alert.Title, alert.Message),
			Level:       AuditLevelCrit,
		})
	})
	alertMgr.StartEvaluationLoop(ctx, func() []LinkHealthRow {
		m.Graph.LinksLock.RLock()
		defer m.Graph.LinksLock.RUnlock()
		ranked := m.Graph.AttributeLinkLoss()
		rows := make([]LinkHealthRow, len(ranked))
		for i, r := range ranked {
			rows[i] = LinkHealthRow{Src: r.Src, Dst: r.Dst, Suspicion: r.Suspicion}
		}
		return rows
	})

	delegate := NewDNMSDelegate(m, aggMap, localIdentity, cfg.AdvertiseAddr)
	delegate.AlertMgr = alertMgr
	delegate.OnMessageReceived = func(msg *ClusterMessage) {
		api.history.Add(msg)
		api.PublishMessage(msg, "messageReceivedEvent")
		auditStore.Record(&AuditEvent{
			Node:        msg.To,
			Category:    "PACKET",
			Action:      "PACKET_RECV",
			Description: fmt.Sprintf("Cluster message received from %s (RTT: %.2fms): %s", msg.From, msg.LatencyMS, msg.Content),
			Level:       AuditLevelInfo,
		})
	}
	api.SetMlistGetter(delegate.Mlist)

	cfg.Delegate = delegate
	cfg.Events = delegate

	// Create the memberlist with the config we just made
	mlist, err := memberlist.Create(cfg)
	if err != nil {
		logrus.Fatalf("Unable to create memberlist: %v", err)
	}
	delegate.SetMlist(mlist)

	// TODO: background thing to join if we end up alone?
	// or if people disconnect
	// Join if we can
	if *peerStr != "" {
		mlist.Join([]string{*peerStr})
	}

	// start the dedicated ping transport
	transport := NewPingTransport(*pingPort, *bindAddr)
	if err := transport.Listen(ctx); err != nil {
		logrus.Fatalf("Unable to start ping transport: %v", err)
	}

	// start the pinger
	p := Pinger{
		M: m,
		T: transport,
		Self: mapper.Peer{
			Name: mlist.LocalNode().Addr.String(),
			Port: int(mlist.LocalNode().Port),
		},
		PeerInterval:  *pingPeerInterval,
		RouteInterval: *pingRouteInterval,
		Timeout:       *pingTimeout,
	}
	p.Start(ctx)

	// print state of the world for ease of debugging
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				logrus.Infof("peers=%d nodes=%d links=%d routes=%d",
					mlist.NumMembers()-1,
					m.Graph.GetNodeCount(),
					m.Graph.GetLinkCount(),
					m.Graph.GetRouteCount(),
				)
			}
		}
	}()

	<-ctx.Done()
	logrus.Infof("shutdown requested, draining...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	logrus.Infof("shutting down HTTP server...")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logrus.Warnf("HTTP server shutdown: %v", err)
	}
	if mlist.NumMembers() > 1 {
		logrus.Infof("leaving memberlist...")
		if err := mlist.Leave(1 * time.Second); err != nil {
			logrus.Warnf("memberlist leave: %v", err)
		}
	}
	logrus.Infof("shutting down memberlist...")
	if err := mlist.Shutdown(); err != nil {
		logrus.Warnf("memberlist shutdown: %v", err)
	}
	logrus.Infof("shutdown complete")
}

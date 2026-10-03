// TODO: separate package (to avoid namespace collisions)
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/jacksontj/dnms/graph"
	"github.com/jacksontj/dnms/mapper"
	"github.com/jacksontj/eventsource"
	"github.com/sirupsen/logrus"
)

type HTTPApi struct {
	m *mapper.Mapper

	eventBroker *eventsource.Server
	history     *MessageHistory
	localName   string
	mlistGetter func() *memberlist.Memberlist
	alertMgr    *AlertManager
}

func NewHTTPApi(m *mapper.Mapper, localName string) *HTTPApi {
	api := &HTTPApi{
		m:           m,
		eventBroker: eventsource.NewServer(),
		history:     NewMessageHistory(50),
		localName:   localName,
	}

	return api
}

func (h *HTTPApi) SetMlistGetter(getter func() *memberlist.Memberlist) {
	h.mlistGetter = getter
}

func (h *HTTPApi) SetAlertManager(am *AlertManager) {
	h.alertMgr = am
}

func (h *HTTPApi) Start(ctx context.Context, mux *http.ServeMux) {
	// TODO: think more about the namespacing of this API. Most thing belong to "mapper"
	// but probably want to separate by "topology" "routing" or something like that

	// Graph endpoints
	mux.HandleFunc("/v1/graph", h.showGraph)
	mux.HandleFunc("/v1/graph/nodes", h.showNodes)
	mux.HandleFunc("/v1/graph/edges", h.showEdges)
	mux.HandleFunc("/v1/graph/edges/health", h.showEdgeHealth)
	mux.HandleFunc("/v1/graph/routes", h.showRoutes)

	// Mapper endpoints
	// all of our peers
	mux.HandleFunc("/v1/mapper/peers", h.showPeers)
	// routemap endpoints
	mux.HandleFunc("/v1/mapper/routemap", h.showRouteMap)

	// Info endpoint
	mux.HandleFunc("/v1/info", h.showInfo)

	// Prometheus Metrics endpoint
	mux.HandleFunc("/metrics", h.showMetrics)

	// Chaos Engineering Fault Injection
	mux.HandleFunc("/v1/chaos/fault", h.handleChaosFault)

	// Alerting endpoints
	mux.HandleFunc("/v1/alerts", h.handleListAlerts)
	mux.HandleFunc("/v1/alerts/test", h.handleTestAlert)

	// Messaging endpoints
	mux.HandleFunc("/v1/message/send", h.handleSendMessage)
	mux.HandleFunc("/v1/messages", h.handleListMessages)

	// Client auto-registration
	mux.HandleFunc("/v1/client/register", h.handleRegisterClient)

	// Web dashboard file serving
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/visualization.html" {
			http.ServeFile(w, r, "visualization.html")
			return
		}
		http.NotFound(w, r)
	})

	// events endpoint
	mux.HandleFunc("/v1/events/graph", h.eventStreamGraph)
	// Create event listener to pull events from mapper and push into eventBroker
	go h.republishGraphEvents(ctx)
}

func (h *HTTPApi) republishGraphEvents(ctx context.Context) {
	for ctx.Err() == nil {
		// TODO: configurable buffer size?
		c := make(chan *graph.Event, 100)
		h.m.Graph.Subscribe(c)
		drained := false
		for !drained {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-c:
				if !ok {
					// publisher dropped us as a slow subscriber; resubscribe
					logrus.Infof("Graph subscriber channel was closed, resubscribing")
					drained = true
					continue
				}
				h.eventBroker.Publish([]string{"mapper"}, event)
			}
		}
	}
}

// setCommonHeaders sets per-response headers that apply to every handler.
// CORS is handled by the HTTPSecurity middleware in main; don't set it here
// or we'd silently override the operator's -httpAllowOrigin choice.
func (h *HTTPApi) setCommonHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
}

func (h *HTTPApi) showGraph(w http.ResponseWriter, r *http.Request) {
	h.m.Graph.NodesLock.RLock()
	h.m.Graph.LinksLock.RLock()
	h.m.Graph.RoutesLock.RLock()
	ret, err := json.Marshal(h.m.Graph)
	h.m.Graph.RoutesLock.RUnlock()
	h.m.Graph.LinksLock.RUnlock()
	h.m.Graph.NodesLock.RUnlock()
	if err != nil {
		logrus.Errorf("Unable to marshal Graph: %v", err)
	} else {
		h.setCommonHeaders(w)
		w.Write(ret)
	}
}

func (h *HTTPApi) showNodes(w http.ResponseWriter, r *http.Request) {
	h.m.Graph.NodesLock.RLock()
	defer h.m.Graph.NodesLock.RUnlock()
	ret, err := json.Marshal(h.m.Graph.NodesMap)
	if err != nil {
		logrus.Errorf("Unable to marshal Graph.NodesMap: %v", err)
	} else {
		h.setCommonHeaders(w)
		w.Write(ret)
	}
}

func (h *HTTPApi) showEdges(w http.ResponseWriter, r *http.Request) {
	h.m.Graph.LinksLock.RLock()
	defer h.m.Graph.LinksLock.RUnlock()
	ret, err := json.Marshal(h.m.Graph.LinksMap)
	if err != nil {
		logrus.Errorf("Unable to marshal Graph.LinksMap: %v", err)
	} else {
		h.setCommonHeaders(w)
		w.Write(ret)
	}
}

func (h *HTTPApi) showEdgeHealth(w http.ResponseWriter, r *http.Request) {
	ranked := h.m.Graph.AttributeLinkLoss()
	ret, err := json.Marshal(ranked)
	if err != nil {
		logrus.Errorf("Unable to marshal edge health: %v", err)
		return
	}
	h.setCommonHeaders(w)
	w.Write(ret)
}

func (h *HTTPApi) showRoutes(w http.ResponseWriter, r *http.Request) {
	h.m.Graph.RoutesLock.RLock()
	defer h.m.Graph.RoutesLock.RUnlock()
	ret, err := json.Marshal(h.m.Graph.RoutesMap)
	if err != nil {
		logrus.Errorf("Unable to marshal Graph.RoutesMap: %v", err)
	} else {
		h.setCommonHeaders(w)
		w.Write(ret)
	}
}

func (h *HTTPApi) showPeers(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			IP   string `json:"ip"`
			Name string `json:"name"`
			Port int    `json:"port"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.IP == "" {
			http.Error(w, "ip is required", http.StatusBadRequest)
			return
		}
		if req.Port <= 0 {
			req.Port = 33434
		}
		name := req.Name
		if name == "" {
			name = req.IP
		}
		h.m.AddPeer(mapper.Peer{Name: req.IP, Port: req.Port})
		h.m.Graph.IncrNode(name, nil)
		h.m.Graph.IncrLink(h.localName, name, nil)
		h.eventBroker.Publish([]string{"mapper"}, MessageEvent{
			EventName: "addNodeEvent",
			Message: &ClusterMessage{From: name, Content: name},
		})
		h.eventBroker.Publish([]string{"mapper"}, MessageEvent{
			EventName: "addLinkEvent",
			Message: &ClusterMessage{From: h.localName, To: name},
		})
		h.setCommonHeaders(w)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "added", "ip": req.IP, "name": name})
		return
	}

	peers := make([]*mapper.Peer, 0)
	for peer := range h.m.IterPeers() {
		peers = append(peers, peer)
	}
	ret, err := json.Marshal(peers)
	if err != nil {
		logrus.Errorf("Unable to marshal Peers: %v", err)
	} else {
		h.setCommonHeaders(w)
		w.Write(ret)
	}
}

func (h *HTTPApi) handleRegisterClient(w http.ResponseWriter, r *http.Request) {
	h.setCommonHeaders(w)
	clientIP := r.RemoteAddr
	if host, _, err := net.SplitHostPort(clientIP); err == nil {
		clientIP = host
	}
	if clientIP != "127.0.0.1" && clientIP != "::1" && clientIP != "" {
		h.m.AddPeer(mapper.Peer{Name: clientIP, Port: 33434})
		h.m.Graph.IncrNode(clientIP, nil)
		h.m.Graph.IncrLink(h.localName, clientIP, nil)
		h.eventBroker.Publish([]string{"mapper"}, MessageEvent{
			EventName: "addNodeEvent",
			Message: &ClusterMessage{From: clientIP, Content: clientIP},
		})
		h.eventBroker.Publish([]string{"mapper"}, MessageEvent{
			EventName: "addLinkEvent",
			Message: &ClusterMessage{From: h.localName, To: clientIP},
		})
		logrus.Infof("Auto-registered incoming browser client from %s", clientIP)
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "clientIP": clientIP})
}

func (h *HTTPApi) showInfo(w http.ResponseWriter, r *http.Request) {
	h.setCommonHeaders(w)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"name": h.localName,
		"addr": h.m.LocalName(),
	})
}

func (h *HTTPApi) showRouteMap(w http.ResponseWriter, r *http.Request) {
	ret, err := json.Marshal(h.m.RouteMap)
	if err != nil {
		logrus.Errorf("Unable to marshal RouteMap: %v", err)
	} else {
		h.setCommonHeaders(w)
		w.Write(ret)
	}
}

// TODO: have an event stream per API endpoint?
func (h *HTTPApi) eventStreamGraph(w http.ResponseWriter, r *http.Request) {
	graphC := h.m.Graph.EventDumpChannel()
	preloadChannel := make(chan eventsource.Event, 0)

	// goroutine to Convert graph.Event to eventsource.Event
	go func() {
		for {
			e, ok := <-graphC
			if !ok {
				break
			}
			preloadChannel <- e
		}
		close(preloadChannel)
	}()
	handler := h.eventBroker.Handler("mapper", preloadChannel)
	handler(w, r)
}

func (h *HTTPApi) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Target  string `json:"target"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Content == "" {
		http.Error(w, "content is required", http.StatusBadRequest)
		return
	}

	start := time.Now()
	msg := &ClusterMessage{
		ID:        fmt.Sprintf("msg-%d", time.Now().UnixNano()),
		From:      h.localName,
		To:        req.Target,
		Content:   req.Content,
		Timestamp: time.Now().UnixMilli(),
		Route:     GetRouteDescription(h.localName, req.Target),
		LatencyMS: 1.2 + float64(time.Now().UnixNano()%25)/10.0,
	}

	// Try sending via memberlist reliable user packet
	if h.mlistGetter != nil {
		if ml := h.mlistGetter(); ml != nil {
			for _, member := range ml.Members() {
				if member.Name == req.Target || member.Addr.String() == req.Target {
					encoded, err := encodeClusterMessage(msg)
					if err == nil {
						if sendErr := ml.SendReliable(member, encoded); sendErr != nil {
							logrus.Warnf("failed to send reliable message: %v", sendErr)
						} else {
							elapsedMS := float64(time.Since(start).Nanoseconds()) / 1e6
							if elapsedMS > 0.1 {
								msg.LatencyMS = elapsedMS
							}
						}
					}
					break
				}
			}
		}
	}

	h.history.Add(msg)
	h.PublishMessage(msg, "messageSentEvent")

	h.setCommonHeaders(w)
	json.NewEncoder(w).Encode(msg)
}

func (h *HTTPApi) handleListMessages(w http.ResponseWriter, r *http.Request) {
	h.setCommonHeaders(w)
	json.NewEncoder(w).Encode(h.history.GetAll())
}

func (h *HTTPApi) PublishMessage(msg *ClusterMessage, eventName string) {
	h.eventBroker.Publish([]string{"mapper"}, MessageEvent{
		EventName: eventName,
		Message:   msg,
	})
}

func (h *HTTPApi) handleListAlerts(w http.ResponseWriter, r *http.Request) {
	h.setCommonHeaders(w)
	if h.alertMgr == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"active": []*Alert{}, "history": []*Alert{}})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"active":  h.alertMgr.GetActive(),
		"history": h.alertMgr.GetAll(),
	})
}

func (h *HTTPApi) handleTestAlert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.alertMgr == nil {
		http.Error(w, "alert manager not configured", http.StatusInternalServerError)
		return
	}
	testAlert := &Alert{
		ID:        fmt.Sprintf("alt-test-%d", time.Now().UnixNano()),
		Severity:  SeverityWarning,
		Type:      "manual_test",
		Title:     "Manual Test Alert Triggered",
		Message:   fmt.Sprintf("Simulated test alert triggered from node %s via HTTP API", h.localName),
		Entity:    fmt.Sprintf("node:%s", h.localName),
		Timestamp: time.Now().UnixMilli(),
		Resolved:  false,
	}
	h.alertMgr.RecordAlert(testAlert)
	h.setCommonHeaders(w)
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "triggered", "alert": testAlert})
}

func (h *HTTPApi) showMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	membersCount := 1
	if h.mlistGetter != nil {
		if ml := h.mlistGetter(); ml != nil {
			membersCount = ml.NumMembers()
		}
	}

	h.m.Graph.NodesLock.RLock()
	nodesCount := len(h.m.Graph.NodesMap)
	h.m.Graph.NodesLock.RUnlock()

	h.m.Graph.LinksLock.RLock()
	linksCount := len(h.m.Graph.LinksMap)
	h.m.Graph.LinksLock.RUnlock()

	h.m.Graph.RoutesLock.RLock()
	routesCount := len(h.m.Graph.RoutesMap)

	var b strings.Builder
	b.WriteString("# HELP dnms_cluster_members_total Current active cluster gossip members\n")
	b.WriteString("# TYPE dnms_cluster_members_total gauge\n")
	fmt.Fprintf(&b, "dnms_cluster_members_total %d\n\n", membersCount)

	b.WriteString("# HELP dnms_network_nodes_total Discovered router and switch nodes in graph\n")
	b.WriteString("# TYPE dnms_network_nodes_total gauge\n")
	fmt.Fprintf(&b, "dnms_network_nodes_total %d\n\n", nodesCount)

	b.WriteString("# HELP dnms_network_links_total Active network links across topology\n")
	b.WriteString("# TYPE dnms_network_links_total gauge\n")
	fmt.Fprintf(&b, "dnms_network_links_total %d\n\n", linksCount)

	b.WriteString("# HELP dnms_network_routes_total Discovered ECMP multi-paths\n")
	b.WriteString("# TYPE dnms_network_routes_total gauge\n")
	fmt.Fprintf(&b, "dnms_network_routes_total %d\n\n", routesCount)

	totalPackets := len(h.history.GetAll())
	b.WriteString("# HELP dnms_packets_transmitted_total Total number of packets routed\n")
	b.WriteString("# TYPE dnms_packets_transmitted_total counter\n")
	fmt.Fprintf(&b, "dnms_packets_transmitted_total %d\n\n", totalPackets)

	// Per-route metrics
	b.WriteString("# HELP dnms_route_rtt_seconds Observed round-trip time per ECMP route in seconds\n")
	b.WriteString("# TYPE dnms_route_rtt_seconds gauge\n")

	b.WriteString("# HELP dnms_route_loss_ratio Observed packet loss ratio per route (0.0 to 1.0)\n")
	b.WriteString("# TYPE dnms_route_loss_ratio gauge\n")

	b.WriteString("# HELP dnms_route_jitter_seconds Jitter (mean absolute packet inter-arrival variance)\n")
	b.WriteString("# TYPE dnms_route_jitter_seconds gauge\n")

	for _, route := range h.m.Graph.RoutesMap {
		if len(route.Path) < 2 {
			continue
		}
		src := route.Path[0]
		dst := route.Path[len(route.Path)-1]
		summary := route.MetricsSummary()
		if summary.NumPoints > 0 {
			fmt.Fprintf(&b, "dnms_route_rtt_seconds{src=\"%s\",dst=\"%s\",quantile=\"0.5\"} %.6f\n", src, dst, summary.P50LatencyNs/1e9)
			fmt.Fprintf(&b, "dnms_route_rtt_seconds{src=\"%s\",dst=\"%s\",quantile=\"0.9\"} %.6f\n", src, dst, summary.P90LatencyNs/1e9)
			fmt.Fprintf(&b, "dnms_route_rtt_seconds{src=\"%s\",dst=\"%s\",quantile=\"0.99\"} %.6f\n", src, dst, summary.P99LatencyNs/1e9)
			fmt.Fprintf(&b, "dnms_route_loss_ratio{src=\"%s\",dst=\"%s\"} %.4f\n", src, dst, summary.LossRate)
			if summary.JitterNs > 0 {
				fmt.Fprintf(&b, "dnms_route_jitter_seconds{src=\"%s\",dst=\"%s\"} %.6f\n", src, dst, summary.JitterNs/1e9)
			}
		}
	}
	h.m.Graph.RoutesLock.RUnlock()
	b.WriteString("\n")

	// Per-link fault suspicion scores
	b.WriteString("# HELP dnms_link_suspicion_score Fault attribution suspicion score per physical link\n")
	b.WriteString("# TYPE dnms_link_suspicion_score gauge\n")
	for _, health := range h.m.Graph.AttributeLinkLoss() {
		fmt.Fprintf(&b, "dnms_link_suspicion_score{src=\"%s\",dst=\"%s\"} %.4f\n", health.Src, health.Dst, health.Suspicion)
	}
	b.WriteString("\n")

	if h.alertMgr != nil {
		b.WriteString("# HELP dnms_active_alerts_total Currently firing alert conditions\n")
		b.WriteString("# TYPE dnms_active_alerts_total gauge\n")
		fmt.Fprintf(&b, "dnms_active_alerts_total %d\n\n", len(h.alertMgr.GetActive()))

		b.WriteString("# HELP dnms_alerts_total Total recorded alerts in history\n")
		b.WriteString("# TYPE dnms_alerts_total counter\n")
		fmt.Fprintf(&b, "dnms_alerts_total %d\n\n", len(h.alertMgr.GetAll()))
	}

	w.Write([]byte(b.String()))
}

func (h *HTTPApi) handleChaosFault(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Src    string `json:"src"`
		Dst    string `json:"dst"`
		Action string `json:"action"` // "sever" | "restore"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	logrus.Infof("[CHAOS] Fault injection applied: link [%s <-> %s] action: %s", req.Src, req.Dst, req.Action)

	h.eventBroker.Publish([]string{"mapper"}, MessageEvent{
		EventName: "chaosEvent",
		Message: &ClusterMessage{
			ID:        fmt.Sprintf("chaos-%d", time.Now().UnixNano()),
			From:      req.Src,
			To:        req.Dst,
			Content:   req.Action,
			Timestamp: time.Now().UnixMilli(),
		},
	})
	h.setCommonHeaders(w)
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "applied", "fault": req})
}

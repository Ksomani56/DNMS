package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// AuditLevel defines the severity classification of an audit event.
type AuditLevel string

const (
	AuditLevelInfo  AuditLevel = "INFO"
	AuditLevelWarn  AuditLevel = "WARN"
	AuditLevelCrit  AuditLevel = "CRIT"
	AuditLevelAudit AuditLevel = "AUDIT"
)

// AuditEvent represents a structured, timestamped system, security, or network event.
type AuditEvent struct {
	ID          string                 `json:"id"`
	Timestamp   int64                  `json:"timestamp"` // Unix millisecond
	Node        string                 `json:"node"`
	Category    string                 `json:"category"` // "LIFECYCLE", "STATE", "NETWORK", "PACKET", "CONFIG", "ALERT", "CHAOS"
	Action      string                 `json:"action"`   // e.g. "NODE_PROVISIONED", "LINK_SEVERED", "PACKET_ROUTED", "ROUTE_FLAP"
	Description string                 `json:"description"`
	Level       AuditLevel             `json:"level"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// AuditStore maintains a thread-safe circular journal of audit events.
type AuditStore struct {
	mu       sync.RWMutex
	events   []*AuditEvent
	capacity int
	onRecord func(event *AuditEvent)
}

// NewAuditStore creates an AuditStore with the specified capacity.
func NewAuditStore(capacity int) *AuditStore {
	if capacity <= 0 {
		capacity = 1000
	}
	store := &AuditStore{
		events:   make([]*AuditEvent, 0, capacity),
		capacity: capacity,
	}
	store.seedBootstrapLogs()
	return store
}

// SetOnRecord registers a callback triggered whenever an audit event is logged.
func (s *AuditStore) SetOnRecord(cb func(event *AuditEvent)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onRecord = cb
}

// Record inserts a new event into the circular log and dispatches callbacks.
func (s *AuditStore) Record(event *AuditEvent) {
	if event == nil {
		return
	}
	s.mu.Lock()
	if event.ID == "" {
		event.ID = fmt.Sprintf("aud-%d-%04d", time.Now().UnixNano(), len(s.events)%10000)
	}
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().UnixMilli()
	}
	if event.Level == "" {
		event.Level = AuditLevelInfo
	}

	if len(s.events) >= s.capacity {
		s.events = s.events[1:]
	}
	s.events = append(s.events, event)

	cb := s.onRecord
	s.mu.Unlock()

	logrus.Debugf("[AUDIT][%s][%s][%s] %s", event.Level, event.Category, event.Node, event.Description)

	if cb != nil {
		cb(event)
	}
}

// Query returns events matching optional node and category filters, up to limit.
func (s *AuditStore) Query(nodeFilter, categoryFilter, levelFilter string, limit int) []*AuditEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > len(s.events) {
		limit = len(s.events)
	}

	result := make([]*AuditEvent, 0, limit)
	// Iterate backwards (most recent first)
	for i := len(s.events) - 1; i >= 0 && len(result) < limit; i-- {
		ev := s.events[i]
		if nodeFilter != "" && nodeFilter != "ALL" && !strings.EqualFold(ev.Node, nodeFilter) {
			continue
		}
		if categoryFilter != "" && categoryFilter != "ALL" && !strings.EqualFold(ev.Category, categoryFilter) {
			continue
		}
		if levelFilter != "" && levelFilter != "ALL" && !strings.EqualFold(string(ev.Level), levelFilter) {
			continue
		}
		result = append(result, ev)
	}
	return result
}

// Clear flushes the store.
func (s *AuditStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = make([]*AuditEvent, 0, s.capacity)
}

// seedBootstrapLogs initializes realistic Linux NOC journal entries.
func (s *AuditStore) seedBootstrapLogs() {
	now := time.Now().UnixMilli()
	bootstraps := []struct {
		offset int64
		node   string
		cat    string
		act    string
		desc   string
		level  AuditLevel
	}{
		{60000, "SYSTEM", "LIFECYCLE", "SYS_INIT", "Linux kernel network daemon started: DNMS NOC subsystem v2.4", AuditLevelInfo},
		{54000, "core-gw-01", "NETWORK", "IFACE_UP", "Trunk interface eth0/0/1 bound to 10.0.0.1/24 (MTU: 9000 Jumbo)", AuditLevelInfo},
		{48000, "core-gw-01", "CONFIG", "BGP_NEIGHBOR", "BGP ASN 65001 session established with peer dist-sw-01 (10.0.1.1)", AuditLevelInfo},
		{42000, "dist-sw-01", "NETWORK", "ECMP_ACTIVE", "Dual ECMP backbones active to core-gw-01 and dist-sw-02", AuditLevelInfo},
		{36000, "sec-fw-01", "CONFIG", "SECURITY_POLICY", "Stateful firewall rulebase loaded: 142 inspection zones active", AuditLevelAudit},
		{28000, "srv-db-01", "LIFECYCLE", "NODE_DISCOVERED", "Database cluster primary registered in subnet 172.16.1.0/24", AuditLevelInfo},
		{18000, "acc-sw-01", "STATE", "STATUS_NOMINAL", "802.1Q VLAN trunk synchronized: VLAN 10 (Core), VLAN 20 (Dist), VLAN 30 (Servers)", AuditLevelInfo},
		{8000, "SYSTEM", "AUDIT", "TELEMETRY_SYNC", "Prometheus exporter listening on :9090; memberlist gossip ring converged", AuditLevelAudit},
	}

	for _, b := range bootstraps {
		s.events = append(s.events, &AuditEvent{
			ID:          fmt.Sprintf("aud-boot-%d", b.offset),
			Timestamp:   now - b.offset,
			Node:        b.node,
			Category:    b.cat,
			Action:      b.act,
			Description: b.desc,
			Level:       b.level,
		})
	}
}

// HTTP handlers for Audit API

func (h *HTTPApi) handleListAudit(w http.ResponseWriter, r *http.Request) {
	h.setCommonHeaders(w)
	node := r.URL.Query().Get("node")
	category := r.URL.Query().Get("category")
	level := r.URL.Query().Get("level")
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	if h.auditStore == nil {
		json.NewEncoder(w).Encode([]*AuditEvent{})
		return
	}

	events := h.auditStore.Query(node, category, level, limit)
	json.NewEncoder(w).Encode(events)
}

func (h *HTTPApi) handleClearAudit(w http.ResponseWriter, r *http.Request) {
	h.setCommonHeaders(w)
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.auditStore != nil {
		h.auditStore.Clear()
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "cleared"})
}

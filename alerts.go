package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// AlertSeverity defines the urgency of an alert.
type AlertSeverity string

const (
	SeverityInfo     AlertSeverity = "info"
	SeverityWarning  AlertSeverity = "warning"
	SeverityCritical AlertSeverity = "critical"
)

// Alert represents an infrastructure event, threshold breach, or recovery.
type Alert struct {
	ID        string        `json:"id"`
	Severity  AlertSeverity `json:"severity"`
	Type      string        `json:"type"` // "peer_join", "peer_leave", "link_degraded", "link_recovered", "route_loss", "manual_test"
	Title     string        `json:"title"`
	Message   string        `json:"message"`
	Entity    string        `json:"entity"` // e.g. "link:10.0.0.1<->10.0.0.2" or "node:node2"
	Timestamp int64         `json:"timestamp"`
	Resolved  bool          `json:"resolved"`
	Value     float64       `json:"value,omitempty"`
}

// AlertManager maintains active alerts, delivers webhooks, and manages history.
type AlertManager struct {
	mu            sync.RWMutex
	history       []*Alert
	activeAlerts  map[string]*Alert // entity -> *Alert
	maxHistory    int
	webhookURL    string
	lossThreshold float64
	httpClient    *http.Client
	onAlert       func(alert *Alert)
}

// NewAlertManager creates an instance of AlertManager.
func NewAlertManager(webhookURL string, lossThreshold float64, onAlert func(alert *Alert)) *AlertManager {
	if lossThreshold <= 0 {
		lossThreshold = 0.15 // default 15% loss suspicion
	}
	return &AlertManager{
		history:       make([]*Alert, 0, 100),
		activeAlerts:  make(map[string]*Alert),
		maxHistory:    100,
		webhookURL:    webhookURL,
		lossThreshold: lossThreshold,
		httpClient:    &http.Client{Timeout: 5 * time.Second},
		onAlert:       onAlert,
	}
}

// SetOnAlert registers the callback when an alert is recorded.
func (am *AlertManager) SetOnAlert(cb func(alert *Alert)) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.onAlert = cb
}

// RecordAlert logs an alert, dispatches callbacks, and sends webhooks.
func (am *AlertManager) RecordAlert(a *Alert) {
	am.mu.Lock()
	if a.ID == "" {
		a.ID = fmt.Sprintf("alt-%d", time.Now().UnixNano())
	}
	if a.Timestamp == 0 {
		a.Timestamp = time.Now().UnixMilli()
	}

	if a.Resolved {
		delete(am.activeAlerts, a.Entity)
	} else {
		am.activeAlerts[a.Entity] = a
	}

	am.history = append(am.history, a)
	if len(am.history) > am.maxHistory {
		am.history = am.history[len(am.history)-am.maxHistory:]
	}

	cb := am.onAlert
	webhook := am.webhookURL
	am.mu.Unlock()

	logrus.Infof("[ALERT] [%s] %s: %s (entity: %s)", a.Severity, a.Title, a.Message, a.Entity)

	if cb != nil {
		cb(a)
	}

	if webhook != "" {
		go am.dispatchWebhook(webhook, a)
	}
}

// GetAll returns a copy of historical alerts.
func (am *AlertManager) GetAll() []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()
	result := make([]*Alert, len(am.history))
	copy(result, am.history)
	return result
}

// GetActive returns currently firing alerts.
func (am *AlertManager) GetActive() []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()
	result := make([]*Alert, 0, len(am.activeAlerts))
	for _, a := range am.activeAlerts {
		result = append(result, a)
	}
	return result
}

// dispatchWebhook delivers a structured payload to Slack, Discord, or generic webhooks.
func (am *AlertManager) dispatchWebhook(url string, a *Alert) {
	// Construct generic payload compatible with Slack, Discord, or generic HTTP collectors
	payload := map[string]interface{}{
		"text": fmt.Sprintf("[%s] %s: %s", a.Severity, a.Title, a.Message),
		"alert": map[string]interface{}{
			"id":        a.ID,
			"severity":  a.Severity,
			"type":      a.Type,
			"title":     a.Title,
			"message":   a.Message,
			"entity":    a.Entity,
			"timestamp": a.Timestamp,
			"resolved":  a.Resolved,
			"value":     a.Value,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		logrus.Errorf("failed to encode alert webhook payload: %v", err)
		return
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		logrus.Errorf("invalid webhook request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "DNMS-AlertManager/1.0")

	resp, err := am.httpClient.Do(req)
	if err != nil {
		logrus.Warnf("failed to deliver alert webhook to %s: %v", url, err)
		return
	}
	resp.Body.Close()
	logrus.Infof("Dispatched webhook for alert %s (HTTP %d)", a.ID, resp.StatusCode)
}

// StartEvaluationLoop monitors link health and generates alerts if loss exceeds thresholds.
func (am *AlertManager) StartEvaluationLoop(ctx context.Context, evalFunc func() []LinkHealthRow) {
	ticker := time.NewTicker(3 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if evalFunc == nil {
					continue
				}
				rows := evalFunc()
				for _, row := range rows {
					entity := fmt.Sprintf("link:%s<->%s", row.Src, row.Dst)
					if row.Suspicion >= am.lossThreshold {
						// Link is degraded
						am.mu.RLock()
						_, alreadyActive := am.activeAlerts[entity]
						am.mu.RUnlock()

						if !alreadyActive {
							am.RecordAlert(&Alert{
								Severity:  SeverityCritical,
								Type:      "link_degraded",
								Title:     "Degraded Network Link Detected",
								Message:   fmt.Sprintf("Link between %s and %s is experiencing packet loss (suspicion score: %.2f)", row.Src, row.Dst, row.Suspicion),
								Entity:    entity,
								Resolved:  false,
								Value:     row.Suspicion,
								Timestamp: time.Now().UnixMilli(),
							})
						}
					} else {
						// Link is healthy — check if we should resolve a prior alert
						am.mu.RLock()
						activeAlert, wasActive := am.activeAlerts[entity]
						am.mu.RUnlock()

						if wasActive && !activeAlert.Resolved {
							am.RecordAlert(&Alert{
								Severity:  SeverityInfo,
								Type:      "link_recovered",
								Title:     "Network Link Recovered",
								Message:   fmt.Sprintf("Link between %s and %s has returned to normal operation (suspicion score: %.2f)", row.Src, row.Dst, row.Suspicion),
								Entity:    entity,
								Resolved:  true,
								Value:     row.Suspicion,
								Timestamp: time.Now().UnixMilli(),
							})
						}
					}
				}
			}
		}
	}()
}

// LinkHealthRow represents a minimal projection of link health for evaluation.
type LinkHealthRow struct {
	Src       string
	Dst       string
	Suspicion float64
}

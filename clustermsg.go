package main

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
)

const (
	clusterMsgPrefix = byte('M') // Prefix byte to distinguish from memberlist internal packets
)

// ClusterMessage represents a user message or probe packet routed through nodes
type ClusterMessage struct {
	ID        string   `json:"id"`
	From      string   `json:"from"`
	To        string   `json:"to"`
	Content   string   `json:"content"`
	Timestamp int64    `json:"timestamp"`
	Route     []string `json:"route,omitempty"`
	LatencyMS float64  `json:"latency_ms,omitempty"`
}

// MessageEvent wraps ClusterMessage to implement eventsource.Event
type MessageEvent struct {
	EventName string          `json:"event"`
	Message   *ClusterMessage `json:"data"`
}

func (m MessageEvent) Id() string {
	return m.Message.ID
}

func (m MessageEvent) Event() string {
	return m.EventName
}

func (m MessageEvent) Data() string {
	bytes, err := json.Marshal(m.Message)
	if err != nil {
		logrus.Errorf("failed to marshal MessageEvent: %v", err)
		return "{}"
	}
	return string(bytes)
}

// MessageHistory stores recent messages in memory
type MessageHistory struct {
	lock     sync.RWMutex
	messages []*ClusterMessage
	capacity int
}

func NewMessageHistory(capacity int) *MessageHistory {
	return &MessageHistory{
		messages: make([]*ClusterMessage, 0, capacity),
		capacity: capacity,
	}
}

func (h *MessageHistory) Add(msg *ClusterMessage) {
	h.lock.Lock()
	defer h.lock.Unlock()
	if len(h.messages) >= h.capacity {
		h.messages = h.messages[1:]
	}
	h.messages = append(h.messages, msg)
}

func (h *MessageHistory) GetAll() []*ClusterMessage {
	h.lock.RLock()
	defer h.lock.RUnlock()
	res := make([]*ClusterMessage, len(h.messages))
	copy(res, h.messages)
	return res
}

func encodeClusterMessage(msg *ClusterMessage) ([]byte, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal message: %w", err)
	}
	buf := make([]byte, len(data)+1)
	buf[0] = clusterMsgPrefix
	copy(buf[1:], data)
	return buf, nil
}

func decodeClusterMessage(buf []byte) (*ClusterMessage, error) {
	if len(buf) < 2 || buf[0] != clusterMsgPrefix {
		return nil, fmt.Errorf("invalid cluster message header")
	}
	var msg ClusterMessage
	if err := json.Unmarshal(buf[1:], &msg); err != nil {
		return nil, fmt.Errorf("unmarshal message: %w", err)
	}
	return &msg, nil
}

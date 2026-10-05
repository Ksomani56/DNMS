package main

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents a structured YAML/JSON configuration for a DNMS daemon.
type Config struct {
	Cluster struct {
		NodeName   string `yaml:"node_name"`
		BindAddr   string `yaml:"bind_addr"`
		GossipAddr string `yaml:"gossip_addr"`
		GossipPort int    `yaml:"gossip_port"`
		Peer       string `yaml:"peer"`
		Aggregator bool   `yaml:"aggregator"`
	} `yaml:"cluster"`

	HTTP struct {
		Addr        string `yaml:"addr"`
		AllowOrigin string `yaml:"allow_origin"`
		Token       string `yaml:"token"`
	} `yaml:"http"`

	Mapper struct {
		Interval     time.Duration `yaml:"interval"`
		SrcPortStart int           `yaml:"src_port_start"`
		SrcPortEnd   int           `yaml:"src_port_end"`
	} `yaml:"mapper"`

	Pinger struct {
		PingPort      int           `yaml:"ping_port"`
		PeerInterval  time.Duration `yaml:"peer_interval"`
		RouteInterval time.Duration `yaml:"route_interval"`
		Timeout       time.Duration `yaml:"timeout"`
		RingSize      int           `yaml:"ring_size"`
	} `yaml:"pinger"`

	Alerting struct {
		WebhookURL    string  `yaml:"webhook_url"`
		LossThreshold float64 `yaml:"loss_threshold"`
	} `yaml:"alerting"`
}

// LoadConfig reads and parses a YAML configuration file.
func LoadConfig(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("unable to read config file %q: %w", filePath, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %w", err)
	}

	return &cfg, nil
}

// Package config handles loading and validating simulation configuration from JSON files.
package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config is the top-level simulation configuration.
type Config struct {
	Cluster   ClusterConfig   `json:"cluster"`
	Workload  WorkloadConfig  `json:"workload"`
	Scheduler string          `json:"scheduler"` // "fifo", "priority", "wfq", "srtf", "dynbatch"
	Tiers     []TierConfig    `json:"tiers"`
	Timing    TimingConfig    `json:"timing"`
	Sim       SimConfig       `json:"sim"`
}

// ClusterConfig defines the simulated cluster.
type ClusterConfig struct {
	NumWorkers     int     `json:"num_workers"`
	MemoryGB       float64 `json:"memory_gb"` // GPU memory per worker in GB
	MaxBatchSize   int     `json:"max_batch_size"`
	PagedAttention bool    `json:"paged_attention"` // Enable dynamic memory allocation
}

// WorkloadConfig defines how requests are generated.
type WorkloadConfig struct {
	Type            string             `json:"type"`             // "poisson", "bursty", "trace"
	RPS             float64            `json:"rps"`              // For poisson
	PeakRPS         float64            `json:"peak_rps"`         // For bursty
	BurstDurationMs float64            `json:"burst_duration_ms"`
	IdleDurationMs  float64            `json:"idle_duration_ms"`
	TraceFile       string             `json:"trace_file"`       // For trace
	DurationMs      float64            `json:"duration_ms"`      // Total simulation duration
	ContextTokens   TokenRange         `json:"context_tokens"`
	OutputTokens    TokenRange         `json:"output_tokens"`
	TierWeights     map[string]float64 `json:"tier_weights"` // tier name -> probability weight
	Seed            int64              `json:"seed"`
	PrefixHitRate   float64            `json:"prefix_hit_rate"` // Probability of a request sharing a prefix
}

// TokenRange defines a min/max range for token counts.
type TokenRange struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// TierConfig defines a user tier (e.g., free, pro, enterprise).
type TierConfig struct {
	Name         string  `json:"name"`
	Priority     int     `json:"priority"`      // Lower = higher priority
	Weight       float64 `json:"weight"`         // For WFQ scheduling
	SLATTFTMs    float64 `json:"sla_ttft_ms"`    // TTFT SLA threshold
	SLATotalMs   float64 `json:"sla_total_ms"`   // Total latency SLA threshold
}

// TimingConfig parameterizes the step-time model.
// step_ms = base_ms + per_token_ms * avg_seq_len + per_batch_ms * batch_size
type TimingConfig struct {
	BaseMs     float64 `json:"base_ms"`
	PerTokenMs float64 `json:"per_token_ms"`
	PerBatchMs float64 `json:"per_batch_ms"`
	KVPerTokenGB float64 `json:"kv_per_token_gb"` // KV cache memory per token in GB
}

// SimConfig holds general simulation parameters.
type SimConfig struct {
	SampleIntervalMs float64 `json:"sample_interval_ms"` // Periodic sampling interval
}

// Load reads and parses a JSON config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return &cfg, nil
}

// Validate checks the config for consistency.
func (c *Config) Validate() error {
	if c.Cluster.NumWorkers <= 0 {
		return fmt.Errorf("cluster.num_workers must be > 0")
	}
	if c.Cluster.MemoryGB <= 0 {
		return fmt.Errorf("cluster.memory_gb must be > 0")
	}
	if c.Cluster.MaxBatchSize <= 0 {
		return fmt.Errorf("cluster.max_batch_size must be > 0")
	}
	if c.Workload.DurationMs <= 0 {
		return fmt.Errorf("workload.duration_ms must be > 0")
	}
	if len(c.Tiers) == 0 {
		return fmt.Errorf("at least one tier must be defined")
	}
	if c.Timing.KVPerTokenGB <= 0 {
		return fmt.Errorf("timing.kv_per_token_gb must be > 0")
	}

	validSchedulers := map[string]bool{
		"fifo": true, "priority": true, "wfq": true, "srtf": true, "dynbatch": true,
	}
	if !validSchedulers[c.Scheduler] {
		return fmt.Errorf("unknown scheduler %q; valid: fifo, priority, wfq, srtf, dynbatch", c.Scheduler)
	}

	return nil
}

// TierByName returns the TierConfig with the given name, or nil if not found.
func (c *Config) TierByName(name string) *TierConfig {
	for i := range c.Tiers {
		if c.Tiers[i].Name == name {
			return &c.Tiers[i]
		}
	}
	return nil
}

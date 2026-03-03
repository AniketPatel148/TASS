// Package metrics collects and computes simulation performance metrics.
package metrics

import (
	"fmt"
	"strings"

	"github.com/aniketpatel/tass/internal/config"
	"github.com/aniketpatel/tass/internal/model"
	"github.com/aniketpatel/tass/internal/util"
)

// Collector accumulates per-request and system-level metrics during a simulation run.
type Collector struct {
	Completed []*model.Request // All completed requests
	Samples   []Sample         // Periodic system samples
	tiers     []config.TierConfig
}

// Sample is a periodic system snapshot.
type Sample struct {
	TimeMs        float64
	QueueDepth    int
	ActiveWorkers int
	TotalBatched  int
}

// NewCollector creates a new metrics collector.
func NewCollector(tiers []config.TierConfig) *Collector {
	return &Collector{
		tiers: tiers,
	}
}

// RecordCompletion records a completed request.
func (c *Collector) RecordCompletion(r *model.Request) {
	c.Completed = append(c.Completed, r)
}

// RecordSample records a periodic system snapshot.
func (c *Collector) RecordSample(timeMs float64, queueDepth, activeWorkers, totalBatched int) {
	c.Samples = append(c.Samples, Sample{
		TimeMs:        timeMs,
		QueueDepth:    queueDepth,
		ActiveWorkers: activeWorkers,
		TotalBatched:  totalBatched,
	})
}

// --- Computed metrics ---

// TierMetrics holds aggregated metrics for a single tier.
type TierMetrics struct {
	Tier         string
	Count        int
	AvgTTFTMs    float64
	P50TTFTMs    float64
	P95TTFTMs    float64
	P99TTFTMs    float64
	AvgLatencyMs float64
	P50LatencyMs float64
	P95LatencyMs float64
	P99LatencyMs float64
	AvgQueueMs   float64
	AvgTokPerSec float64
	SLATTFTViol  float64 // Fraction of requests violating TTFT SLA
	SLATotalViol float64 // Fraction of requests violating total latency SLA
}

// RunSummary holds the complete summary of a simulation run.
type RunSummary struct {
	SchedulerName    string
	TotalRequests    int
	TotalTokensGen   int
	SimDurationMs    float64
	ThroughputTokSec float64
	AvgUtilization   float64
	OverallP50Ms     float64
	OverallP95Ms     float64
	OverallP99Ms     float64
	FairnessIndex    float64
	TierMetrics      []TierMetrics
}

// ComputeSummary calculates the RunSummary from collected data.
func (c *Collector) ComputeSummary(schedulerName string, simDurationMs float64, workers []*model.Worker) RunSummary {
	summary := RunSummary{
		SchedulerName: schedulerName,
		TotalRequests: len(c.Completed),
		SimDurationMs: simDurationMs,
	}

	if len(c.Completed) == 0 {
		return summary
	}

	// Overall latencies
	var allLatencies []float64
	var totalTokens int
	for _, r := range c.Completed {
		allLatencies = append(allLatencies, r.TotalLatencyMs())
		totalTokens += r.OutputTokens
	}

	summary.TotalTokensGen = totalTokens
	summary.ThroughputTokSec = float64(totalTokens) / (simDurationMs / 1000.0)
	summary.OverallP50Ms = util.Percentile(allLatencies, 50)
	summary.OverallP95Ms = util.Percentile(allLatencies, 95)
	summary.OverallP99Ms = util.Percentile(allLatencies, 99)

	// Utilization
	var totalBusy float64
	for _, w := range workers {
		totalBusy += w.BusyTimeMs
	}
	summary.AvgUtilization = totalBusy / (simDurationMs * float64(len(workers)))

	// Per-tier metrics
	tierReqs := make(map[string][]*model.Request)
	for _, r := range c.Completed {
		tierReqs[string(r.Tier)] = append(tierReqs[string(r.Tier)], r)
	}

	var tierAvgLatencies []float64
	for _, tc := range c.tiers {
		reqs := tierReqs[tc.Name]
		tm := c.computeTierMetrics(tc, reqs)
		summary.TierMetrics = append(summary.TierMetrics, tm)
		if tm.Count > 0 {
			tierAvgLatencies = append(tierAvgLatencies, tm.AvgLatencyMs)
		}
	}

	// Jain's fairness index across tier average latencies
	// We use normalized throughput (tokens/sec per tier) for fairness
	var tierThroughputs []float64
	for _, tm := range summary.TierMetrics {
		if tm.Count > 0 {
			tierThroughputs = append(tierThroughputs, tm.AvgTokPerSec)
		}
	}
	summary.FairnessIndex = util.JainsIndex(tierThroughputs)

	return summary
}

func (c *Collector) computeTierMetrics(tc config.TierConfig, reqs []*model.Request) TierMetrics {
	tm := TierMetrics{Tier: tc.Name, Count: len(reqs)}
	if len(reqs) == 0 {
		return tm
	}

	var ttfts, latencies, queueDelays, tokPerSecs []float64
	var ttftViols, totalViols int

	for _, r := range reqs {
		ttft := r.TTFTMs()
		lat := r.TotalLatencyMs()
		qd := r.QueueDelayMs()
		tps := r.TokensPerSec()

		ttfts = append(ttfts, ttft)
		latencies = append(latencies, lat)
		queueDelays = append(queueDelays, qd)
		tokPerSecs = append(tokPerSecs, tps)

		if tc.SLATTFTMs > 0 && ttft > tc.SLATTFTMs {
			ttftViols++
		}
		if tc.SLATotalMs > 0 && lat > tc.SLATotalMs {
			totalViols++
		}
	}

	tm.AvgTTFTMs = util.Mean(ttfts)
	tm.P50TTFTMs = util.Percentile(ttfts, 50)
	tm.P95TTFTMs = util.Percentile(ttfts, 95)
	tm.P99TTFTMs = util.Percentile(ttfts, 99)
	tm.AvgLatencyMs = util.Mean(latencies)
	tm.P50LatencyMs = util.Percentile(latencies, 50)
	tm.P95LatencyMs = util.Percentile(latencies, 95)
	tm.P99LatencyMs = util.Percentile(latencies, 99)
	tm.AvgQueueMs = util.Mean(queueDelays)
	tm.AvgTokPerSec = util.Mean(tokPerSecs)
	tm.SLATTFTViol = float64(ttftViols) / float64(len(reqs))
	tm.SLATotalViol = float64(totalViols) / float64(len(reqs))

	return tm
}

// FormatTable returns a formatted text table of the run summary.
func (s *RunSummary) FormatTable() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("╔══════════════════════════════════════════════════════════════╗\n"))
	b.WriteString(fmt.Sprintf("║  Scheduler: %-47s ║\n", s.SchedulerName))
	b.WriteString(fmt.Sprintf("╠══════════════════════════════════════════════════════════════╣\n"))
	b.WriteString(fmt.Sprintf("║  Requests: %-6d  Tokens: %-8d  Duration: %.0fms %8s ║\n",
		s.TotalRequests, s.TotalTokensGen, s.SimDurationMs, ""))
	b.WriteString(fmt.Sprintf("║  Throughput: %.1f tok/s   Utilization: %.1f%%   Fairness: %.3f ║\n",
		s.ThroughputTokSec, s.AvgUtilization*100, s.FairnessIndex))
	b.WriteString(fmt.Sprintf("║  Latency P50: %.1fms  P95: %.1fms  P99: %.1fms %14s ║\n",
		s.OverallP50Ms, s.OverallP95Ms, s.OverallP99Ms, ""))
	b.WriteString(fmt.Sprintf("╠══════════════════════════════════════════════════════════════╣\n"))
	b.WriteString(fmt.Sprintf("║  %-12s %5s %8s %8s %8s %6s %6s   ║\n",
		"Tier", "Count", "TTFT95", "Lat95", "Lat99", "TTFT%V", "Lat%V"))
	b.WriteString(fmt.Sprintf("╠══════════════════════════════════════════════════════════════╣\n"))

	for _, tm := range s.TierMetrics {
		b.WriteString(fmt.Sprintf("║  %-12s %5d %7.1fms %7.1fms %7.1fms %5.1f%% %5.1f%%   ║\n",
			tm.Tier, tm.Count, tm.P95TTFTMs, tm.P95LatencyMs, tm.P99LatencyMs,
			tm.SLATTFTViol*100, tm.SLATotalViol*100))
	}
	b.WriteString(fmt.Sprintf("╚══════════════════════════════════════════════════════════════╝\n"))

	return b.String()
}

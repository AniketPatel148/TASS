// Package workload generates request arrival events for the simulation.
package workload

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/aniketpatel/tass/internal/config"
	"github.com/aniketpatel/tass/internal/model"
	"github.com/aniketpatel/tass/internal/util"
)

// Generate creates a slice of Requests based on the workload config.
func Generate(cfg *config.Config) ([]*model.Request, error) {
	rng := util.NewRNG(cfg.Workload.Seed)

	switch cfg.Workload.Type {
	case "poisson":
		return generatePoisson(cfg, rng), nil
	case "bursty":
		return generateBursty(cfg, rng), nil
	case "trace":
		return generateTrace(cfg)
	default:
		return nil, fmt.Errorf("unknown workload type: %s", cfg.Workload.Type)
	}
}

// generatePoisson creates arrivals with exponentially distributed inter-arrival times.
func generatePoisson(cfg *config.Config, rng *util.RNG) []*model.Request {
	var requests []*model.Request
	id := 0
	timeMs := 0.0

	for timeMs < cfg.Workload.DurationMs {
		// Exponential inter-arrival time
		interArrival := rng.Exponential(cfg.Workload.RPS/1000.0) // Convert RPS to per-ms rate
		timeMs += interArrival

		if timeMs >= cfg.Workload.DurationMs {
			break
		}

		r := makeRequest(id, timeMs, cfg, rng)
		requests = append(requests, r)
		id++
	}

	return requests
}

// generateBursty creates alternating burst/idle periods.
func generateBursty(cfg *config.Config, rng *util.RNG) []*model.Request {
	var requests []*model.Request
	id := 0
	timeMs := 0.0
	burstDur := cfg.Workload.BurstDurationMs
	idleDur := cfg.Workload.IdleDurationMs
	if burstDur <= 0 {
		burstDur = 2000 // default 2s burst
	}
	if idleDur <= 0 {
		idleDur = 3000 // default 3s idle
	}

	inBurst := true
	periodStart := 0.0

	for timeMs < cfg.Workload.DurationMs {
		if inBurst {
			// During burst: use peak RPS
			interArrival := rng.Exponential(cfg.Workload.PeakRPS / 1000.0)
			timeMs += interArrival

			if timeMs >= cfg.Workload.DurationMs {
				break
			}
			if timeMs-periodStart >= burstDur {
				inBurst = false
				periodStart = timeMs
				continue
			}

			r := makeRequest(id, timeMs, cfg, rng)
			requests = append(requests, r)
			id++
		} else {
			// Idle period: skip ahead
			timeMs = periodStart + idleDur
			periodStart = timeMs
			inBurst = true
		}
	}

	return requests
}

// generateTrace reads requests from a CSV file: arrival_ms,context_tokens,output_tokens,tier
func generateTrace(cfg *config.Config) ([]*model.Request, error) {
	f, err := os.Open(cfg.Workload.TraceFile)
	if err != nil {
		return nil, fmt.Errorf("opening trace file %s: %w", cfg.Workload.TraceFile, err)
	}
	defer f.Close()

	reader := csv.NewReader(f)

	// Skip header
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("reading trace header: %w", err)
	}
	_ = header

	var requests []*model.Request
	id := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading trace row %d: %w", id, err)
		}
		if len(record) < 4 {
			return nil, fmt.Errorf("trace row %d: expected 4 columns, got %d", id, len(record))
		}

		arrivalMs, err := strconv.ParseFloat(record[0], 64)
		if err != nil {
			return nil, fmt.Errorf("trace row %d: invalid arrival_ms: %w", id, err)
		}
		contextTokens, err := strconv.Atoi(record[1])
		if err != nil {
			return nil, fmt.Errorf("trace row %d: invalid context_tokens: %w", id, err)
		}
		outputTokens, err := strconv.Atoi(record[2])
		if err != nil {
			return nil, fmt.Errorf("trace row %d: invalid output_tokens: %w", id, err)
		}
		tier := model.Tier(record[3])

		requests = append(requests, &model.Request{
			ID:            id,
			Tier:          tier,
			ContextTokens: contextTokens,
			OutputTokens:  outputTokens,
			ArrivalMs:     arrivalMs,
			WorkerID:      -1,
		})
		id++
	}

	return requests, nil
}

// makeRequest creates a request with random token counts and tier assignment.
func makeRequest(id int, arrivalMs float64, cfg *config.Config, rng *util.RNG) *model.Request {
	contextTokens := rng.IntRange(cfg.Workload.ContextTokens.Min, cfg.Workload.ContextTokens.Max)
	outputTokens := rng.IntRange(cfg.Workload.OutputTokens.Min, cfg.Workload.OutputTokens.Max)

	tier := pickTier(cfg, rng)

	return &model.Request{
		ID:            id,
		Tier:          tier,
		ContextTokens: contextTokens,
		OutputTokens:  outputTokens,
		ArrivalMs:     arrivalMs,
		WorkerID:      -1,
	}
}

// pickTier selects a tier based on configured weights.
func pickTier(cfg *config.Config, rng *util.RNG) model.Tier {
	if len(cfg.Workload.TierWeights) == 0 {
		// Default: uniform across defined tiers
		idx := rng.Intn(len(cfg.Tiers))
		return model.Tier(cfg.Tiers[idx].Name)
	}

	// Weighted selection
	var totalWeight float64
	for _, w := range cfg.Workload.TierWeights {
		totalWeight += w
	}

	r := rng.Float64() * totalWeight
	var cumulative float64
	for name, w := range cfg.Workload.TierWeights {
		cumulative += w
		if r <= cumulative {
			return model.Tier(name)
		}
	}

	// Fallback
	return model.Tier(cfg.Tiers[0].Name)
}

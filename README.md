# TASS — Token-Aware Scheduling Simulator

A production-quality discrete-event simulator for multi-tenant LLM inference scheduling. Models token-by-token autoregressive decoding, KV-cache memory growth, bursty arrivals, user tiers, dynamic batching, and five scheduling policies.

## Quick Start

```bash
# Build
go build ./cmd/sim

# Run with default config (bursty stress scenario)
go run ./cmd/sim --config examples/config.json --out out/

# Compare all scheduling policies
go run ./cmd/sim --config examples/config.json --out out/ --compare

# Verbose mode (logs every event)
go run ./cmd/sim --config examples/config.json --out out/ --verbose
```

## Architecture

```
cmd/sim/main.go              CLI entry point
internal/
  config/config.go           JSON configuration loading + validation
  sim/event.go               Event types (Arrival, Dispatch, TokenStepDone, ...)
  sim/engine.go              Discrete-event engine with priority queue
  model/request.go           Request struct with scheduling state
  model/cluster.go           Worker + Cluster with KV-cache memory tracking
  model/kvcache.go           KV-cache memory model
  model/timing.go            Step-time model (calibration point for GPU benchmarks)
  workload/generator.go      Poisson, Bursty, Trace CSV generators
  scheduler/scheduler.go     Scheduler interface
  scheduler/fifo.go          FIFO scheduling
  scheduler/priority.go      Tier-based priority
  scheduler/wfq.go           Weighted Fair Queue
  scheduler/srtf.go          Shortest Remaining Tokens First
  scheduler/dynbatch.go      Latency-aware dynamic batching
  metrics/collector.go       Per-request + system metrics, fairness index
  metrics/exporter.go        CSV + JSON export
  util/heap.go               Generic priority queue
  util/stats.go              Percentile, Jain's index
  util/rng.go                Deterministic seeded RNG
docs/
  ARCHITECTURE.md            Event flow + design details
  POLICY_NOTES.md            Scheduling policy explanations + tradeoffs
examples/
  config.json                Bursty stress scenario config
  trace.csv                  Sample trace file
scripts/
  plot_results.py            Visualization script
```

## Configuration

See [`examples/config.json`](examples/config.json) for a complete example. Key sections:

| Section | Description |
|---------|-------------|
| `cluster` | Number of workers, GPU memory (GB), max batch size |
| `workload` | Arrival pattern (poisson/bursty/trace), RPS, token ranges, tier weights |
| `scheduler` | Policy name: `fifo`, `priority`, `wfq`, `srtf`, `dynbatch` |
| `tiers` | User tiers with priority, weight, and SLA thresholds |
| `timing` | Step-time model parameters (base_ms, per_token_ms, per_batch_ms, kv_per_token_gb) |

## Output

Single run produces:
- `summary.json` — Run summary with per-tier metrics
- `requests.csv` — Per-request latency breakdown (when using `--compare`, per-policy subdirectories)

## Interpreting Results

- **TTFT (Time to First Token)**: Measures responsiveness. Lower is better.
- **P95/P99 Latency**: Tail latency at the 95th and 99th percentiles.
- **SLA Violation %**: Fraction of requests exceeding the tier's SLA threshold.
- **Fairness Index**: Jain's fairness index across tiers (1.0 = perfectly fair).
- **Throughput (tok/s)**: Total tokens generated per second of simulation time.

## Extending: GPU Calibration

The timing model lives in [`internal/model/timing.go`](internal/model/timing.go). To calibrate with real GPU benchmarks:

1. Profile your GPU with varying batch sizes and sequence lengths
2. Fit the parameters: `base_ms`, `per_token_ms`, `per_batch_ms`
3. Update the config or subclass `TimingModel` with a non-linear model

## Running Tests

```bash
go test ./...
```

## License

MIT

# TASS — Token-Aware Scheduling Simulator

My personal sandbox for exploring the frontier of continuous batching and memory-aware scheduling. A production-quality discrete-event simulator modeling multi-tenant LLM inference, PagedAttention (with Recomputation Preemption), Prefix Caching, dynamic batching, and priority queuing.

## Quick Start

```bash
# Build
go build ./cmd/tass

# Run baseline FIFO scheduler vs PagedAttention/PrefixCaching simulator
go run ./cmd/tass --config experiments/s4_bursty/config.json --out out/baseline_srtf --verbose
go run ./cmd/tass --config experiments/s4_bursty/config_paged.json --out out/paged_srtf --verbose

# Plot memory-aware preemption results
python scripts/plot_results.py --dir out/ --compare
```

## 🌟 Core Features

- **PagedAttention & Preemption**: Simulates block-based memory allocation and evicts/recomputes requests on Out-Of-Memory (OOM) events based on a dynamic KV-cache model.
- **Prefix Caching**: Evaluates context-hit-rates for shared system prompts, accurately bypassing prefill computation metrics in continuous batching scenarios.
- **Advanced Schedulers**: Includes standard `FIFO`, Tier-based `Priority`, `WFQ` (Weighted Fair Queuing), `SRTF` (Shortest Remaining Tokens First), and dynamic tail-latency aware batching.
- **Bursty Traffic Generators**: Simulates spiky production traffic versus uniform Poisson arrivals.

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

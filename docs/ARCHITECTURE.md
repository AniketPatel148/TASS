# Architecture

## Overview

TASS (Token-Aware Scheduling Simulator) is a discrete-event simulator modeling multi-tenant LLM inference serving. It captures the key dynamics of autoregressive decoding: token-by-token generation, KV-cache memory growth, and the tension between throughput (large batches) and latency (small batches).

## Event-Driven Simulation

The core engine (`internal/sim/engine.go`) processes events from a min-heap priority queue ordered by simulation time. Events fire in strict chronological order; ties are broken FIFO by insertion sequence number.

### Event Types

| Event | Trigger | Handler |
|-------|---------|---------|
| **Arrival** | Workload generator schedules request | Enqueue to scheduler; try dispatch on idle workers |
| **Dispatch** | Worker becomes free | Form batch from scheduler queue; start decode loop |
| **TokenStepDone** | One decode step completes | Advance generated_tokens++; remove completed requests; fill batch slots; schedule next step |
| **Completion** | Request has all tokens generated | Record metrics (handled inline in TokenStepDone) |
| **PeriodicSample** | Timer fires | Snapshot queue depth, active workers, batch sizes |

### Event Flow

```
Arrival → Enqueue → [Worker Idle?] → FormBatch → AddToBatch
                                                      ↓
                                               TokenStepDone ←──┐
                                                      ↓         │
                                              gen_tokens++       │
                                              complete? ──No──→ next step
                                                  │
                                                 Yes → RecordCompletion
                                                       RemoveFromBatch
                                                       [Queue?] → FormBatch → ...
```

## Key Structs

### Request (`internal/model/request.go`)
Tracks the full lifecycle: arrival → enqueue → dispatch → first token → completion. Derived metrics (TTFT, queue delay, throughput) are computed from timestamps.

### Worker (`internal/model/cluster.go`)
Represents a GPU. Maintains a batch of active requests, tracks KV-cache memory usage, and enforces memory capacity. `CanFit()` checks worst-case memory (all output tokens) before admitting a request.

### TimingModel (`internal/model/timing.go`)
Single source of truth for step timing:
```
step_ms = base_ms + per_token_ms × avg_seq_len + per_batch_ms × batch_size
```
Designed for easy calibration with real GPU benchmarks — just update the three coefficients.

### KV Cache (`internal/model/kvcache.go`)
```
kv_gb = kv_per_token_gb × (context_tokens + generated_tokens)
```
Memory grows linearly as tokens are generated. Workers check feasibility before admitting requests.

## Workload Generation (`internal/workload/generator.go`)

Three modes:
1. **Poisson**: Memoryless arrivals at constant rate (exponential inter-arrival times)
2. **Bursty**: Alternating burst/idle periods. Burst uses `peak_rps`, idle skips time.
3. **Trace CSV**: Replay exact arrival times and token counts from a file.

Tier assignment uses weighted random selection from `tier_weights` in config.

## Scheduling (`internal/scheduler/`)

All schedulers implement the `Scheduler` interface: `Enqueue()`, `FormBatch()`, `QueueLen()`. `FormBatch()` receives a worker and must respect memory feasibility when selecting requests.

See [POLICY_NOTES.md](POLICY_NOTES.md) for detailed policy descriptions.

## Metrics (`internal/metrics/`)

**Per-request**: queue delay, TTFT, total latency, tokens/sec
**System**: throughput, utilization, per-tier percentiles (p50/p95/p99), SLA violation rates, Jain's fairness index across tier throughputs.

Export formats: per-request CSV and run summary JSON.

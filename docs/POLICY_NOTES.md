# Scheduling Policy Notes

## Overview

TASS implements five scheduling policies. All share the same interface and respect per-worker memory feasibility (KV-cache capacity) when forming batches. The choice of policy determines the **order** in which queued requests are selected and the **batch size** strategy.

---

## 1. FIFO (First-In, First-Out)

**File**: `internal/scheduler/fifo.go`

**Strategy**: Serve requests strictly in arrival order. Fill batch slots greedily from the head of the queue, skipping requests that don't fit memory.

**Tradeoffs**:
- ✅ Simple, predictable, no starvation
- ✅ Fair in a single-tier scenario
- ❌ No differentiation between tiers — enterprise requests wait behind free-tier bursts
- ❌ Long-context requests at the head can block shorter requests (head-of-line blocking)

**Best for**: Baseline comparison; single-tenant systems.

---

## 2. Tier Priority

**File**: `internal/scheduler/priority.go`

**Strategy**: Maintain a single queue sorted by `(tier_priority, arrival_time)`. Lower priority value = higher priority (enterprise=1, pro=2, free=3). Within a tier, FIFO order is preserved.

**Tradeoffs**:
- ✅ Enterprise requests get consistently low latency
- ✅ Simple to reason about and implement
- ❌ Free-tier starvation under sustained load — free requests only run when no higher-tier requests are queued
- ❌ Unfair Jain's index when load is skewed

**Best for**: Systems with strict tier SLAs where free-tier delay is acceptable.

---

## 3. Weighted Fair Queue (WFQ)

**File**: `internal/scheduler/wfq.go`

**Strategy**: Each tier has its own queue and a virtual time counter. The tier with the lowest virtual time is selected next. Virtual time advances by `1/weight` per request served, so higher-weight tiers get proportionally more service.

**Example**: If enterprise has weight 10 and free has weight 1, enterprise gets ~10× the scheduling opportunities.

**Tradeoffs**:
- ✅ Guarantees proportional fairness — every tier gets service proportional to its weight
- ✅ No starvation: even low-weight tiers progress
- ✅ Good Jain's fairness index
- ❌ Enterprise latency may be worse than strict priority under light load
- ❌ More complex state management (virtual time per tier)

**Best for**: Multi-tenant systems that need fairness guarantees without starvation.

---

## 4. Shortest Remaining Tokens First (SRTF)

**File**: `internal/scheduler/srtf.go`

**Strategy**: Sort queue by `remaining_tokens = output_tokens - generated_tokens` (ascending). Requests with fewer tokens to generate are served first. Ties broken by arrival time.

**Tradeoffs**:
- ✅ Minimizes average completion time (optimal for that metric in theory)
- ✅ Short requests finish quickly, avoiding head-of-line blocking
- ❌ Long-context, long-output requests suffer significant delays (starvation risk)
- ❌ No tier differentiation
- ❌ In practice, remaining tokens can only be estimated from `output_tokens`

**Best for**: Systems optimizing for average latency where request size varies widely.

---

## 5. Latency-Aware Dynamic Batching

**File**: `internal/scheduler/dynbatch.go`

**Strategy**: Adapts the effective batch size based on queue pressure:
- **Under pressure** (estimated queue wait > 50% of the **lowest** tier's TTFT SLA): halve the batch size to reduce step time and improve TTFT.
- **Low pressure**: use the full max batch size for throughput.

Requests are served FIFO within the dynamically-sized batch.

**Tradeoffs**:
- ✅ Automatically balances latency vs. throughput
- ✅ Responsive to load spikes — reduces batch size when queue builds up
- ✅ SLA-aware: uses TTFT targets to trigger adaptation
- ❌ Heuristic-based (the 50% threshold and queue-depth estimator are approximate)
- ❌ FIFO ordering within batch — no tier differentiation
- ❌ Throughput drops during high-pressure periods (by design)

**Best for**: Systems with bursty traffic and TTFT SLA requirements.

---

## Choosing a Policy

| Scenario | Recommended | Why |
|----------|-------------|-----|
| Single-tenant, predictable load | FIFO | Simple, fair, no overhead |
| Strict enterprise SLA | Priority | Guarantees lowest latency for top tier |
| Multi-tenant fairness required | WFQ | Proportional service, no starvation |
| Variable request sizes | SRTF | Minimizes avg latency |
| Bursty traffic + TTFT SLA | DynBatch | Adapts batch size to load |
| Unknown — run `--compare` | All | Use comparison mode to evaluate |

## Extending

To add a new policy:
1. Create `internal/scheduler/mypolicy.go`
2. Implement the `Scheduler` interface (Enqueue, FormBatch, QueueLen, Name)
3. Add a case in `cmd/sim/main.go` → `createScheduler()`
4. Add the name to `config.Validate()` in `internal/config/config.go`

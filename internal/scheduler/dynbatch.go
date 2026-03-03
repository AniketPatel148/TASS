package scheduler

import (
	"github.com/aniketpatel/tass/internal/config"
	"github.com/aniketpatel/tass/internal/model"
)

// DynBatchScheduler implements latency-aware dynamic batching.
// It adjusts the effective batch size based on queue depth and TTFT SLA targets.
// When the queue is deep and requests are waiting too long, it uses smaller batches
// to reduce step time and improve TTFT. When the queue is shallow, it batches more
// aggressively for throughput.
type DynBatchScheduler struct {
	queue    []*model.Request
	tiers    []config.TierConfig
	maxBatch int
}

func NewDynBatch(tiers []config.TierConfig, maxBatchSize int) *DynBatchScheduler {
	return &DynBatchScheduler{
		tiers:    tiers,
		maxBatch: maxBatchSize,
	}
}

func (s *DynBatchScheduler) Name() string { return "dynbatch" }

func (s *DynBatchScheduler) Enqueue(r *model.Request) {
	s.queue = append(s.queue, r)
}

func (s *DynBatchScheduler) FormBatch(w *model.Worker, kvPerTokenGB float64) []*model.Request {
	if len(s.queue) == 0 {
		return nil
	}

	// Dynamic batch size: shrink when queue is deep or TTFT pressure is high
	effectiveBatch := s.computeEffectiveBatchSize(w)

	var batch []*model.Request
	var remaining []*model.Request

	for _, r := range s.queue {
		if len(w.Batch)+len(batch) >= effectiveBatch {
			remaining = append(remaining, r)
			continue
		}
		worstTokens := r.ContextTokens + r.OutputTokens
		needed := float64(worstTokens) * kvPerTokenGB
		used := w.UsedMemoryGB(kvPerTokenGB)
		for _, br := range batch {
			used += float64(br.ContextTokens+br.OutputTokens) * kvPerTokenGB
		}
		avail := w.MemoryGB - used

		if needed <= avail {
			batch = append(batch, r)
		} else {
			remaining = append(remaining, r)
		}
	}

	s.queue = remaining
	return batch
}

// computeEffectiveBatchSize determines how large the batch should be.
// Strategy: reduce batch size when queue depth suggests latency pressure.
func (s *DynBatchScheduler) computeEffectiveBatchSize(w *model.Worker) int {
	qLen := len(s.queue)
	maxB := w.MaxBatchSize
	if s.maxBatch < maxB {
		maxB = s.maxBatch
	}

	// Check if any queued request is at risk of SLA violation
	hasPressure := false
	for _, r := range s.queue {
		// Estimate: if queue delay already exceeds 50% of the lowest TTFT SLA, there's pressure
		estimatedWait := 0.0
		if r.EnqueueMs > 0 {
			// Can't access clock here, so estimate from position in queue
			estimatedWait = float64(qLen) * 5.0 // rough heuristic: 5ms per queued request
		}
		for _, t := range s.tiers {
			if model.Tier(t.Name) == r.Tier && t.SLATTFTMs > 0 {
				if estimatedWait > t.SLATTFTMs*0.5 {
					hasPressure = true
					break
				}
			}
		}
		if hasPressure {
			break
		}
	}

	if hasPressure {
		// Reduce batch size to lower step time → faster TTFT
		reduced := maxB / 2
		if reduced < 1 {
			reduced = 1
		}
		return reduced
	}

	// Scale batch size with queue depth for throughput
	if qLen <= 2 {
		return min(qLen, maxB)
	}

	return maxB
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *DynBatchScheduler) QueueLen() int {
	return len(s.queue)
}

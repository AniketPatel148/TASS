package scheduler

import (
	"github.com/aniketpatel/tass/internal/model"
)

// FIFOScheduler serves requests in arrival order, respecting memory limits.
type FIFOScheduler struct {
	queue []*model.Request
}

func NewFIFO() *FIFOScheduler {
	return &FIFOScheduler{}
}

func (s *FIFOScheduler) Name() string { return "fifo" }

func (s *FIFOScheduler) Enqueue(r *model.Request) {
	s.queue = append(s.queue, r)
}

func (s *FIFOScheduler) FormBatch(w *model.Worker, kvPerTokenGB float64) []*model.Request {
	var batch []*model.Request
	var remaining []*model.Request

	for _, r := range s.queue {
		if len(w.Batch)+len(batch) >= w.MaxBatchSize {
			remaining = append(remaining, r)
			continue
		}
		// Check memory feasibility (worst-case: context + all output tokens)
		worstTokens := r.ContextTokens + r.OutputTokens
		needed := float64(worstTokens) * kvPerTokenGB

		// Calculate available memory after batch additions
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

func (s *FIFOScheduler) QueueLen() int {
	return len(s.queue)
}

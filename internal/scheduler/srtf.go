package scheduler

import (
	"sort"

	"github.com/aniketpatel/tass/internal/model"
)

// SRTFScheduler implements Shortest Remaining Tokens First.
// Approximates SRTF using expected remaining output tokens.
// Reduces head-of-line blocking by shorter requests.
type SRTFScheduler struct {
	queue []*model.Request
}

func NewSRTF() *SRTFScheduler {
	return &SRTFScheduler{}
}

func (s *SRTFScheduler) Name() string { return "srtf" }

func (s *SRTFScheduler) Enqueue(r *model.Request) {
	s.queue = append(s.queue, r)
	// Sort by remaining tokens (ascending), then arrival time for ties
	sort.SliceStable(s.queue, func(i, j int) bool {
		ri := s.queue[i].RemainingTokens()
		rj := s.queue[j].RemainingTokens()
		if ri != rj {
			return ri < rj
		}
		return s.queue[i].ArrivalMs < s.queue[j].ArrivalMs
	})
}

func (s *SRTFScheduler) FormBatch(w *model.Worker, kvPerTokenGB float64) []*model.Request {
	var batch []*model.Request
	var remaining []*model.Request

	for _, r := range s.queue {
		if len(w.Batch)+len(batch) >= w.MaxBatchSize {
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

func (s *SRTFScheduler) QueueLen() int {
	return len(s.queue)
}

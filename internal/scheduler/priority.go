package scheduler

import (
	"sort"

	"github.com/aniketpatel/tass/internal/config"
	"github.com/aniketpatel/tass/internal/model"
)

// PriorityScheduler serves requests ordered by tier priority (lower priority value = higher priority).
// Within the same tier, FIFO order is maintained.
type PriorityScheduler struct {
	queue    []*model.Request
	tierPrio map[model.Tier]int // tier name -> priority (lower = higher)
}

func NewPriority(tiers []config.TierConfig) *PriorityScheduler {
	prio := make(map[model.Tier]int)
	for _, t := range tiers {
		prio[model.Tier(t.Name)] = t.Priority
	}
	return &PriorityScheduler{
		tierPrio: prio,
	}
}

func (s *PriorityScheduler) Name() string { return "priority" }

func (s *PriorityScheduler) Enqueue(r *model.Request) {
	s.queue = append(s.queue, r)
	// Sort by tier priority, then by arrival time (FIFO within tier)
	sort.SliceStable(s.queue, func(i, j int) bool {
		pi := s.tierPrio[s.queue[i].Tier]
		pj := s.tierPrio[s.queue[j].Tier]
		if pi != pj {
			return pi < pj
		}
		return s.queue[i].ArrivalMs < s.queue[j].ArrivalMs
	})
}

func (s *PriorityScheduler) FormBatch(w *model.Worker, kvPerTokenGB float64) []*model.Request {
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

func (s *PriorityScheduler) QueueLen() int {
	return len(s.queue)
}

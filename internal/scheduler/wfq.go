package scheduler

import (
	"github.com/aniketpatel/tass/internal/config"
	"github.com/aniketpatel/tass/internal/model"
)

// WFQScheduler implements Weighted Fair Queueing across tiers.
// Each tier has a virtual time counter; the tier with the lowest virtual time
// is selected next. Virtual time advances inversely proportional to weight.
type WFQScheduler struct {
	tierQueues  map[model.Tier][]*model.Request
	tierWeights map[model.Tier]float64
	virtualTime map[model.Tier]float64
	tierOrder   []model.Tier // Deterministic iteration order
}

func NewWFQ(tiers []config.TierConfig) *WFQScheduler {
	queues := make(map[model.Tier][]*model.Request)
	weights := make(map[model.Tier]float64)
	vt := make(map[model.Tier]float64)
	var order []model.Tier

	for _, t := range tiers {
		tier := model.Tier(t.Name)
		queues[tier] = nil
		w := t.Weight
		if w <= 0 {
			w = 1.0
		}
		weights[tier] = w
		vt[tier] = 0
		order = append(order, tier)
	}

	return &WFQScheduler{
		tierQueues:  queues,
		tierWeights: weights,
		virtualTime: vt,
		tierOrder:   order,
	}
}

func (s *WFQScheduler) Name() string { return "wfq" }

func (s *WFQScheduler) Enqueue(r *model.Request) {
	s.tierQueues[r.Tier] = append(s.tierQueues[r.Tier], r)
}

func (s *WFQScheduler) FormBatch(w *model.Worker, kvPerTokenGB float64) []*model.Request {
	var batch []*model.Request

	// Keep picking from the tier with the lowest virtual time until batch is full.
	for len(w.Batch)+len(batch) < w.MaxBatchSize {
		tier, ok := s.pickLowestVTTier()
		if !ok {
			break
		}

		// Try to take one request from this tier
		queue := s.tierQueues[tier]
		picked := false
		var remaining []*model.Request
		for _, r := range queue {
			if picked {
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
				// Advance virtual time inversely by weight
				s.virtualTime[tier] += 1.0 / s.tierWeights[tier]
				picked = true
			} else {
				remaining = append(remaining, r)
			}
		}
		s.tierQueues[tier] = remaining

		if !picked {
			// This tier can't fit anything, skip it for this batch
			// Temporarily set high VT to avoid infinite loop
			s.virtualTime[tier] += 1e9
			// We'll reset it later only if queue is empty
			if len(s.tierQueues[tier]) == 0 {
				s.virtualTime[tier] = 0
			}
			// Check if all tiers are exhausted
			if s.QueueLen() == 0 {
				break
			}
		}
	}

	return batch
}

// pickLowestVTTier returns the tier with the lowest virtual time that has pending requests.
func (s *WFQScheduler) pickLowestVTTier() (model.Tier, bool) {
	bestVT := 1e18
	var bestTier model.Tier
	found := false

	for _, tier := range s.tierOrder {
		if len(s.tierQueues[tier]) > 0 && s.virtualTime[tier] < bestVT {
			bestVT = s.virtualTime[tier]
			bestTier = tier
			found = true
		}
	}

	return bestTier, found
}

func (s *WFQScheduler) QueueLen() int {
	total := 0
	for _, q := range s.tierQueues {
		total += len(q)
	}
	return total
}

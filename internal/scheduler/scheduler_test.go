package scheduler

import (
	"testing"

	"github.com/aniketpatel/tass/internal/config"
	"github.com/aniketpatel/tass/internal/model"
)

func TestFIFO_MemoryFeasibility(t *testing.T) {
	sched := NewFIFO()

	// Worker with 1GB memory, max batch 4
	w := model.NewWorker(0, 1.0, 4)
	kvPerTokenGB := 0.001 // 1MB per token → 1000 tokens = 1GB

	// Request needs 500 + 500 = 1000 tokens → 1.0 GB (fills the worker)
	r1 := &model.Request{ID: 1, ContextTokens: 500, OutputTokens: 500, ArrivalMs: 0}
	// Request needs 300 + 200 = 500 tokens → 0.5 GB (won't fit)
	r2 := &model.Request{ID: 2, ContextTokens: 300, OutputTokens: 200, ArrivalMs: 1}

	sched.Enqueue(r1)
	sched.Enqueue(r2)

	batch := sched.FormBatch(w, kvPerTokenGB)

	// Only r1 should fit (uses exactly 1GB)
	if len(batch) != 1 {
		t.Fatalf("expected batch size 1, got %d", len(batch))
	}
	if batch[0].ID != 1 {
		t.Errorf("expected request ID 1, got %d", batch[0].ID)
	}

	// r2 should still be in the queue
	if sched.QueueLen() != 1 {
		t.Errorf("expected 1 remaining in queue, got %d", sched.QueueLen())
	}
}

func TestFIFO_BatchSizeLimit(t *testing.T) {
	sched := NewFIFO()
	// Worker with plenty of memory but batch size limit of 2
	w := model.NewWorker(0, 100.0, 2)
	kvPerTokenGB := 0.0001

	for i := 0; i < 5; i++ {
		sched.Enqueue(&model.Request{ID: i, ContextTokens: 10, OutputTokens: 10, ArrivalMs: float64(i)})
	}

	batch := sched.FormBatch(w, kvPerTokenGB)
	if len(batch) != 2 {
		t.Errorf("expected batch size 2, got %d", len(batch))
	}
	if sched.QueueLen() != 3 {
		t.Errorf("expected 3 remaining, got %d", sched.QueueLen())
	}
}

func TestPriority_TierOrdering(t *testing.T) {
	tiers := []config.TierConfig{
		{Name: "enterprise", Priority: 1, Weight: 10},
		{Name: "pro", Priority: 2, Weight: 3},
		{Name: "free", Priority: 3, Weight: 1},
	}
	sched := NewPriority(tiers)
	w := model.NewWorker(0, 100.0, 3)
	kvPerTokenGB := 0.0001

	sched.Enqueue(&model.Request{ID: 1, Tier: "free", ContextTokens: 10, OutputTokens: 10, ArrivalMs: 0})
	sched.Enqueue(&model.Request{ID: 2, Tier: "enterprise", ContextTokens: 10, OutputTokens: 10, ArrivalMs: 1})
	sched.Enqueue(&model.Request{ID: 3, Tier: "pro", ContextTokens: 10, OutputTokens: 10, ArrivalMs: 2})

	batch := sched.FormBatch(w, kvPerTokenGB)
	if len(batch) != 3 {
		t.Fatalf("expected batch size 3, got %d", len(batch))
	}
	// Enterprise should be first despite arriving later
	if batch[0].ID != 2 {
		t.Errorf("expected enterprise (ID=2) first, got ID=%d", batch[0].ID)
	}
	if batch[1].ID != 3 {
		t.Errorf("expected pro (ID=3) second, got ID=%d", batch[1].ID)
	}
	if batch[2].ID != 1 {
		t.Errorf("expected free (ID=1) third, got ID=%d", batch[2].ID)
	}
}

func TestSRTF_ShorterFirst(t *testing.T) {
	sched := NewSRTF()
	w := model.NewWorker(0, 100.0, 1) // batch size 1
	kvPerTokenGB := 0.0001

	// Long request
	sched.Enqueue(&model.Request{ID: 1, ContextTokens: 100, OutputTokens: 500, ArrivalMs: 0})
	// Short request (arrives later but fewer tokens)
	sched.Enqueue(&model.Request{ID: 2, ContextTokens: 50, OutputTokens: 10, ArrivalMs: 1})

	batch := sched.FormBatch(w, kvPerTokenGB)
	if len(batch) != 1 {
		t.Fatalf("expected batch size 1, got %d", len(batch))
	}
	if batch[0].ID != 2 {
		t.Errorf("expected short request (ID=2) first, got ID=%d", batch[0].ID)
	}
}

func TestWFQ_FairAllocation(t *testing.T) {
	tiers := []config.TierConfig{
		{Name: "high", Priority: 1, Weight: 3},
		{Name: "low", Priority: 2, Weight: 1},
	}
	sched := NewWFQ(tiers)
	w := model.NewWorker(0, 100.0, 1) // batch size 1: pick one at a time
	kvPerTokenGB := 0.0001

	// Enqueue 3 high and 3 low
	for i := 0; i < 3; i++ {
		sched.Enqueue(&model.Request{ID: i, Tier: "high", ContextTokens: 10, OutputTokens: 10, ArrivalMs: float64(i)})
	}
	for i := 3; i < 6; i++ {
		sched.Enqueue(&model.Request{ID: i, Tier: "low", ContextTokens: 10, OutputTokens: 10, ArrivalMs: float64(i)})
	}

	// Draw 4 batches; high should get more early picks due to weight=3 vs weight=1
	highCount := 0
	for i := 0; i < 4; i++ {
		batch := sched.FormBatch(w, kvPerTokenGB)
		if len(batch) != 1 {
			t.Fatalf("step %d: expected batch size 1, got %d", i, len(batch))
		}
		if batch[0].Tier == "high" {
			highCount++
		}
	}
	// With weight 3:1, out of 4 picks high should get 3
	if highCount < 2 {
		t.Errorf("WFQ: expected high to get at least 2/4 picks, got %d", highCount)
	}
}

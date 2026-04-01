package model

// PrefixState tracks the size and references of a cached prefix
type PrefixState struct {
	Tokens int
	Refs   int
}

// Worker represents a single GPU worker that processes a batch of requests.
type Worker struct {
	ID             int
	MemoryGB       float64                 // Total GPU memory available
	MaxBatchSize   int                     // Maximum batch size
	Batch          []*Request              // Currently active batch
	PrefixCache    map[string]*PrefixState // Cache for shared prefixes
	PagedAttention bool                    // If true, use dynamic memory admission
	BusyUntilMs    float64                 // Time until the current step completes
	BusyTimeMs     float64                 // Cumulative busy time (for utilization calc)
}

// NewWorker creates a worker with the given capacity.
func NewWorker(id int, memGB float64, maxBatch int, pagedAttention bool) *Worker {
	return &Worker{
		ID:             id,
		MemoryGB:       memGB,
		MaxBatchSize:   maxBatch,
		Batch:          make([]*Request, 0),
		PrefixCache:    make(map[string]*PrefixState),
		PagedAttention: pagedAttention,
	}
}

// UsedMemoryGB returns the total KV-cache memory used by the current batch.
func (w *Worker) UsedMemoryGB(kvPerTokenGB float64) float64 {
	var total float64
	for _, r := range w.Batch {
		total += KVCacheGB(r, kvPerTokenGB)
	}
	for _, pc := range w.PrefixCache {
		total += float64(pc.Tokens) * kvPerTokenGB
	}
	return total
}

// AvailableMemoryGB returns remaining GPU memory.
func (w *Worker) AvailableMemoryGB(kvPerTokenGB float64) float64 {
	return w.MemoryGB - w.UsedMemoryGB(kvPerTokenGB)
}

// CanFit checks whether a request can fit in the worker's memory, considering
// future token generation (worst-case: all output tokens generated).
func (w *Worker) CanFit(r *Request, kvPerTokenGB float64) bool {
	if len(w.Batch) >= w.MaxBatchSize {
		return false
	}
	
	// Check if prefix is cached (simulate a hit to compute needed memory)
	matchedPrefixTokens := 0
	if r.PrefixID != "" {
		if pc, exists := w.PrefixCache[r.PrefixID]; exists {
			matchedPrefixTokens = pc.Tokens
		}
	}

	if w.PagedAttention {
		// Only need memory for context tokens (minus prefix hit) + 1 token for safety
		needed := float64(r.ContextTokens-matchedPrefixTokens+1) * kvPerTokenGB
		return w.AvailableMemoryGB(kvPerTokenGB) >= needed
	}

	// Worst-case memory: context + all output tokens
	worstCaseTokens := r.ContextTokens + r.OutputTokens - matchedPrefixTokens
	needed := float64(worstCaseTokens) * kvPerTokenGB
	return w.AvailableMemoryGB(kvPerTokenGB) >= needed
}

// IsBusy returns true if the worker is currently processing a batch.
func (w *Worker) IsBusy(nowMs float64) bool {
	return nowMs < w.BusyUntilMs
}

// IsIdle returns true if the worker has no active batch.
func (w *Worker) IsIdle() bool {
	return len(w.Batch) == 0
}

// AddToBatch adds a request to the worker's batch.
func (w *Worker) AddToBatch(r *Request) {
	if r.PrefixID != "" {
		if pc, exists := w.PrefixCache[r.PrefixID]; exists {
			r.MatchedPrefixTokens = pc.Tokens
			pc.Refs++
		} else {
			r.MatchedPrefixTokens = 0
			w.PrefixCache[r.PrefixID] = &PrefixState{Tokens: r.ContextTokens, Refs: 1}
		}
	}
	r.IsActive = true
	r.WorkerID = w.ID
	w.Batch = append(w.Batch, r)
}

// removeRef decrements the ref counter for a cached prefix, deleting it if 0.
func (w *Worker) removeRef(r *Request) {
	if r.PrefixID != "" {
		if pc, exists := w.PrefixCache[r.PrefixID]; exists {
			pc.Refs--
			if pc.Refs <= 0 {
				delete(w.PrefixCache, r.PrefixID)
			}
		}
	}
}

// RemoveCompleted removes completed requests from the batch and returns them.
func (w *Worker) RemoveCompleted() []*Request {
	var completed []*Request
	active := w.Batch[:0]
	for _, r := range w.Batch {
		if r.IsComplete() {
			r.IsActive = false
			r.WorkerID = -1
			w.removeRef(r)
			completed = append(completed, r)
		} else {
			active = append(active, r)
		}
	}
	w.Batch = active
	return completed
}

// Evict forcefully removes a request from the current batch for preemption.
func (w *Worker) Evict(evictedReq *Request) {
	active := w.Batch[:0]
	for _, r := range w.Batch {
		if r.ID == evictedReq.ID {
			r.IsActive = false
			r.WorkerID = -1
			w.removeRef(r)
			// Re-computation eviction strategy: lose all generated tokens
			r.GeneratedTokens = 0
			// Note: We don't clear r.FirstTokenMs here so we can see the severe TTFT penalty, 
			// or we could clear it. For realism, preemption blows away the first token SLA if 
			// it recomputes it, so we leave FirstTokenMs alone and update it next time.
			r.FirstTokenMs = 0
		} else {
			active = append(active, r)
		}
	}
	w.Batch = active
}

// BatchAvgSeqLen returns the average total sequence length across the batch.
func (w *Worker) BatchAvgSeqLen() float64 {
	if len(w.Batch) == 0 {
		return 0
	}
	var total int
	for _, r := range w.Batch {
		total += r.TotalTokens()
	}
	return float64(total) / float64(len(w.Batch))
}

// Cluster represents the simulated GPU cluster.
type Cluster struct {
	Workers []*Worker
}

// NewCluster creates a cluster with the given number of identical workers.
func NewCluster(numWorkers int, memGB float64, maxBatch int, pagedAttention bool) *Cluster {
	workers := make([]*Worker, numWorkers)
	for i := range workers {
		workers[i] = NewWorker(i, memGB, maxBatch, pagedAttention)
	}
	return &Cluster{Workers: workers}
}

// IdleWorkers returns all workers that currently have no active batch.
func (c *Cluster) IdleWorkers() []*Worker {
	var idle []*Worker
	for _, w := range c.Workers {
		if w.IsIdle() {
			idle = append(idle, w)
		}
	}
	return idle
}

// WorkersReadyAt returns all workers whose current step is done at or before nowMs.
func (c *Cluster) WorkersReadyAt(nowMs float64) []*Worker {
	var ready []*Worker
	for _, w := range c.Workers {
		if !w.IsBusy(nowMs) {
			ready = append(ready, w)
		}
	}
	return ready
}

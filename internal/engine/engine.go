package engine

import (
	"fmt"
	"log"

	"github.com/aniketpatel/tass/internal/config"
	"github.com/aniketpatel/tass/internal/metrics"
	"github.com/aniketpatel/tass/internal/model"
	"github.com/aniketpatel/tass/internal/scheduler"
	"github.com/aniketpatel/tass/internal/util"
)

// Engine is the discrete-event simulation engine.
type Engine struct {
	cfg       *config.Config
	clock     float64                   // Current simulation time in ms
	eventQ    *util.PriorityQueue[*Event]
	seqNum    int64                     // Monotonic event sequence counter
	Cluster   *model.Cluster
	timing    *model.TimingModel
	sched     scheduler.Scheduler
	collector *metrics.Collector
	requests  map[int]*model.Request    // All requests by ID
	completed int                       // Number of completed requests
	verbose   bool
}

// NewEngine creates a simulation engine from config and scheduler.
func NewEngine(cfg *config.Config, sched scheduler.Scheduler, collector *metrics.Collector) *Engine {
	cluster := model.NewCluster(cfg.Cluster.NumWorkers, cfg.Cluster.MemoryGB, cfg.Cluster.MaxBatchSize)
	timing := model.NewTimingModel(cfg.Timing.BaseMs, cfg.Timing.PerTokenMs, cfg.Timing.PerBatchMs)

	return &Engine{
		cfg:       cfg,
		eventQ:    util.NewPriorityQueue[*Event](),
		Cluster:   cluster,
		timing:    timing,
		sched:     sched,
		collector: collector,
		requests:  make(map[int]*model.Request),
	}
}

// SetVerbose enables verbose logging of events.
func (e *Engine) SetVerbose(v bool) {
	e.verbose = v
}

// ScheduleEvent adds an event to the event queue.
func (e *Engine) ScheduleEvent(ev *Event) {
	ev.SeqNum = e.seqNum
	e.seqNum++
	e.eventQ.Push(ev)
}

// ScheduleArrival schedules a request arrival event.
func (e *Engine) ScheduleArrival(r *model.Request) {
	e.requests[r.ID] = r
	e.ScheduleEvent(&Event{
		TimeMs:    r.ArrivalMs,
		Type:      EventArrival,
		RequestID: r.ID,
		WorkerID:  -1,
	})
}

// Run executes the simulation until all events are processed.
func (e *Engine) Run() {
	// Schedule periodic sampling
	if e.cfg.Sim.SampleIntervalMs > 0 {
		e.ScheduleEvent(&Event{
			TimeMs:    e.cfg.Sim.SampleIntervalMs,
			Type:      EventPeriodicSample,
			RequestID: -1,
			WorkerID:  -1,
		})
	}

	for !e.eventQ.IsEmpty() {
		ev := e.eventQ.Pop()
		e.clock = ev.TimeMs

		if e.verbose {
			log.Printf("[%.2fms] %s req=%d worker=%d", ev.TimeMs, ev.Type, ev.RequestID, ev.WorkerID)
		}

		switch ev.Type {
		case EventArrival:
			e.handleArrival(ev)
		case EventDispatch:
			e.handleDispatch(ev)
		case EventTokenStepDone:
			e.handleTokenStepDone(ev)
		case EventCompletion:
			e.handleCompletion(ev)
		case EventPeriodicSample:
			e.handlePeriodicSample(ev)
		}
	}
}

// Clock returns the current simulation time.
func (e *Engine) Clock() float64 { return e.clock }

// CompletedCount returns the number of completed requests.
func (e *Engine) CompletedCount() int { return e.completed }

// handleArrival enqueues a request and tries to dispatch.
func (e *Engine) handleArrival(ev *Event) {
	r := e.requests[ev.RequestID]
	r.EnqueueMs = e.clock
	e.sched.Enqueue(r)

	// Try to dispatch on any idle workers
	e.tryDispatchAll()
}

// handleDispatch is triggered when a worker becomes free. Try to fill it.
func (e *Engine) handleDispatch(ev *Event) {
	e.tryDispatch(e.Cluster.Workers[ev.WorkerID])
}

// handleTokenStepDone processes the completion of one decode step for a worker's batch.
func (e *Engine) handleTokenStepDone(ev *Event) {
	w := e.Cluster.Workers[ev.WorkerID]

	// Advance each request by one generated token
	for _, r := range w.Batch {
		r.GeneratedTokens++
		// Mark first token time
		if r.GeneratedTokens == 1 {
			r.FirstTokenMs = e.clock
		}
	}

	// Remove completed requests
	completed := w.RemoveCompleted()
	for _, r := range completed {
		r.CompletionMs = e.clock
		e.completed++
		e.collector.RecordCompletion(r)
		if e.verbose {
			log.Printf("[%.2fms] Completed req=%d latency=%.2fms ttft=%.2fms",
				e.clock, r.ID, r.TotalLatencyMs(), r.TTFTMs())
		}
	}

	// If there are still active requests in the batch, schedule next step
	if len(w.Batch) > 0 {
		// Try to fill empty slots in the batch with waiting requests
		e.tryFillBatch(w)
		e.scheduleTokenStep(w)
	} else {
		// Worker is idle, try to form a new batch
		e.tryDispatch(w)
	}
}

// handleCompletion is unused directly; completions are handled in handleTokenStepDone.
func (e *Engine) handleCompletion(ev *Event) {
	// Completions are handled inline in handleTokenStepDone.
}

// handlePeriodicSample collects a metrics snapshot and schedules the next one.
func (e *Engine) handlePeriodicSample(ev *Event) {
	activeBatches := 0
	totalBatchSize := 0
	for _, w := range e.Cluster.Workers {
		if len(w.Batch) > 0 {
			activeBatches++
			totalBatchSize += len(w.Batch)
		}
	}
	e.collector.RecordSample(e.clock, e.sched.QueueLen(), activeBatches, totalBatchSize)

	// Schedule next sample if within duration
	nextTime := e.clock + e.cfg.Sim.SampleIntervalMs
	if nextTime <= e.cfg.Workload.DurationMs*1.5 { // Allow samples a bit after workload ends
		e.ScheduleEvent(&Event{
			TimeMs:    nextTime,
			Type:      EventPeriodicSample,
			RequestID: -1,
			WorkerID:  -1,
		})
	}
}

// tryDispatchAll attempts to dispatch batches to all idle workers.
func (e *Engine) tryDispatchAll() {
	for _, w := range e.Cluster.Workers {
		if w.IsIdle() && e.sched.QueueLen() > 0 {
			e.tryDispatch(w)
		}
	}
}

// tryDispatch forms a batch for a worker and starts processing.
func (e *Engine) tryDispatch(w *model.Worker) {
	if e.sched.QueueLen() == 0 {
		return
	}

	batch := e.sched.FormBatch(w, e.cfg.Timing.KVPerTokenGB)
	if len(batch) == 0 {
		return
	}

	for _, r := range batch {
		r.DispatchMs = e.clock
		w.AddToBatch(r)
	}

	e.scheduleTokenStep(w)
}

// tryFillBatch fills empty slots in a running worker's batch.
func (e *Engine) tryFillBatch(w *model.Worker) {
	if e.sched.QueueLen() == 0 {
		return
	}

	fillBatch := e.sched.FormBatch(w, e.cfg.Timing.KVPerTokenGB)
	for _, r := range fillBatch {
		r.DispatchMs = e.clock
		w.AddToBatch(r)
	}
}

// scheduleTokenStep computes the step time and schedules a TokenStepDone event.
func (e *Engine) scheduleTokenStep(w *model.Worker) {
	stepMs := e.timing.StepMs(len(w.Batch), w.BatchAvgSeqLen())
	w.BusyUntilMs = e.clock + stepMs
	w.BusyTimeMs += stepMs

	e.ScheduleEvent(&Event{
		TimeMs:   e.clock + stepMs,
		Type:     EventTokenStepDone,
		WorkerID: w.ID,
	})
}

// Summary returns a text summary of the simulation run.
func (e *Engine) Summary() string {
	return fmt.Sprintf("Scheduler: %s | Completed: %d | SimTime: %.2fms",
		e.sched.Name(), e.completed, e.clock)
}

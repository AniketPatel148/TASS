// Package scheduler defines the Scheduler interface and scheduling policies.
package scheduler

import (
	"github.com/aniketpatel/tass/internal/model"
)

// Scheduler is the interface that all scheduling policies must implement.
type Scheduler interface {
	// Name returns the human-readable name of this scheduling policy.
	Name() string

	// Enqueue adds a request to the scheduler's internal queue(s).
	Enqueue(r *model.Request)

	// FormBatch selects requests from the queue to form a batch for the given worker,
	// respecting the worker's memory capacity (using kvPerTokenGB for KV-cache sizing).
	// Returns the selected requests (removed from queue). May return empty if nothing fits.
	FormBatch(w *model.Worker, kvPerTokenGB float64) []*model.Request

	// QueueLen returns the total number of requests waiting across all internal queues.
	QueueLen() int
}

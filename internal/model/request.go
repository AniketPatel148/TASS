// Package model defines the core domain types for the LLM inference simulator.
package model

// Tier represents a user tier name.
type Tier string

const (
	TierFree       Tier = "free"
	TierPro        Tier = "pro"
	TierEnterprise Tier = "enterprise"
)

// Request represents a single LLM inference request in the simulation.
type Request struct {
	ID            int     // Unique request ID
	Tier          Tier    // User tier
	ContextTokens int     // Number of input/context tokens (prefill)
	OutputTokens  int     // Total number of output tokens to generate
	ArrivalMs     float64 // Arrival time in simulation milliseconds

	// --- Scheduling state (mutated during simulation) ---

	EnqueueMs       float64 // Time the request was enqueued to a scheduler
	DispatchMs      float64 // Time the request was first dispatched to a worker (TTFT anchor)
	FirstTokenMs    float64 // Time first token was produced (TTFT end)
	CompletionMs    float64 // Time the request completed all token generation

	GeneratedTokens int     // Number of output tokens generated so far
	IsActive        bool    // Currently being processed by a worker
	WorkerID        int     // ID of the worker processing this request (-1 if not assigned)
}

// TotalTokens returns the total token count (context + generated so far).
func (r *Request) TotalTokens() int {
	return r.ContextTokens + r.GeneratedTokens
}

// RemainingTokens returns the number of output tokens left to generate.
func (r *Request) RemainingTokens() int {
	return r.OutputTokens - r.GeneratedTokens
}

// IsComplete returns true if all output tokens have been generated.
func (r *Request) IsComplete() bool {
	return r.GeneratedTokens >= r.OutputTokens
}

// QueueDelayMs returns the time spent waiting in the scheduler queue.
func (r *Request) QueueDelayMs() float64 {
	if r.DispatchMs <= 0 {
		return 0
	}
	return r.DispatchMs - r.ArrivalMs
}

// TTFTMs returns the time to first token.
func (r *Request) TTFTMs() float64 {
	if r.FirstTokenMs <= 0 {
		return 0
	}
	return r.FirstTokenMs - r.ArrivalMs
}

// TotalLatencyMs returns the end-to-end latency.
func (r *Request) TotalLatencyMs() float64 {
	if r.CompletionMs <= 0 {
		return 0
	}
	return r.CompletionMs - r.ArrivalMs
}

// TokensPerSec returns the throughput for this request.
func (r *Request) TokensPerSec() float64 {
	lat := r.TotalLatencyMs()
	if lat <= 0 {
		return 0
	}
	return float64(r.OutputTokens) / (lat / 1000.0)
}

package model

// TimingModel computes the simulated time for one autoregressive decode step.
// This is the single source of truth for step timing, designed to be easily
// calibrated with real GPU benchmarks later.
//
// Current model (linear approximation):
//
//	step_ms = base_ms + per_token_ms * avg_seq_len + per_batch_ms * batch_size
//
// Where:
//   - base_ms:      fixed overhead per step (kernel launch, sync, etc.)
//   - per_token_ms: marginal cost per token in the average sequence (attention scales with seq len)
//   - per_batch_ms: marginal cost per request in the batch (memory bandwidth)
//   - avg_seq_len:  average total tokens across all sequences in the batch
//   - batch_size:   number of sequences in the current batch
type TimingModel struct {
	BaseMs     float64
	PerTokenMs float64
	PerBatchMs float64
}

// NewTimingModel creates a TimingModel from the config parameters.
func NewTimingModel(baseMs, perTokenMs, perBatchMs float64) *TimingModel {
	return &TimingModel{
		BaseMs:     baseMs,
		PerTokenMs: perTokenMs,
		PerBatchMs: perBatchMs,
	}
}

// StepMs computes the wall-clock time for one decode step given batch parameters.
func (tm *TimingModel) StepMs(batchSize int, avgSeqLen float64) float64 {
	if batchSize == 0 {
		return 0
	}
	return tm.BaseMs + tm.PerTokenMs*avgSeqLen + tm.PerBatchMs*float64(batchSize)
}

// PrefillMs computes the time for prefill (processing all context tokens for a new request).
// Prefill is typically more compute-bound and processes all context tokens at once.
// Simplified: prefill_ms ≈ base_ms + per_token_ms * (context_tokens - matched_prefix)
func (tm *TimingModel) PrefillMs(contextTokens int, matchedPrefix int) float64 {
	return tm.BaseMs + tm.PerTokenMs*float64(contextTokens-matchedPrefix)
}

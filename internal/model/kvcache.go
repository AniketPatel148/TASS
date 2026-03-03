package model

// KVCacheGB computes the KV-cache memory in GB for a request at its current state.
// kv_gb = kv_per_token_gb * (context_tokens + generated_tokens)
func KVCacheGB(r *Request, kvPerTokenGB float64) float64 {
	return kvPerTokenGB * float64(r.TotalTokens())
}

// KVCacheMaxGB computes the worst-case KV-cache memory for a request
// (when all output tokens are generated).
// kv_gb = kv_per_token_gb * (context_tokens + output_tokens)
func KVCacheMaxGB(r *Request, kvPerTokenGB float64) float64 {
	return kvPerTokenGB * float64(r.ContextTokens+r.OutputTokens)
}

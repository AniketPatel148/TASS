package util

import (
	"math"
	"math/rand"
)

// RNG wraps a *rand.Rand with convenience methods for simulation distributions.
type RNG struct {
	r *rand.Rand
}

// NewRNG creates a deterministic RNG from the given seed.
func NewRNG(seed int64) *RNG {
	return &RNG{r: rand.New(rand.NewSource(seed))}
}

// Float64 returns a uniform random float64 in [0, 1).
func (rng *RNG) Float64() float64 {
	return rng.r.Float64()
}

// Intn returns a uniform random int in [0, n).
func (rng *RNG) Intn(n int) int {
	return rng.r.Intn(n)
}

// IntRange returns a uniform random int in [lo, hi].
func (rng *RNG) IntRange(lo, hi int) int {
	if lo >= hi {
		return lo
	}
	return lo + rng.r.Intn(hi-lo+1)
}

// Exponential returns an exponentially distributed random variate with the given rate (lambda).
// Used for Poisson inter-arrival times: inter_arrival = Exponential(rps).
func (rng *RNG) Exponential(rate float64) float64 {
	if rate <= 0 {
		return math.MaxFloat64
	}
	return -math.Log(1.0-rng.r.Float64()) / rate
}

// Poisson returns a Poisson-distributed random variate with the given mean (lambda).
// Uses the Knuth algorithm for small lambda, normal approximation for large lambda.
func (rng *RNG) Poisson(lambda float64) int {
	if lambda <= 0 {
		return 0
	}
	if lambda < 30 {
		// Knuth algorithm
		l := math.Exp(-lambda)
		k := 0
		p := 1.0
		for {
			k++
			p *= rng.r.Float64()
			if p <= l {
				return k - 1
			}
		}
	}
	// Normal approximation for large lambda
	n := rng.r.NormFloat64()*math.Sqrt(lambda) + lambda
	if n < 0 {
		return 0
	}
	return int(math.Round(n))
}

// NormFloat64 returns a standard normal random variate.
func (rng *RNG) NormFloat64() float64 {
	return rng.r.NormFloat64()
}

// Choice returns a random element from the slice. Panics if empty.
func Choice[T any](rng *RNG, items []T) T {
	return items[rng.Intn(len(items))]
}

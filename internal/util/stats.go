package util

import (
	"math"
	"sort"
)

// Percentile computes the p-th percentile (0–100) of a float64 slice.
// Uses linear interpolation. Returns 0 for empty slices.
func Percentile(data []float64, p float64) float64 {
	n := len(data)
	if n == 0 {
		return 0
	}

	sorted := make([]float64, n)
	copy(sorted, data)
	sort.Float64s(sorted)

	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[n-1]
	}

	// Rank using the NIST recommended method
	rank := (p / 100) * float64(n-1)
	lower := int(math.Floor(rank))
	upper := lower + 1
	frac := rank - float64(lower)

	if upper >= n {
		return sorted[n-1]
	}
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}

// JainsIndex computes Jain's fairness index over a slice of non-negative values.
// Returns 1.0 for perfectly fair, 1/n for maximally unfair.
// Returns 1.0 for empty or single-element slices.
func JainsIndex(values []float64) float64 {
	n := len(values)
	if n <= 1 {
		return 1.0
	}

	var sum, sumSq float64
	for _, v := range values {
		sum += v
		sumSq += v * v
	}

	if sumSq == 0 {
		return 1.0
	}

	return (sum * sum) / (float64(n) * sumSq)
}

// Mean computes the arithmetic mean of a float64 slice. Returns 0 for empty slices.
func Mean(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	var sum float64
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

// Sum returns the sum of a float64 slice.
func Sum(data []float64) float64 {
	var s float64
	for _, v := range data {
		s += v
	}
	return s
}

package util

import (
	"math"
	"testing"
)

func TestPercentile_Basic(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	tests := []struct {
		p    float64
		want float64
		tol  float64
	}{
		{50, 5.5, 0.01},
		{0, 1.0, 0.01},
		{100, 10.0, 0.01},
		{95, 9.55, 0.01},
		{99, 9.91, 0.01},
	}

	for _, tc := range tests {
		got := Percentile(data, tc.p)
		if math.Abs(got-tc.want) > tc.tol {
			t.Errorf("Percentile(data, %.0f) = %.4f, want %.4f (±%.2f)", tc.p, got, tc.want, tc.tol)
		}
	}
}

func TestPercentile_Empty(t *testing.T) {
	got := Percentile(nil, 50)
	if got != 0 {
		t.Errorf("Percentile(nil, 50) = %f, want 0", got)
	}
}

func TestPercentile_SingleElement(t *testing.T) {
	got := Percentile([]float64{42}, 99)
	if got != 42 {
		t.Errorf("Percentile([42], 99) = %f, want 42", got)
	}
}

func TestJainsIndex_PerfectlyFair(t *testing.T) {
	values := []float64{10, 10, 10, 10}
	got := JainsIndex(values)
	if math.Abs(got-1.0) > 1e-9 {
		t.Errorf("JainsIndex(equal) = %f, want 1.0", got)
	}
}

func TestJainsIndex_MaximallyUnfair(t *testing.T) {
	// One user gets everything, rest get nothing: J = 1/n
	values := []float64{100, 0, 0, 0}
	got := JainsIndex(values)
	expected := 0.25
	if math.Abs(got-expected) > 1e-9 {
		t.Errorf("JainsIndex(unfair) = %f, want %f", got, expected)
	}
}

func TestJainsIndex_Empty(t *testing.T) {
	got := JainsIndex(nil)
	if got != 1.0 {
		t.Errorf("JainsIndex(nil) = %f, want 1.0", got)
	}
}

func TestMean(t *testing.T) {
	got := Mean([]float64{10, 20, 30})
	if math.Abs(got-20.0) > 1e-9 {
		t.Errorf("Mean = %f, want 20", got)
	}
}

func TestMean_Empty(t *testing.T) {
	got := Mean(nil)
	if got != 0 {
		t.Errorf("Mean(nil) = %f, want 0", got)
	}
}

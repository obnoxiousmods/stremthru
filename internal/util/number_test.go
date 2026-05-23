package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSafeParseInt(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			str      string
			fallback int
			want     int
		}{
			{"valid", "42", 0, 42},
			{"negative", "-7", 0, -7},
			{"zero", "0", 99, 0},
			{"empty_uses_fallback", "", 5, 5},
			{"invalid_uses_fallback", "abc", 5, 5},
			{"float_uses_fallback", "1.5", 5, 5},
			{"trailing_garbage_uses_fallback", "42abc", 5, 5},
			{"whitespace_uses_fallback", " 42 ", 5, 5},
		} {
			t.Run(tc.name, func(t *testing.T) {
				assert.Equal(t, tc.want, SafeParseInt(tc.str, tc.fallback))
			})
		}
	})

	t.Run("int64", func(t *testing.T) {
		assert.Equal(t, int64(9223372036854775807), SafeParseInt("9223372036854775807", int64(0)))
		assert.Equal(t, int64(-1), SafeParseInt("-1", int64(0)))
		assert.Equal(t, int64(99), SafeParseInt("invalid", int64(99)))
	})
}

func TestPercentile(t *testing.T) {
	for _, tc := range []struct {
		name   string
		sorted []float64
		p      float64
		want   float64
	}{
		{"empty", nil, 50, 0},
		{"single", []float64{5}, 50, 5},
		{"single_p0", []float64{5}, 0, 5},
		{"single_p100", []float64{5}, 100, 5},
		{"two_p0", []float64{10, 20}, 0, 10},
		{"two_p50", []float64{10, 20}, 50, 15},
		{"two_p100", []float64{10, 20}, 100, 20},
		{"five_p50", []float64{1, 2, 3, 4, 5}, 50, 3},
		{"five_p25", []float64{1, 2, 3, 4, 5}, 25, 2},
		{"five_p75", []float64{1, 2, 3, 4, 5}, 75, 4},
		{"five_p95", []float64{1, 2, 3, 4, 5}, 95, 4.8},
		{"five_p99", []float64{1, 2, 3, 4, 5}, 99, 4.96},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Percentile(tc.sorted, tc.p)
			assert.InDelta(t, tc.want, got, 0.001)
		})
	}
}

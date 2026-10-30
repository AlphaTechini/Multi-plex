package bench

import (
	"math"
	"slices"
	"time"
)

// Stats summarises a set of durations, in milliseconds.
type Stats struct {
	N    int     `json:"n"`
	Min  float64 `json:"min_ms"`
	Mean float64 `json:"mean_ms"`
	P50  float64 `json:"p50_ms"`
	P95  float64 `json:"p95_ms"`
	P99  float64 `json:"p99_ms"`
	Max  float64 `json:"max_ms"`
}

// ms converts a duration to fractional milliseconds.
func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

// summarize computes Stats over samples. An empty set gives the zero Stats.
func summarize(samples []time.Duration) Stats {
	if len(samples) == 0 {
		return Stats{}
	}
	s := slices.Clone(samples)
	slices.Sort(s)
	var sum time.Duration
	for _, d := range s {
		sum += d
	}
	return Stats{
		N:    len(s),
		Min:  ms(s[0]),
		Mean: ms(sum) / float64(len(s)),
		P50:  ms(nearestRank(s, 50)),
		P95:  ms(nearestRank(s, 95)),
		P99:  ms(nearestRank(s, 99)),
		Max:  ms(s[len(s)-1]),
	}
}

// nearestRank returns the p-th percentile of sorted samples (nearest rank).
func nearestRank(sorted []time.Duration, p int) time.Duration {
	i := int(math.Ceil(float64(len(sorted))*float64(p)/100)) - 1
	return sorted[max(0, min(i, len(sorted)-1))]
}

package statsd

import (
	"math/rand/v2"
)

func shouldSample(rate float64, r *rand.Rand) bool {
	if rate >= 1 {
		return true
	}

	if r.Float64() > rate {
		return false
	}
	return true
}

func copySlice(src []string) []string {
	if src == nil {
		return nil
	}

	c := make([]string, len(src))
	copy(c, src)
	return c
}

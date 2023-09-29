package statsd

import (
	"sync/atomic"
)

func shouldSample(rate float64, attempts, written *atomic.Uint64) bool {
	if rate >= 1 {
		return true
	}

	// protect against dividing by 0
	attempts.CompareAndSwap(0, 1)
	w := written.Load()
	a := attempts.Add(1)
	// we want to submit the metric if the ratio of written vs attempts drops below the threshold
	if float64(w)/float64(a) <= rate {
		written.Add(1)
		return true
	}
	return false
}

func copySlice(src []string) []string {
	if src == nil {
		return nil
	}

	c := make([]string, len(src))
	copy(c, src)
	return c
}

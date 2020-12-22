package statsd

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldSample(t *testing.T) {
	rates := []float64{0.01, 0.05, 0.1, 0.25, 0.5, 0.75, 0.9, 0.99, 1.0}
	iterations := 50_000
	worker := newWorker(newBufferPool(1, 1, 1), nil)
	for _, rate := range rates {
		rate := rate // Capture range variable.
		t.Run(fmt.Sprintf("Rate %0.2f", rate), func(t *testing.T) {
			count := 0
			for i := 0; i < iterations; i++ {
				if worker.shouldSample(rate) {
					count++
				}
			}
			assert.InDelta(t, rate, float64(count)/float64(iterations), 0.01)
		})
	}
}

func BenchmarkShouldSample(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		worker := newWorker(newBufferPool(1, 1, 1), nil)
		for pb.Next() {
			worker.shouldSample(0.1)
		}
	})
}

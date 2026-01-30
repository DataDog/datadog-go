package statsd

import (
	"fmt"
	"testing"
)

// Prevent compiler from optimizing away function calls
var benchErr error

func BenchmarkAggregatorSharding(b *testing.B) {
	shardCounts := []int{1, 2, 3, 4, 5, 6, 8, 16, 32, 64, 128, 256}

	// Pre-generate metric names to avoid measuring fmt.Sprintf performance
	const numMetrics = 100000
	metricNames := make([]string, numMetrics)
	for i := 0; i < numMetrics; i++ {
		metricNames[i] = fmt.Sprintf("metric.%d", i)
	}

	for _, shards := range shardCounts {
		b.Run(fmt.Sprintf("Shards_%d", shards), func(b *testing.B) {
			a := newAggregator(nil, 0, shards)
			tags := []string{"tag:1", "tag:2"}

			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				var err error
				i := 0
				for pb.Next() {
					name := metricNames[i%numMetrics]
					i++
					err = a.count(name, 1, tags, CardinalityLow)
					err = a.gauge(name, 10.0, tags, CardinalityLow)
					err = a.set(name, "val", tags, CardinalityLow)
				}
				benchErr = err
			})
		})
		if benchErr != nil {
			b.Fatal(benchErr)
		}
	}
}

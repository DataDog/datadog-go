package statsd

import (
	"fmt"
	"testing"
)

func BenchmarkAggregatorSharding(b *testing.B) {
	shardCounts := []int{1, 8, 16, 32, 64}

	for _, shards := range shardCounts {
		b.Run(fmt.Sprintf("Shards_%d", shards), func(b *testing.B) {
			a := newAggregator(nil, 0, shards)
			tags := []string{"tag:1", "tag:2"}

			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					i++
					name := fmt.Sprintf("metric.%d", i%100000)
					a.count(name, 1, tags, CardinalityLow)
					a.gauge(name, 10.0, tags, CardinalityLow)
					a.set(name, "val", tags, CardinalityLow)
				}
			})
		})
	}
}

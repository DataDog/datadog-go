package statsd

import (
	"fmt"
	"testing"
)

// Prevent compiler from optimizing away function calls
var benchErr error
var benchAggregator *aggregator
var benchMetrics []metric

func benchmarkMetricNames(n int) []string {
	names := make([]string, n)
	for i := 0; i < n; i++ {
		names[i] = fmt.Sprintf("metric.%d", i)
	}
	return names
}

// Pre-creates contexts so benchmarks can measure the steady-state update path
// separately from first-sample insertion.
func benchmarkPopulateAggregator(a *aggregator, names []string, tags []string) {
	for _, name := range names {
		_ = a.count(name, 1, tags, CardinalityLow)
		_ = a.gauge(name, 10.0, tags, CardinalityLow)
		_ = a.set(name, "val", tags, CardinalityLow)
	}
}

// Measures the cost of creating an aggregator at different shard counts.
func BenchmarkAggregatorConstruct(b *testing.B) {
	for _, shards := range []int{1, 32, 256} {
		b.Run(fmt.Sprintf("Shards_%d", shards), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				benchAggregator = newAggregator(nil, 0, shards)
			}
		})
	}
}

// Measures concurrent first-sample insertion and updates across many contexts.
func BenchmarkAggregatorSharding(b *testing.B) {
	shardCounts := []int{1, 2, 3, 4, 5, 6, 8, 16, 32, 64, 128, 256}

	const numMetrics = 100000
	metricNames := benchmarkMetricNames(numMetrics)
	tags := []string{"tag:1", "tag:2"}

	for _, shards := range shardCounts {
		b.Run(fmt.Sprintf("Shards_%d", shards), func(b *testing.B) {
			a := newAggregator(nil, 0, shards)

			b.ReportAllocs()
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

// Measures the steady-state mixed update path across shard counts and tagset
// sizes.
func BenchmarkAggregatorSampleExistingByTagCount(b *testing.B) {
	shardCounts := []int{1, 2, 4, 8, 16, 32, 64, 128, 256}

	const numMetrics = 100000
	metricNames := benchmarkMetricNames(numMetrics)

	for _, shards := range shardCounts {
		for _, tagCount := range benchmarkTagCounts {
			tags := benchGenerateSizedTags(tagCount, benchTagByteLen)
			b.Run(fmt.Sprintf("Shards_%d_Tags_%d", shards, tagCount), func(b *testing.B) {
				a := newAggregator(nil, 0, shards)
				benchmarkPopulateAggregator(a, metricNames, tags)

				b.ReportAllocs()
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
				if benchErr != nil {
					b.Fatal(benchErr)
				}
			})
		}
	}
}

// Measures flush overhead when no shard contains count, gauge, or set contexts
// to isolate the cost of scanning/resetting idle shards.
func BenchmarkAggregatorFlushEmpty(b *testing.B) {
	for _, shards := range []int{1, 32, 256} {
		b.Run(fmt.Sprintf("Shards_%d", shards), func(b *testing.B) {
			a := newAggregator(nil, 0, shards)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchMetrics = a.flushMetrics()
			}
		})
	}
}

// Measures the sparse-flush case: one context is inserted for each simple
// metric type, then all shards are flushed. This is the hot path most affected
// by lazy map allocation and reset.
func BenchmarkAggregatorFlushSparse(b *testing.B) {
	const shards = 256
	tags := []string{"tag:1", "tag:2"}
	a := newAggregator(nil, 0, shards)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = a.count("metric.same", 1, tags, CardinalityLow)
		_ = a.gauge("metric.same", 10.0, tags, CardinalityLow)
		_ = a.set("metric.same", "val", tags, CardinalityLow)
		benchMetrics = a.flushMetrics()
	}
}

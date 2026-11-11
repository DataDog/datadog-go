package statsd

import (
	"testing"
)

// benchTags is intentionally long so the per-sample key build (and its memmove)
// is visible in the benchmark.
var benchTags = []string{"env:prod", "service:checkout", "endpoint:/v1/orders", "region:us-east-1", "shard:42"}

// BenchmarkAggregatorCountDirect measures the cost of rebuilding and re-hashing
// the context key on every sample (the current behavior).
func BenchmarkAggregatorCountDirect(b *testing.B) {
	c := newAggClientExForTest(8, 0)
	a := c.agg
	// Prime the context so we always hit the existing-entry hot path.
	_ = a.count("bench.count", 1, benchTags, CardinalityNotSet)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = a.count("bench.count", 1, benchTags, CardinalityNotSet)
	}
}

// BenchmarkAggregatorCountContext measures the same workload through a reused
// MetricContext, which skips the per-sample key build.
func BenchmarkAggregatorCountContext(b *testing.B) {
	c := newAggClientExForTest(8, 0)
	mc := c.NewMetricContext("bench.count", benchTags)
	_ = mc.Count(1)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mc.Count(1)
	}
}

// BenchmarkAggregatorHistogramDirect / Context do the same comparison for a
// buffered metric type in mutex mode.
func BenchmarkAggregatorHistogramDirect(b *testing.B) {
	c := newAggClientExForTest(8, 0)
	a := c.agg
	_ = a.histogram("bench.histo", 1, benchTags, 1, CardinalityNotSet)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = a.histogram("bench.histo", 1, benchTags, 1, CardinalityNotSet)
	}
}

func BenchmarkAggregatorHistogramContext(b *testing.B) {
	c := newAggClientExForTest(8, 0)
	mc := c.NewMetricContext("bench.histo", benchTags)
	_ = mc.Histogram(1, 1)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mc.Histogram(1, 1)
	}
}

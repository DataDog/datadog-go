package statsd

import (
	"fmt"
	"strings"
	"testing"
)

var (
	benchContextString string
	benchStringTags    string
	benchContextErr    error
)

func benchGenerateTags(n int) []string {
	tags := make([]string, n)
	for i := range tags {
		tags[i] = fmt.Sprintf("tag%d:tag%d", i, i)
	}
	return tags
}

// Length of a single "key:value" tag used by the tag-count sweeps which covers
// the P99 tag-value pair's length.
const benchTagByteLen = 55

// The set of tag counts swept by the *ByTagCount benchmarks.
var benchmarkTagCounts = []int{5, 10, 40, 60, 100}

// Returns n distinct tags that are each exactly byteLen bytes long, shaped as
// a realistic "key:value" pair.
func benchGenerateSizedTags(n, byteLen int) []string {
	tags := make([]string, n)
	for i := range tags {
		key := fmt.Sprintf("key.%d:", i)
		if len(key) > byteLen {
			key = key[:byteLen]
		}
		tags[i] = key + strings.Repeat("v", byteLen-len(key))
	}
	return tags
}

func benchmarkContextCases() []struct {
	name        string
	metricName  string
	tags        []string
	cardinality Cardinality
} {
	return []struct {
		name        string
		metricName  string
		tags        []string
		cardinality Cardinality
	}{
		{
			name:        "NoTags_NotSet",
			metricName:  "test.metric",
			cardinality: CardinalityNotSet,
		},
		{
			name:        "NoTags_Low",
			metricName:  "test.metric",
			cardinality: CardinalityLow,
		},
		{
			name:        "OneTag_NotSet",
			metricName:  "test.metric",
			tags:        []string{"env:prod"},
			cardinality: CardinalityNotSet,
		},
		{
			name:        "TwoTags_NotSet",
			metricName:  "test.metric",
			tags:        []string{"env:prod", "service:api"},
			cardinality: CardinalityNotSet,
		},
		{
			name:        "OneTag_Low",
			metricName:  "test.metric",
			tags:        []string{"env:prod"},
			cardinality: CardinalityLow,
		},
		{
			name:        "TwoTags_Low",
			metricName:  "test.metric",
			tags:        []string{"env:prod", "service:api"},
			cardinality: CardinalityLow,
		},
		{
			name:        "FiveTags_Low",
			metricName:  "test.metric",
			tags:        []string{"env:prod", "service:api", "version:2026.06.09", "region:us-east-1", "endpoint:/checkout"},
			cardinality: CardinalityLow,
		},
		{
			name:        "FiveTags_Orchestrator",
			metricName:  "test.metric",
			tags:        []string{"env:prod", "service:api", "version:2026.06.09", "region:us-east-1", "pod_name:checkout-api-7f7c8d6f98-b6l9x"},
			cardinality: CardinalityOrchestrator,
		},
		{
			name:        "Tags100_Low",
			metricName:  "test.metric",
			tags:        benchGenerateTags(100),
			cardinality: CardinalityLow,
		},
		{
			name:        "Tags100_NotSet",
			metricName:  "test.metric",
			tags:        benchGenerateTags(100),
			cardinality: CardinalityNotSet,
		},
		{
			name:        "Tags100_High",
			metricName:  "test.metric",
			tags:        benchGenerateTags(100),
			cardinality: CardinalityHigh,
		},
	}
}

func BenchmarkGetContextAndTagsShapes(b *testing.B) {
	for _, tc := range benchmarkContextCases() {
		tc := tc
		b.Run(tc.name, func(b *testing.B) {
			var context string
			var stringTags string

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				context, stringTags = getContextAndTags(tc.metricName, tc.tags, tc.cardinality)
			}
			benchContextString = context
			benchStringTags = stringTags
		})
	}
}

func BenchmarkAggregatorSampleExistingByMetricType(b *testing.B) {
	const (
		shards     = 256
		numMetrics = 100000
	)
	metricNames := benchmarkMetricNames(numMetrics)
	tags := []string{"tag:1", "tag:2"}

	benchmarks := []struct {
		name     string
		populate func(*aggregator, string) error
		sample   func(*aggregator, string) error
	}{
		{
			name: "Count",
			populate: func(a *aggregator, name string) error {
				return a.count(name, 1, tags, CardinalityLow)
			},
			sample: func(a *aggregator, name string) error {
				return a.count(name, 1, tags, CardinalityLow)
			},
		},
		{
			name: "Gauge",
			populate: func(a *aggregator, name string) error {
				return a.gauge(name, 10.0, tags, CardinalityLow)
			},
			sample: func(a *aggregator, name string) error {
				return a.gauge(name, 10.0, tags, CardinalityLow)
			},
		},
		{
			name: "Set",
			populate: func(a *aggregator, name string) error {
				return a.set(name, "initial", tags, CardinalityLow)
			},
			sample: func(a *aggregator, name string) error {
				return a.set(name, "existing", tags, CardinalityLow)
			},
		},
	}

	for _, bm := range benchmarks {
		bm := bm
		b.Run(bm.name, func(b *testing.B) {
			a := newAggregator(nil, 0, shards)
			for _, name := range metricNames {
				if err := bm.populate(a, name); err != nil {
					b.Fatal(err)
				}
			}

			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				var err error
				i := 0
				for pb.Next() {
					name := metricNames[i%numMetrics]
					i++
					err = bm.sample(a, name)
				}
				benchContextErr = err
			})
			if benchContextErr != nil {
				b.Fatal(benchContextErr)
			}
		})
	}
}

func BenchmarkBufferedMetricContextsSampleExistingByShape(b *testing.B) {
	const numMetrics = 100000
	metricNames := benchmarkMetricNames(numMetrics)

	for _, tc := range benchmarkContextCases() {
		tc := tc
		b.Run(tc.name, func(b *testing.B) {
			contexts := newBufferedContexts(newHistogramMetric, 1)
			for _, name := range metricNames {
				if err := contexts.sample(name, 1.0, tc.tags, 1.0, tc.cardinality); err != nil {
					b.Fatal(err)
				}
			}

			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				var err error
				i := 0
				for pb.Next() {
					name := metricNames[i%numMetrics]
					i++
					err = contexts.sample(name, float64(i), tc.tags, 1.0, tc.cardinality)
				}
				benchContextErr = err
			})
			if benchContextErr != nil {
				b.Fatal(benchContextErr)
			}
		})
	}
}

// Isolates the cost of building the context key as the number of tags grows.
// Measures the appendContext/buffer-tier cost on its own.
func BenchmarkGetContextByTagCount(b *testing.B) {
	for _, tagCount := range benchmarkTagCounts {
		tags := benchGenerateSizedTags(tagCount, benchTagByteLen)
		b.Run(fmt.Sprintf("Tags_%d", tagCount), func(b *testing.B) {
			var context string

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				context = getContext("test.metric", tags, CardinalityLow)
			}
			benchContextString = context
		})
	}
}

// Measures the histogram/distribution/timing sample path as the number of tags
// grows, exercising the inline, large-stack, and heap context buffers.
func BenchmarkBufferedMetricContextsSampleExistingByTagCount(b *testing.B) {
	const numMetrics = 100000
	metricNames := benchmarkMetricNames(numMetrics)

	for _, tagCount := range benchmarkTagCounts {
		tags := benchGenerateSizedTags(tagCount, benchTagByteLen)
		b.Run(fmt.Sprintf("Tags_%d", tagCount), func(b *testing.B) {
			contexts := newBufferedContexts(newHistogramMetric, 1)
			for _, name := range metricNames {
				if err := contexts.sample(name, 1.0, tags, 1.0, CardinalityLow); err != nil {
					b.Fatal(err)
				}
			}

			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				var err error
				i := 0
				for pb.Next() {
					name := metricNames[i%numMetrics]
					i++
					err = contexts.sample(name, float64(i), tags, 1.0, CardinalityLow)
				}
				benchContextErr = err
			})
			if benchContextErr != nil {
				b.Fatal(benchContextErr)
			}
		})
	}
}

package statsd

import (
	"fmt"
	"testing"
)

var payloadSink []byte

func benchmarkFormat(b *testing.B, tagsNumber int) {
	payloadSink = make([]byte, 0, 1024*8)
	var tags []string
	for i := 0; i < tagsNumber; i++ {
		tags = append(tags, fmt.Sprintf("tag%d:tag%d\n", i, i))
	}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		payloadSink = appendGauge(payloadSink[:0], "namespace", []string{}, "metric", 1, tags, 0.1)
		payloadSink = appendCount(payloadSink[:0], "namespace", []string{}, "metric", 1, tags, 0.1)
		payloadSink = appendHistogram(payloadSink[:0], "namespace", []string{}, "metric", 1, tags, 0.1)
		payloadSink = appendDistribution(payloadSink[:0], "namespace", []string{}, "metric", 1, tags, 0.1)
		payloadSink = appendDecrement(payloadSink[:0], "namespace", []string{}, "metric", tags, 0.1)
		payloadSink = appendIncrement(payloadSink[:0], "namespace", []string{}, "metric", tags, 0.1)
		payloadSink = appendSet(payloadSink[:0], "namespace", []string{}, "metric", "setelement", tags, 0.1)
		payloadSink = appendTiming(payloadSink[:0], "namespace", []string{}, "metric", 1, tags, 0.1)
	}
}

func benchmarkOldFormat(b *testing.B, tagsNumber int) {
	var tags []string
	for i := 0; i < tagsNumber; i++ {
		tags = append(tags, fmt.Sprintf("tag%d:tag%d\n", i, i))
	}
	c := Client{
		Namespace: "namespace",
		Tags:      tags,
	}

	for n := 0; n < b.N; n++ {
		c.format("metric", float64(1.), gaugeSuffix, tags, 0.1)
		c.format("metric", 1, countSuffix, tags, 0.1)
		c.format("metric", float64(1.), histogramSuffix, tags, 0.1)
		c.format("metric", float64(1.), distributionSuffix, tags, 0.1)
		c.format("metric", nil, incrSuffix, tags, 0.1)
		c.format("metric", nil, decrSuffix, tags, 0.1)
		c.format("metric", "setelement", setSuffix, tags, 0.1)
		c.format("metric", float64(1.), timingSuffix, tags, 0.1)
	}
}

func BenchmarkFormat0(b *testing.B)   { benchmarkOldFormat(b, 0) }
func BenchmarkFormat1(b *testing.B)   { benchmarkOldFormat(b, 1) }
func BenchmarkFormat5(b *testing.B)   { benchmarkOldFormat(b, 5) }
func BenchmarkFormat10(b *testing.B)  { benchmarkOldFormat(b, 10) }
func BenchmarkFormat50(b *testing.B)  { benchmarkOldFormat(b, 50) }
func BenchmarkFormat100(b *testing.B) { benchmarkOldFormat(b, 100) }

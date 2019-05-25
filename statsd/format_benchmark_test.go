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

func BenchmarkFormat0(b *testing.B)   { benchmarkFormat(b, 0) }
func BenchmarkFormat1(b *testing.B)   { benchmarkFormat(b, 1) }
func BenchmarkFormat5(b *testing.B)   { benchmarkFormat(b, 5) }
func BenchmarkFormat10(b *testing.B)  { benchmarkFormat(b, 10) }
func BenchmarkFormat50(b *testing.B)  { benchmarkFormat(b, 50) }
func BenchmarkFormat100(b *testing.B) { benchmarkFormat(b, 100) }

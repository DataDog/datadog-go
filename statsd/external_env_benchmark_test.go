package statsd

import (
	"testing"
)

// BenchmarkGetExternalEnv measures the per-call cost of getExternalEnv when
// the environment variable is set (worst case — value comparison after lock).
func BenchmarkGetExternalEnv(b *testing.B) {
	patchExternalEnv("production")
	defer resetExternalEnv()

	b.ReportAllocs()
	b.ResetTimer()
	var s string
	for i := 0; i < b.N; i++ {
		s = getExternalEnv()
	}
	_ = s
}

// BenchmarkGetExternalEnvEmpty measures the empty-value path (default for
// most users — DD_EXTERNAL_ENV is unset).
func BenchmarkGetExternalEnvEmpty(b *testing.B) {
	resetExternalEnv()

	b.ReportAllocs()
	b.ResetTimer()
	var s string
	for i := 0; i < b.N; i++ {
		s = getExternalEnv()
	}
	_ = s
}

// BenchmarkGetExternalEnvParallel hammers getExternalEnv from GOMAXPROCS
// goroutines. This is where RWMutex contention surfaces — the read lock
// still does atomic CAS on a shared cache line, so it scales poorly with
// the number of cores even though there are no writers.
func BenchmarkGetExternalEnvParallel(b *testing.B) {
	patchExternalEnv("production")
	defer resetExternalEnv()

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var s string
		for pb.Next() {
			s = getExternalEnv()
		}
		_ = s
	})
}

// BenchmarkAppendExternalEnv covers the actual hot path — appendExternalEnv
// is called from every metric write when origin detection is on.
func BenchmarkAppendExternalEnv(b *testing.B) {
	patchExternalEnv("production")
	defer resetExternalEnv()

	buf := make([]byte, 0, 64)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf = appendExternalEnv(buf[:0], true)
	}
}

// BenchmarkAppendExternalEnvParallel: the realistic concurrent metric-write
// case. Each metric emit on every worker goroutine calls this.
func BenchmarkAppendExternalEnvParallel(b *testing.B) {
	patchExternalEnv("production")
	defer resetExternalEnv()

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		buf := make([]byte, 0, 64)
		for pb.Next() {
			buf = appendExternalEnv(buf[:0], true)
		}
	})
}

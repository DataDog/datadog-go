package fastrand

import (
	"math/rand"
	"sync"
	"testing"
	"time"
)

func BenchmarkFastFloat64(b *testing.B) {
	b.Run("fastrand.Float64", func(b *testing.B) {
		b.Run("parallel", func(b *testing.B) {
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					_ = Float64()
				}
			})
		})

		b.Run("serial", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = Float64()
			}
		})
	})

	b.Run("rand.Float64", func(b *testing.B) {
		b.Run("parallel", func(b *testing.B) {
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					_ = rand.Float64()
				}
			})
		})

		b.Run("serial", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = rand.Float64()
			}
		})
	})

	b.Run("rand.NewSource.Float64", func(b *testing.B) {
		var mu sync.Mutex
		random := rand.New(rand.NewSource(time.Now().UnixNano()))
		b.Run("parallel", func(b *testing.B) {
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					mu.Lock()
					_ = random.Float64()
					mu.Unlock()
				}
			})
		})

		b.Run("serial", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = random.Float64()
			}
		})
	})
}

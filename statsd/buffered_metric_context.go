package statsd

import (
	"math/rand/v2"
	"sync"
	"sync/atomic"

	"github.com/puzpuzpuz/xsync"
)

// bufferedMetricContexts represent the contexts for Histograms, Distributions
// and Timing. Since those 3 metric types behave the same way and are sampled
// with the same type they're represented by the same class.
type bufferedMetricContexts struct {
	nbContext uint64
	mutex     sync.RWMutex
	values    *xsync.MapOf[string, *bufferedMetric]
	newMetric func(string, float64, string, float64) *bufferedMetric

	random *rand.Rand
}

func newBufferedContexts(newMetric func(string, float64, string, int64, float64) *bufferedMetric, maxSamples int64) bufferedMetricContexts {
	return bufferedMetricContexts{
		values: xsync.NewMapOf[*bufferedMetric](),
		newMetric: func(name string, value float64, stringTags string, rate float64) *bufferedMetric {
			return newMetric(name, value, stringTags, maxSamples, rate)
		},
		// Note that calling "time.Now().UnixNano()" repeatedly quickly may return
		// very similar values. That's fine for seeding the worker-specific random
		// source because we just need an evenly distributed stream of float values.
		// Do not use this random source for cryptographic randomness.
		random: rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())),
	}
}

func (bc *bufferedMetricContexts) flush(metrics []metric) []metric {
	var nbContext uint64

	bc.values.Range(func(key string, v *bufferedMetric) bool {
		if atomic.LoadInt64(&v.totalSamples) == 0 {
			vv, loaded := bc.values.LoadAndDelete(key)
			if !loaded || atomic.LoadInt64(&vv.totalSamples) == 0 {
				// We don't need to flush this metric, we can just
				// remove it from the map.
				return true
			}

			v = vv
		}

		nbContext++
		metrics = append(metrics, v.flushResetUnsafe())
		return true
	})

	atomic.AddUint64(&bc.nbContext, nbContext)
	return metrics
}

func (bc *bufferedMetricContexts) sample(name string, value float64, tags []string, rate float64) error {
	keepingSample := shouldSample(rate, bc.random)

	// If we don't keep the sample, return early. If we do keep the sample
	// we end up storing the *first* observed sampling rate in the metric.
	// This is the *wrong* behavior but it's the one we had before and the alternative would increase lock contention too
	// much with the current code.
	// TODO: change this behavior in the future, probably by introducing thread-local storage and lockless stuctures.
	// If this code is removed, also remove the observed sampling rate in the metric and fix `bufferedMetric.flushUnsafe()`
	if !keepingSample {
		return nil
	}

	context, stringTags := getContextAndTags(name, tags)
	v, loaded := bc.values.Load(context)
	if !loaded {
		// We need to create a new metric
		v, loaded = bc.values.LoadOrStore(context, bc.newMetric(name, value, stringTags, rate))
		if !loaded {
			// We created a new metric, it's been sampled already.
			return nil
		}
	}

	// Now we can keep the sample or skip it
	if keepingSample {
		v.sampleUnsafe(value)
	} else {
		v.skipSample()
	}

	return nil
}

func (bc *bufferedMetricContexts) getNbContext() uint64 {
	return atomic.LoadUint64(&bc.nbContext)
}

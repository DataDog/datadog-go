package statsd

import (
	"sync"
	"sync/atomic"
)

// bufferedMetricContexts represent the contexts for Histograms, Distributions
// and Timing. Since those 3 metric types behave the same way and are sampled
// with the same type they're represented by the same class.
type bufferedMetricContexts struct {
	nbContext uint64
	mutex     sync.RWMutex
	values    bufferedMetricMap
	newMetric func(string, float64, string) *bufferedMetric

	// These values keep track of how many times metric submission was attempted and
	// let through, which allows the BMC to sample points per the caller's rate
	// parameter.
	attempts atomic.Uint64
	written  atomic.Uint64
}

func newBufferedContexts(newMetric func(string, float64, string) *bufferedMetric) bufferedMetricContexts {
	return bufferedMetricContexts{
		values:    bufferedMetricMap{},
		newMetric: newMetric,
	}
}

func (bc *bufferedMetricContexts) flush(metrics []metric) []metric {
	bc.mutex.Lock()
	values := bc.values
	bc.values = bufferedMetricMap{}
	bc.mutex.Unlock()

	for _, d := range values {
		metrics = append(metrics, d.flushUnsafe())
	}
	atomic.AddUint64(&bc.nbContext, uint64(len(values)))
	return metrics
}

func (bc *bufferedMetricContexts) sample(name string, value float64, tags []string, rate float64) error {
	if !shouldSample(rate, &bc.attempts, &bc.written) {
		return nil
	}

	context, stringTags := getContextAndTags(name, tags)

	bc.mutex.RLock()
	if v, found := bc.values[context]; found {
		v.sample(value)
		bc.mutex.RUnlock()
		return nil
	}
	bc.mutex.RUnlock()

	bc.mutex.Lock()
	// Check if another goroutines hasn't created the value betwen the 'RUnlock' and 'Lock'
	if v, found := bc.values[context]; found {
		v.sample(value)
		bc.mutex.Unlock()
		return nil
	}
	bc.values[context] = bc.newMetric(name, value, stringTags)
	bc.mutex.Unlock()
	return nil
}

func (bc *bufferedMetricContexts) getNbContext() uint64 {
	return atomic.LoadUint64(&bc.nbContext)
}

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
	nbSample  uint64
	mutex     sync.RWMutex
	values    bufferedMetricMap
	newMetric func(string, float64, string) *bufferedMetric
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

	samples := 0
	for _, d := range values {
		samples += len(d.data)
		metrics = append(metrics, d.flushUnsafe())
	}

	atomic.AddUint64(&bc.nbSample, uint64(samples))
	atomic.AddUint64(&bc.nbContext, uint64(len(values)))
	return metrics
}

func (bc *bufferedMetricContexts) sample(name string, value float64, tags []string, rate float64) {
	if !shouldSample(rate) {
		return
	}

	context, stringTags := getContextAndTags(name, tags)

	bc.mutex.RLock()
	if v, found := bc.values[context]; found {
		v.sample(value)
		bc.mutex.RUnlock()
		return
	}
	bc.mutex.RUnlock()

	bc.mutex.Lock()
	// Check if another goroutine hasn't created the value between the 'RUnlock' and 'Lock'
	if v, found := bc.values[context]; found {
		v.sample(value)
		bc.mutex.Unlock()
		return
	}
	bc.values[context] = bc.newMetric(name, value, stringTags)
	bc.mutex.Unlock()
	return
}

func (bc *bufferedMetricContexts) sampleBulk(bulkMap bufferedMetricMap) {
	for context, bm := range bulkMap {
		bc.mutex.RLock()
		if v, found := bc.values[context]; found {
			v.sampleBulk(bm.data)
			bc.mutex.RUnlock()
			continue
		}
		bc.mutex.RUnlock()

		bc.mutex.Lock()
		if v, found := bc.values[context]; found {
			// Check if another goroutine hasn't created the value between the 'RUnlock' and 'Lock'
			v.sampleBulk(bm.data)
			bc.mutex.Unlock()
			continue
		}
		bc.values[context] = bm
		bc.mutex.Unlock()
	}
}

func (bc *bufferedMetricContexts) getNbContext() uint64 {
	return atomic.LoadUint64(&bc.nbContext)
}

func (bc *bufferedMetricContexts) getNbSample() uint64 {
	return atomic.LoadUint64(&bc.nbSample)
}

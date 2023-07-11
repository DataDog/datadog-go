package statsd

import (
	"sync"
)

func newLossyBufferPool(newMetric func(string, float64, string) *bufferedMetric, flushThreshold int) *sync.Pool {
	return &sync.Pool{
		New: func() interface{} {
			return &lossyBuffer{values: bufferedMetricMap{}, newMetric: newMetric, flushThreshold: flushThreshold}
		},
	}
}

type lossyBuffer struct {
	samples        int
	flushThreshold int

	values    bufferedMetricMap
	newMetric func(string, float64, string) *bufferedMetric
}

// sample the incoming metric
func (l *lossyBuffer) sample(name string, value float64, tags []string) {
	context, stringTags := getContextAndTags(name, tags)
	if bm, ok := l.values[context]; ok {
		bm.sampleUnsafe(value)
	} else {
		l.values[context] = l.newMetric(name, value, stringTags)
	}

	l.samples++
}

// full returns true if this buffer has sampled enough metrics for a flush, false otherwise.
func (l *lossyBuffer) full() bool {
	return l.samples >= l.flushThreshold
}

// flush returns the internal bufferedMetricMap and resets this buffer so that it can be put back into a pool
func (l *lossyBuffer) flush() bufferedMetricMap {
	buffer := l.values

	// Reset this buffer
	l.samples = 0
	l.values = bufferedMetricMap{}

	return buffer
}

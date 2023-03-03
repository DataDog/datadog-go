package statsd

import (
	"sync"
)

// flushSampleThreshold is the sample threshold for the lossy buffer to flush its samples to the buffered metric context
// aggregator
const flushSampleThreshold int = 64

func newLossyBufferPool(newMetric func(string, float64, string) *bufferedMetric) *sync.Pool {
	return &sync.Pool{
		New: func() interface{} {
			return &lossyBuffer{values: bufferedMetricMap{}, newMetric: newMetric}
		},
	}
}

type lossyBuffer struct {
	samples int

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

// full returns true if this buffer has sampled enough metrics, false otherwise.
func (l *lossyBuffer) full() bool {
	return l.samples >= flushSampleThreshold
}

// flush returns the internal bufferedMetricMap and resets this buffer so that it can be put back into a pool
func (l *lossyBuffer) flush() bufferedMetricMap {
	buffer := l.values

	// Reset this buffer
	l.samples = 0
	l.values = bufferedMetricMap{}

	return buffer
}

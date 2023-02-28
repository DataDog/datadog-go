package statsd

import (
	"sync"
)

// flushSampleThreshold is the sample threshold for the lossy buffer to flush its samples to the buffered metric context
// aggregator
const flushSampleThreshold int = 64

var lossyBufferPoolTiming = &sync.Pool{
	New: func() interface{} {
		return &lossyBuffer{values: bufferedMetricMap{}, newMetric: newTimingMetric}
	},
}

var lossyBufferPoolDistribution = &sync.Pool{
	New: func() interface{} {
		return &lossyBuffer{values: bufferedMetricMap{}, newMetric: newDistributionMetric}
	},
}

var lossyBufferPoolHistogram = &sync.Pool{
	New: func() interface{} {
		return &lossyBuffer{values: bufferedMetricMap{}, newMetric: newHistogramMetric}
	},
}

type lossyBuffer struct {
	samples int

	values    bufferedMetricMap
	newMetric func(string, float64, string) *bufferedMetric
}

// Sample the incoming metric
func (l *lossyBuffer) Sample(name string, value float64, tags []string) {
	context, stringTags := getContextAndTags(name, tags)
	if bm, ok := l.values[context]; ok {
		bm.sampleUnsafe(value)
	} else {
		l.values[context] = l.newMetric(name, value, stringTags)
	}

	l.samples++
	//fmt.Printf("sampling %d\n", l.samples)
}

// Full returns true if this buffer has sampled enough metrics, false otherwise.
func (l *lossyBuffer) Full() bool {
	return l.samples >= flushSampleThreshold
}

// Flush returns the internal bufferedMetricMap and resets this buffer so that it can be put back into a pool
func (l *lossyBuffer) Flush() bufferedMetricMap {
	buffer := l.values

	// Reset this buffer
	l.samples = 0
	l.values = bufferedMetricMap{}

	return buffer
}

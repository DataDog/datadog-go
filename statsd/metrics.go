package statsd

import (
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
)

/*
Those are metrics type that can be aggregated on the client side:
  - Gauge
  - Count
  - Set
*/

type countMetric struct {
	value int64
	name  string
	tags  []string
}

func newCountMetric(name string, value int64, tags []string) *countMetric {
	return &countMetric{
		value: value,
		name:  name,
		tags:  copySlice(tags),
	}
}

func (c *countMetric) sample(v int64) {
	atomic.AddInt64(&c.value, v)
}

func (c *countMetric) flushUnsafe() metric {
	return metric{
		metricType: count,
		name:       c.name,
		tags:       c.tags,
		rate:       1,
		ivalue:     c.value,
	}
}

// Gauge

type gaugeMetric struct {
	value uint64
	name  string
	tags  []string
}

func newGaugeMetric(name string, value float64, tags []string) *gaugeMetric {
	return &gaugeMetric{
		value: math.Float64bits(value),
		name:  name,
		tags:  copySlice(tags),
	}
}

func (g *gaugeMetric) sample(v float64) {
	atomic.StoreUint64(&g.value, math.Float64bits(v))
}

func (g *gaugeMetric) flushUnsafe() metric {
	return metric{
		metricType: gauge,
		name:       g.name,
		tags:       g.tags,
		rate:       1,
		fvalue:     math.Float64frombits(g.value),
	}
}

// Set

type setMetric struct {
	data map[string]struct{}
	name string
	tags []string
	sync.Mutex
}

func newSetMetric(name string, value string, tags []string) *setMetric {
	set := &setMetric{
		data: map[string]struct{}{},
		name: name,
		tags: copySlice(tags),
	}
	set.data[value] = struct{}{}
	return set
}

func (s *setMetric) sample(v string) {
	s.Lock()
	defer s.Unlock()
	s.data[v] = struct{}{}
}

// Sets are aggregated on the agent side too. We flush the keys so a set from
// multiple application can be correctly aggregated on the agent side.
func (s *setMetric) flushUnsafe() []metric {
	if len(s.data) == 0 {
		return nil
	}

	metrics := make([]metric, len(s.data))
	i := 0
	for value := range s.data {
		metrics[i] = metric{
			metricType: set,
			name:       s.name,
			tags:       s.tags,
			rate:       1,
			svalue:     value,
		}
		i++
	}
	return metrics
}

// Histograms, Distributions and Timings

type bufferedMetric struct {
	sync.Mutex

	// Kept samples (after sampling)
	data []float64
	// Total stored samples (after sampling)
	storedSamples int64
	// Total number of observed samples (before sampling). This is used to keep
	// the sampling rate correct.
	totalSamples int64

	name string
	// Histograms and Distributions store tags as one string since we need
	// to compute its size multiple time when serializing.
	tags  string
	mtype metricType

	// maxSamples is the maximum number of samples we keep in memory
	maxSamples int64
}

func (s *bufferedMetric) sample(v float64) {
	s.Lock()
	defer s.Unlock()
	s.sampleUnsafe(v)
}

func (s *bufferedMetric) sampleUnsafe(v float64) {
	s.data = append(s.data, v)
	s.storedSamples++
	// Total samples needs to be incremented though an atomic because it can be accessed without the lock.
	atomic.AddInt64(&s.totalSamples, 1)
}

func (s *bufferedMetric) maybeKeepSample(v float64, rand *rand.Rand, randLock *sync.Mutex) {
	s.Lock()
	defer s.Unlock()
	if s.maxSamples > 0 {
		if s.storedSamples >= s.maxSamples {
			// We reached the maximum number of samples we can keep in memory, so we randomly
			// replace a sample.
			randLock.Lock()
			i := rand.Int63n(atomic.LoadInt64(&s.totalSamples))
			randLock.Unlock()
			if i < s.maxSamples {
				s.data[i] = v
			}
		} else {
			s.data[s.storedSamples] = v
			s.storedSamples++
		}
		s.totalSamples++
	} else {
		// This code path appends to the slice since we did not pre-allocate memory in this case.
		s.sampleUnsafe(v)
	}
}

func (s *bufferedMetric) skipSample() {
	atomic.AddInt64(&s.totalSamples, 1)
}

func (s *bufferedMetric) flushUnsafe() metric {
	return metric{
		metricType: s.mtype,
		name:       s.name,
		stags:      s.tags,
		rate:       float64(s.storedSamples) / float64(atomic.LoadInt64(&s.totalSamples)),
		fvalues:    s.data[:s.storedSamples],
	}
}

type histogramMetric = bufferedMetric

func newHistogramMetric(name string, value float64, stringTags string, maxSamples int64) *histogramMetric {
	return &histogramMetric{
		data:          newData(value, maxSamples),
		totalSamples:  1,
		storedSamples: 1,
		name:          name,
		tags:          stringTags,
		mtype:         histogramAggregated,
		maxSamples:    maxSamples,
	}
}

type distributionMetric = bufferedMetric

func newDistributionMetric(name string, value float64, stringTags string, maxSamples int64) *distributionMetric {
	return &distributionMetric{
		data:          newData(value, maxSamples),
		totalSamples:  1,
		storedSamples: 1,
		name:          name,
		tags:          stringTags,
		mtype:         distributionAggregated,
		maxSamples:    maxSamples,
	}
}

type timingMetric = bufferedMetric

func newTimingMetric(name string, value float64, stringTags string, maxSamples int64) *timingMetric {
	return &timingMetric{
		data:          newData(value, maxSamples),
		totalSamples:  1,
		storedSamples: 1,
		name:          name,
		tags:          stringTags,
		mtype:         timingAggregated,
		maxSamples:    maxSamples,
	}
}

// newData creates a new slice of float64 with the given capacity. If maxSample
// is less than or equal to 0, it returns a slice with the given value as the
// only element.
func newData(value float64, maxSample int64) []float64 {
	if maxSample <= 0 {
		return []float64{value}
	} else {
		data := make([]float64, maxSample)
		data[0] = value
		return data
	}
}

package statsd

import (
	"math"
	"sync/atomic"

	"github.com/puzpuzpuz/xsync"
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
		ivalue:     atomic.LoadInt64(&c.value),
	}
}

func (c *countMetric) flushResetUnsafe() metric {
	return metric{
		metricType: count,
		name:       c.name,
		tags:       c.tags,
		rate:       1,
		ivalue:     atomic.SwapInt64(&c.value, 0),
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
		fvalue:     math.Float64frombits(atomic.LoadUint64(&g.value)),
	}
}

func (g *gaugeMetric) flushResetUnsafe() metric {
	return metric{
		metricType: gauge,
		name:       g.name,
		tags:       g.tags,
		rate:       1,
		fvalue:     math.Float64frombits(atomic.SwapUint64(&g.value, math.MaxUint64)),
	}
}

// Set

type setMetric struct {
	data *xsync.MapOf[string, struct{}]
	name string
	tags []string
}

func newSetMetric(name string, value string, tags []string) *setMetric {
	set := &setMetric{
		data: xsync.NewMapOf[struct{}](),
		name: name,
		tags: copySlice(tags),
	}
	set.data.Store(value, struct{}{})
	return set
}

func (s *setMetric) sample(v string) {
	s.data.Store(v, struct{}{})
}

// Sets are aggregated on the agent side too. We flush the keys so a set from
// multiple application can be correctly aggregated on the agent side.
func (s *setMetric) flushUnsafe() []metric {
	if s.data.Size() == 0 {
		return nil
	}

	metrics := make([]metric, 0, s.data.Size())

	s.data.Range(func(key string, _ struct{}) bool {
		_, loaded := s.data.LoadAndDelete(key)
		if loaded {
			metrics = append(metrics, metric{
				metricType: set,
				name:       s.name,
				tags:       s.tags,
				rate:       1,
				svalue:     key,
			})
		}

		return true
	})

	return metrics
}

func (s *setMetric) toMap() map[string]struct{} {
	m := make(map[string]struct{}, s.data.Size())
	s.data.Range(func(key string, _ struct{}) bool {
		m[key] = struct{}{}
		return true
	})

	return m
}

// Histograms, Distributions and Timings

type bufferedMetric struct {
	// Kept samples (after sampling)
	data data

	// Total number of observed samples (before sampling). This is used to keep
	// the sampling rate correct.
	totalSamples int64

	name string
	// Histograms and Distributions store tags as one string since we need
	// to compute its size multiple time when serializing.
	tags  string
	mtype metricType

	// The first observed user-specified sample rate. When specified
	// it is used because we don't know better.
	specifiedRate float64
}

func (s *bufferedMetric) sample(v float64) {
	s.sampleUnsafe(v)
}

func (s *bufferedMetric) sampleUnsafe(v float64) {
	s.data.sample(v)
	// Total samples needs to be incremented though an atomic because it can be accessed without the lock.
	atomic.AddInt64(&s.totalSamples, 1)
}

func (s *bufferedMetric) skipSample() {
	atomic.AddInt64(&s.totalSamples, 1)
}

func (s *bufferedMetric) flushUnsafe() metric {
	totalSamples := atomic.LoadInt64(&s.totalSamples)
	var rate float64

	fValues := s.data.flushToArray()

	// If the user had a specified rate send it because we don't know better.
	// This code should be removed once we can also remove the early return at the top of
	// `bufferedMetricContexts.sample`
	if s.specifiedRate != 1.0 {
		rate = s.specifiedRate
	} else {
		rate = float64(len(fValues)) / float64(totalSamples)
	}

	return metric{
		metricType: s.mtype,
		name:       s.name,
		stags:      s.tags,
		rate:       rate,
		fvalues:    fValues[:],
	}
}

func (s *bufferedMetric) flushResetUnsafe() metric {
	totalSamples := atomic.SwapInt64(&s.totalSamples, 0)
	var rate float64

	fValues := s.data.flushResetToArray()

	// If the user had a specified rate send it because we don't know better.
	// This code should be removed once we can also remove the early return at the top of
	// `bufferedMetricContexts.sample`
	if s.specifiedRate != 1.0 {
		rate = s.specifiedRate
	} else {
		rate = float64(len(fValues)) / float64(totalSamples)
	}

	return metric{
		metricType: s.mtype,
		name:       s.name,
		stags:      s.tags,
		rate:       rate,
		fvalues:    fValues[:],
	}
}

type histogramMetric = bufferedMetric

func newHistogramMetric(name string, value float64, stringTags string, maxSamples int64, rate float64) *histogramMetric {
	return &histogramMetric{
		data:          newData(value, maxSamples),
		totalSamples:  1,
		name:          name,
		tags:          stringTags,
		mtype:         histogramAggregated,
		specifiedRate: rate,
	}
}

type distributionMetric = bufferedMetric

func newDistributionMetric(name string, value float64, stringTags string, maxSamples int64, rate float64) *distributionMetric {
	return &distributionMetric{
		data:          newData(value, maxSamples),
		totalSamples:  1,
		name:          name,
		tags:          stringTags,
		mtype:         distributionAggregated,
		specifiedRate: rate,
	}
}

type timingMetric = bufferedMetric

func newTimingMetric(name string, value float64, stringTags string, maxSamples int64, rate float64) *timingMetric {
	return &timingMetric{
		data:          newData(value, maxSamples),
		totalSamples:  1,
		name:          name,
		tags:          stringTags,
		mtype:         timingAggregated,
		specifiedRate: rate,
	}
}

// newData creates a new slice of float64 with the given capacity. If maxSample
// is less than or equal to 0, it returns a slice with the given value as the
// only element.
func newData(value float64, maxSample int64) data {
	if maxSample <= 0 {
		d := &unlimitedData{
			data: xsync.NewTypedMapOf[float64, *atomic.Uint64](func(f float64) uint64 {
				return math.Float64bits(f)
			}),
		}

		d.sample(value)

		return d
	} else {
		d := &limitedData{
			data: make(chan float64, maxSample),
		}

		d.sample(value)

		return d
	}
}

type data interface {
	sample(float64)
	flushToArray() []float64
	flushResetToArray() []float64
}

type unlimitedData struct {
	data *xsync.MapOf[float64, *atomic.Uint64]
}

// flushToArray flushes the data to an array of float64. It uses a
// Range function to iterate over the map and load and delete each value.
func (d *unlimitedData) flushToArray() []float64 {
	arr := make([]float64, 0, d.data.Size())

	d.data.Range(func(k float64, value *atomic.Uint64) bool {
		for i := 0; i < int(value.Load()); i++ {
			arr = append(arr, k)
		}

		return true
	})

	return arr
}

// flushToArray flushes the data to an array of float64. It uses a
// Range function to iterate over the map and load and delete each value.
func (d *unlimitedData) flushResetToArray() []float64 {
	arr := make([]float64, 0, d.data.Size())

	d.data.Range(func(k float64, _ *atomic.Uint64) bool {
		value, _ := d.data.LoadAndDelete(k)

		for i := 0; i < int(value.Load()); i++ {
			arr = append(arr, k)
		}

		return true
	})

	return arr
}

// sample adds a sample to the data. It uses LoadOrStore to get the
// atomic.Uint64 for the given value. If it doesn't exist, it creates a new
// one. Then it increments the value by 1.
func (d *unlimitedData) sample(v float64) {
	vv, _ := d.data.LoadOrStore(v, &atomic.Uint64{})
	vv.Add(1)
}

type limitedData struct {
	data chan float64
}

// flushToArray flushes the data to an array of float64. It uses a
// for loop to read from the channel until it is empty or the length of
// the array is equal to the length of the data.
func (d *limitedData) flushToArray() []float64 {
	l := len(d.data)

	arr := make([]float64, 0, l)

	c := true
	for c && len(arr) < l {
		select {
		case v := <-d.data:
			arr = append(arr, v)
			d.sample(v)
		default:
			// we're done reading from the channel
			c = false
		}
	}

	return arr
}

// flushToArray flushes the data to an array of float64. It uses a
// for loop to read from the channel until it is empty or the length of
// the array is equal to the length of the data.
func (d *limitedData) flushResetToArray() []float64 {
	l := len(d.data)

	arr := make([]float64, 0, l)

	c := true
	for c && len(arr) < l {
		select {
		case v := <-d.data:
			arr = append(arr, v)
		default:
			// we're done reading from the channel
			c = false
		}
	}

	return arr
}

func (d *limitedData) sample(v float64) {
	// Try to send the sample to the channel. If it is full, we need to drop
	// the oldest sample to make room for the new one. If we can't send to the
	// channel, after dropping the oldest sample, we drop the new sample.

	select {
	case d.data <- v:
		// We can send the sample to the channel.
	default:
		// We can't send the sample to the channel because it is full.

		select {
		case <-d.data:
			// We've dropped the oldest sample to make room for the new one.
			select {
			case d.data <- v:
			default:
				// We can't send the sample to the channel because it is full, drop
			}
		default:
			// Channel appears to be empty, we can send the sample to the channel.
			select {
			case d.data <- v:
			default:
				// We can't send the sample to the channel because it is full, drop
			}
		}
	}
}

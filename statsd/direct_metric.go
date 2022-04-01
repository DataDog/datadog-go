package statsd

import (
	"sync"
	"sync/atomic"
)

type directMetric interface {
	flush() []metric
}

// Gauge

type Gauge struct {
	sync.Mutex

	value     float64
	name      string
	tags      []string
	sampled   bool
	telemetry *statsdTelemetry
}

// Sample samples the metric
func (g *Gauge) Sample(v float64) {
	atomic.AddUint64(&g.telemetry.totalMetricsGauge, 1)
	g.Lock()
	defer g.Unlock()
	g.value = v
	g.sampled = true
}

func (g *Gauge) flush() []metric {
	g.Lock()
	defer g.Unlock()

	if !g.sampled {
		return nil
	}

	v := g.value
	g.value = 0
	g.sampled = false

	return []metric{
		{
			metricType: gauge,
			name:       g.name,
			tags:       g.tags,
			rate:       1,
			fvalue:     v,
		},
	}
}

// Count

type Count struct {
	sync.Mutex

	value     int64
	name      string
	tags      []string
	sampled   bool
	telemetry *statsdTelemetry
}

// Sample samples the metric
func (c *Count) Sample(v int64) {
	atomic.AddUint64(&c.telemetry.totalMetricsCount, 1)
	c.Lock()
	defer c.Unlock()

	c.value = v
	c.sampled = true
}

func (c *Count) flush() []metric {
	c.Lock()
	defer c.Unlock()

	if !c.sampled {
		return nil
	}

	v := c.value
	c.value = 0
	c.sampled = false

	return []metric{
		{
			metricType: count,
			name:       c.name,
			tags:       c.tags,
			rate:       1,
			ivalue:     v,
		},
	}
}

// Set

type Set struct {
	sync.Mutex

	data      map[string]struct{}
	name      string
	tags      []string
	telemetry *statsdTelemetry
}

// Sample samples the metric
func (s *Set) Sample(v string) {
	atomic.AddUint64(&s.telemetry.totalMetricsSet, 1)
	s.Lock()
	defer s.Unlock()
	s.data[v] = struct{}{}
}

// Sets are aggregated on the agent side too. We flush the keys so a set from
// multiple application can be correctly aggregated on the agent side.
func (s *Set) flush() []metric {
	s.Lock()
	defer s.Unlock()

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
	s.data = map[string]struct{}{}
	return metrics
}

// Histograms, Distributions and Timings

type directBufferedMetric struct {
	sync.Mutex

	data []float64
	name string
	// Histograms and Distributions store tags as one string since we need
	// to compute its size multiple time when serializing.
	tags      string
	mtype     metricType
	telemetry *statsdTelemetry
}
type Histogram = directBufferedMetric
type Distribution = directBufferedMetric
type Timing = directBufferedMetric

// Sample samples the metric
func (s *directBufferedMetric) Sample(v float64) {
	atomic.AddUint64(&s.telemetry.totalMetricsDistribution, 1)
	s.Lock()
	defer s.Unlock()
	s.data = append(s.data, v)
}

func (s *directBufferedMetric) flush() []metric {
	s.Lock()
	defer s.Unlock()
	if len(s.data) == 0 {
		return nil
	}
	data := s.data
	s.data = []float64{}

	return []metric{
		{
			metricType: s.mtype,
			name:       s.name,
			stags:      s.tags,
			rate:       1,
			fvalues:    data,
		},
	}
}

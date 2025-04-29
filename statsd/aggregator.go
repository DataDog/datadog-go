package statsd

import (
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/puzpuzpuz/xsync"
)

type aggregator struct {
	nbContextGauge uint64
	nbContextCount uint64
	nbContextSet   uint64

	countsM sync.RWMutex
	gaugesM sync.RWMutex
	setsM   sync.RWMutex

	gauges        *xsync.MapOf[string, *gaugeMetric]
	counts        *xsync.MapOf[string, *countMetric]
	sets          *xsync.MapOf[string, *setMetric]
	histograms    bufferedMetricContexts
	distributions bufferedMetricContexts
	timings       bufferedMetricContexts

	closed chan struct{}

	client *Client

	// aggregator implements channelMode mechanism to receive histograms,
	// distributions and timings. Since they need sampling they need to
	// lock for random. When using both channelMode and ExtendedAggregation
	// we don't want goroutine to fight over the lock.
	inputMetrics    chan metric
	stopChannelMode chan struct{}
	wg              sync.WaitGroup
}

func newAggregator(c *Client, maxSamplesPerContext int64) *aggregator {
	return &aggregator{
		client:          c,
		counts:          xsync.NewMapOf[*countMetric](),
		gauges:          xsync.NewMapOf[*gaugeMetric](),
		sets:            xsync.NewMapOf[*setMetric](),
		histograms:      newBufferedContexts(newHistogramMetric, maxSamplesPerContext),
		distributions:   newBufferedContexts(newDistributionMetric, maxSamplesPerContext),
		timings:         newBufferedContexts(newTimingMetric, maxSamplesPerContext),
		closed:          make(chan struct{}),
		stopChannelMode: make(chan struct{}),
	}
}

func (a *aggregator) start(flushInterval time.Duration) {
	ticker := time.NewTicker(flushInterval)

	go func() {
		for {
			select {
			case <-ticker.C:
				a.flush()
			case <-a.closed:
				ticker.Stop()
				return
			}
		}
	}()
}

func (a *aggregator) startReceivingMetric(bufferSize int, nbWorkers int) {
	a.inputMetrics = make(chan metric, bufferSize)
	for i := 0; i < nbWorkers; i++ {
		a.wg.Add(1)
		go a.pullMetric()
	}
}

func (a *aggregator) stopReceivingMetric() {
	close(a.stopChannelMode)
	a.wg.Wait()
}

func (a *aggregator) stop() {
	a.closed <- struct{}{}
}

func (a *aggregator) pullMetric() {
	for {
		select {
		case m := <-a.inputMetrics:
			switch m.metricType {
			case histogram:
				a.histogram(m.name, m.fvalue, m.tags, m.rate)
			case distribution:
				a.distribution(m.name, m.fvalue, m.tags, m.rate)
			case timing:
				a.timing(m.name, m.fvalue, m.tags, m.rate)
			}
		case <-a.stopChannelMode:
			a.wg.Done()
			return
		}
	}
}

func (a *aggregator) flush() {
	for _, m := range a.flushMetrics() {
		a.client.sendBlocking(m)
	}
}

func (a *aggregator) flushTelemetryMetrics(t *Telemetry) {
	if a == nil {
		// aggregation is disabled
		return
	}

	t.AggregationNbContextGauge = atomic.LoadUint64(&a.nbContextGauge)
	t.AggregationNbContextCount = atomic.LoadUint64(&a.nbContextCount)
	t.AggregationNbContextSet = atomic.LoadUint64(&a.nbContextSet)
	t.AggregationNbContextHistogram = a.histograms.getNbContext()
	t.AggregationNbContextDistribution = a.distributions.getNbContext()
	t.AggregationNbContextTiming = a.timings.getNbContext()
}

func (a *aggregator) flushMetrics() []metric {
	metrics := make([]metric, 0, a.sets.Size()+a.gauges.Size()+a.counts.Size())

	setContexts := 0
	a.sets.Range(func(key string, v *setMetric) bool {
		if v.data.Size() == 0 {
			vv, loaded := a.sets.LoadAndDelete(key)
			if !loaded || vv.data.Size() == 0 {
				return true
			}

			v = vv
		}

		setContexts++
		metrics = append(metrics, v.flushUnsafe()...)
		return true
	})

	gaugeContexts := 0
	a.gauges.Range(func(key string, v *gaugeMetric) bool {
		if atomic.LoadUint64(&v.value) == math.MaxUint64 {
			vv, ok := a.gauges.LoadAndDelete(key)
			if !ok || vv.value == math.MaxUint64 {
				return true
			}
			v = vv
		}

		gaugeContexts++
		metrics = append(metrics, v.flushResetUnsafe())
		return true
	})

	counterContexts := 0
	a.counts.Range(func(key string, v *countMetric) bool {
		if atomic.LoadInt64(&v.value) == 0 {
			vv, ok := a.counts.LoadAndDelete(key)
			if !ok || vv.value == 0 {
				return true
			}
			v = vv
		}

		counterContexts++
		metrics = append(metrics, v.flushResetUnsafe())
		return true
	})

	metrics = a.histograms.flush(metrics)
	metrics = a.distributions.flush(metrics)
	metrics = a.timings.flush(metrics)

	atomic.AddUint64(&a.nbContextCount, uint64(counterContexts))
	atomic.AddUint64(&a.nbContextGauge, uint64(gaugeContexts))
	atomic.AddUint64(&a.nbContextSet, uint64(setContexts))
	return metrics
}

// getContext returns the context for a metric name and tags.
//
// The context is the metric name and tags separated by a separator symbol.
// It is not intended to be used as a metric name but as a unique key to aggregate
func getContext(name string, tags []string) string {
	c, _ := getContextAndTags(name, tags)
	return c
}

// getContextAndTags returns the context and tags for a metric name and tags.
//
// See getContext for usage for context
// The tags are the tags separated by a separator symbol and can be re-used to pass down to the writer
func getContextAndTags(name string, tags []string) (string, string) {
	if len(tags) == 0 {
		return name, ""
	}
	n := len(name) + len(nameSeparatorSymbol) + len(tagSeparatorSymbol)*(len(tags)-1)
	for _, s := range tags {
		n += len(s)
	}

	var sb strings.Builder
	sb.Grow(n)
	sb.WriteString(name)
	sb.WriteString(nameSeparatorSymbol)
	sb.WriteString(tags[0])
	for _, s := range tags[1:] {
		sb.WriteString(tagSeparatorSymbol)
		sb.WriteString(s)
	}

	s := sb.String()

	return s, s[len(name)+len(nameSeparatorSymbol):]
}

func (a *aggregator) count(name string, value int64, tags []string) error {
	context := getContext(name, tags)

	v, loaded := a.counts.Load(context)
	if loaded {
		v.sample(value)
		return nil
	}

	v, loaded = a.counts.LoadOrStore(context, newCountMetric(name, value, tags))

	if loaded {
		v.sample(value)
	}

	return nil
}

func (a *aggregator) gauge(name string, value float64, tags []string) error {
	context := getContext(name, tags)

	v, loaded := a.gauges.Load(context)
	if loaded {
		v.sample(value)
		return nil
	}

	v, loaded = a.gauges.LoadOrStore(context, newGaugeMetric(name, value, tags))

	if loaded {
		v.sample(value)
	}

	return nil
}

func (a *aggregator) set(name string, value string, tags []string) error {
	context := getContext(name, tags)

	v, loaded := a.sets.Load(context)
	if loaded {
		v.sample(value)
		return nil
	}

	v, loaded = a.sets.LoadOrStore(context, newSetMetric(name, value, tags))

	if loaded {
		v.sample(value)
	}

	return nil
}

// Only histograms, distributions and timings are sampled with a rate since we
// only pack them in on message instead of aggregating them. Discarding the
// sample rate will have impacts on the CPU and memory usage of the Agent.

// type alias for Client.sendToAggregator
type bufferedMetricSampleFunc func(name string, value float64, tags []string, rate float64) error

func (a *aggregator) histogram(name string, value float64, tags []string, rate float64) error {
	return a.histograms.sample(name, value, tags, rate)
}

func (a *aggregator) distribution(name string, value float64, tags []string, rate float64) error {
	return a.distributions.sample(name, value, tags, rate)
}

func (a *aggregator) timing(name string, value float64, tags []string, rate float64) error {
	return a.timings.sample(name, value, tags, rate)
}

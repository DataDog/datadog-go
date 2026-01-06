package statsd

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type bufferedMetricMap map[string]*bufferedMetric

type statMap struct {
	// mu guards access to m (the pointer), not m (the map itself.) m is
	// safe to access concurrently. The purpose of mu is to allow flushing
	// to safely swap out m for a new map.
	mu sync.RWMutex
	m  *sync.Map
}

func newStatMap() *statMap {
	return &statMap{m: new(sync.Map)}
}

func (s *statMap) loadOrCreate(key string, f func() any) (any, bool) {
	v, ok := s.m.Load(key)
	if !ok {
		v, ok = s.m.LoadOrStore(key, f())
	}
	return v, ok
}

func (s *statMap) flush() func(yield func(key string, value any) bool) {
	// This matches iter.Seq2, even though we can't
	// depend on that yet
	return func(yield func(key string, value any) bool) {
		s.mu.Lock()
		m := s.m
		s.m = new(sync.Map)
		s.mu.Unlock()
		m.Range(func(k, v any) bool {
			return yield(k.(string), v)
		})
	}
}

type aggregator struct {
	nbContextGauge uint64
	nbContextCount uint64
	nbContextSet   uint64

	gauges        *statMap
	counts        *statMap
	sets          *statMap
	histograms    bufferedMetricContexts
	distributions bufferedMetricContexts
	timings       bufferedMetricContexts

	closed chan struct{}

	client *ClientEx

	// aggregator implements channelMode mechanism to receive histograms,
	// distributions and timings. Since they need sampling they need to
	// lock for random. When using both channelMode and ExtendedAggregation
	// we don't want goroutine to fight over the lock.
	inputMetrics    chan metric
	stopChannelMode chan struct{}
	wg              sync.WaitGroup
}

func newAggregator(c *ClientEx, maxSamplesPerContext int64) *aggregator {
	return &aggregator{
		client:          c,
		counts:          newStatMap(),
		gauges:          newStatMap(),
		sets:            newStatMap(),
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
				a.histogram(m.name, m.fvalue, m.tags, m.rate, m.overrideCard)
			case distribution:
				a.distribution(m.name, m.fvalue, m.tags, m.rate, m.overrideCard)
			case timing:
				a.timing(m.name, m.fvalue, m.tags, m.rate, m.overrideCard)
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
	metrics := []metric{}

	// We reset the values to avoid sending 'zero' values for metrics not
	// sampled during this flush interval

	sets := 0
	a.sets.flush()(func(_ string, s any) bool {
		metrics = append(metrics, s.(*setMetric).flushUnsafe()...)
		sets++
		return true
	})

	gauges := 0
	a.gauges.flush()(func(_ string, g any) bool {
		metrics = append(metrics, g.(*gaugeMetric).flushUnsafe())
		gauges++
		return true
	})

	counts := 0
	a.counts.flush()(func(_ string, c any) bool {
		metrics = append(metrics, c.(*countMetric).flushUnsafe())
		counts++
		return true
	})

	metrics = a.histograms.flush(metrics)
	metrics = a.distributions.flush(metrics)
	metrics = a.timings.flush(metrics)

	atomic.AddUint64(&a.nbContextCount, uint64(counts))
	atomic.AddUint64(&a.nbContextGauge, uint64(gauges))
	atomic.AddUint64(&a.nbContextSet, uint64(sets))
	return metrics
}

// getContext returns the context for a metric name, tags, and cardinality.
//
// The context is the metric name, tags, and cardinality separated by separator symbols.
// It is not intended to be used as a metric name but as a unique key to aggregate
func getContext(name string, tags []string, cardinality Cardinality) string {
	c, _ := getContextAndTags(name, tags, cardinality)
	return c
}

// getContextAndTags returns the context and tags for a metric name, tags, and cardinality.
//
// See getContext for usage for context
// The tags are the tags separated by a separator symbol and can be re-used to pass down to the writer
func getContextAndTags(name string, tags []string, cardinality Cardinality) (string, string) {
	cardString := cardinality.String()
	if len(tags) == 0 {
		if cardString == "" {
			return name, ""
		}
		return name + nameSeparatorSymbol + cardinality.String(), ""
	}

	n := len(name) + len(nameSeparatorSymbol) + len(tagSeparatorSymbol)*(len(tags)-1)
	for _, s := range tags {
		n += len(s)
	}
	var cardStringLen = 0
	if cardString != "" {
		n += len(cardString) + len(cardSeparatorSymbol)
		cardStringLen = len(cardString) + len(cardSeparatorSymbol)
	}

	var sb strings.Builder
	sb.Grow(n)
	sb.WriteString(name)
	sb.WriteString(nameSeparatorSymbol)
	if cardString != "" {
		sb.WriteString(cardString)
		sb.WriteString(cardSeparatorSymbol)
	}
	sb.WriteString(tags[0])
	for _, s := range tags[1:] {
		sb.WriteString(tagSeparatorSymbol)
		sb.WriteString(s)
	}

	s := sb.String()

	return s, s[len(name)+len(nameSeparatorSymbol)+cardStringLen:]
}

func (a *aggregator) count(name string, value int64, tags []string, cardinality Cardinality) error {
	resolvedCardinality := resolveCardinality(cardinality)
	context := getContext(name, tags, resolvedCardinality)
	a.counts.mu.RLock()
	defer a.counts.mu.RUnlock()
	v, ok := a.counts.loadOrCreate(context, func() any { return newCountMetric(name, value, tags, resolvedCardinality) })
	if !ok {
		// This call created the value for the given key,
		// so we don't need to sample
		return nil
	}
	v.(*countMetric).sample(value)
	return nil
}

func (a *aggregator) gauge(name string, value float64, tags []string, cardinality Cardinality) error {
	resolvedCardinality := resolveCardinality(cardinality)
	context := getContext(name, tags, resolvedCardinality)
	a.gauges.mu.RLock()
	defer a.gauges.mu.RUnlock()
	v, ok := a.gauges.loadOrCreate(context, func() any { return newGaugeMetric(name, value, tags, resolvedCardinality) })
	if !ok {
		return nil
	}
	v.(*gaugeMetric).sample(value)
	return nil
}

func (a *aggregator) set(name string, value string, tags []string, cardinality Cardinality) error {
	resolvedCardinality := resolveCardinality(cardinality)
	context := getContext(name, tags, resolvedCardinality)
	a.sets.mu.RLock()
	defer a.sets.mu.RUnlock()
	v, ok := a.sets.loadOrCreate(context, func() any { return newSetMetric(name, value, tags, resolvedCardinality) })
	if !ok {
		return nil
	}
	v.(*setMetric).sample(value)
	return nil
}

// Only histograms, distributions and timings are sampled with a rate since we
// only pack them in on message instead of aggregating them. Discarding the
// sample rate will have impacts on the CPU and memory usage of the Agent.

// type alias for Client.sendToAggregator
type bufferedMetricSampleFunc func(name string, value float64, tags []string, rate float64, cardinality Cardinality) error

func (a *aggregator) histogram(name string, value float64, tags []string, rate float64, cardinality Cardinality) error {
	return a.histograms.sample(name, value, tags, rate, cardinality)
}

func (a *aggregator) distribution(name string, value float64, tags []string, rate float64, cardinality Cardinality) error {
	return a.distributions.sample(name, value, tags, rate, cardinality)
}

func (a *aggregator) timing(name string, value float64, tags []string, rate float64, cardinality Cardinality) error {
	return a.timings.sample(name, value, tags, rate, cardinality)
}

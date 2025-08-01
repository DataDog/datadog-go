package statsd

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type (
	countsMap         map[string]*countMetric
	gaugesMap         map[string]*gaugeMetric
	setsMap           map[string]*setMetric
	bufferedMetricMap map[string]*bufferedMetric
)

type aggregator struct {
	nbContextGauge uint64
	nbContextCount uint64
	nbContextSet   uint64

	countsM sync.RWMutex
	gaugesM sync.RWMutex
	setsM   sync.RWMutex

	gauges        gaugesMap
	counts        countsMap
	sets          setsMap
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
		counts:          countsMap{},
		gauges:          gaugesMap{},
		sets:            setsMap{},
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

	a.setsM.Lock()
	sets := a.sets
	a.sets = setsMap{}
	a.setsM.Unlock()

	for _, s := range sets {
		metrics = append(metrics, s.flushUnsafe()...)
	}

	a.gaugesM.Lock()
	gauges := a.gauges
	a.gauges = gaugesMap{}
	a.gaugesM.Unlock()

	for _, g := range gauges {
		metrics = append(metrics, g.flushUnsafe())
	}

	a.countsM.Lock()
	counts := a.counts
	a.counts = countsMap{}
	a.countsM.Unlock()

	for _, c := range counts {
		metrics = append(metrics, c.flushUnsafe())
	}

	metrics = a.histograms.flush(metrics)
	metrics = a.distributions.flush(metrics)
	metrics = a.timings.flush(metrics)

	atomic.AddUint64(&a.nbContextCount, uint64(len(counts)))
	atomic.AddUint64(&a.nbContextGauge, uint64(len(gauges)))
	atomic.AddUint64(&a.nbContextSet, uint64(len(sets)))
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
	a.countsM.RLock()
	if count, found := a.counts[context]; found {
		count.sample(value)
		a.countsM.RUnlock()
		return nil
	}
	a.countsM.RUnlock()

	a.countsM.Lock()
	// Check if another goroutines hasn't created the value betwen the RUnlock and 'Lock'
	if count, found := a.counts[context]; found {
		count.sample(value)
		a.countsM.Unlock()
		return nil
	}

	a.counts[context] = newCountMetric(name, value, tags, resolvedCardinality)
	a.countsM.Unlock()
	return nil
}

func (a *aggregator) gauge(name string, value float64, tags []string, cardinality Cardinality) error {
	resolvedCardinality := resolveCardinality(cardinality)
	context := getContext(name, tags, resolvedCardinality)
	a.gaugesM.RLock()
	if gauge, found := a.gauges[context]; found {
		gauge.sample(value)
		a.gaugesM.RUnlock()
		return nil
	}
	a.gaugesM.RUnlock()

	gauge := newGaugeMetric(name, value, tags, resolvedCardinality)

	a.gaugesM.Lock()
	// Check if another goroutines hasn't created the value betwen the 'RUnlock' and 'Lock'
	if gauge, found := a.gauges[context]; found {
		gauge.sample(value)
		a.gaugesM.Unlock()
		return nil
	}
	a.gauges[context] = gauge
	a.gaugesM.Unlock()
	return nil
}

func (a *aggregator) set(name string, value string, tags []string, cardinality Cardinality) error {
	resolvedCardinality := resolveCardinality(cardinality)
	context := getContext(name, tags, resolvedCardinality)
	a.setsM.RLock()
	if set, found := a.sets[context]; found {
		set.sample(value)
		a.setsM.RUnlock()
		return nil
	}
	a.setsM.RUnlock()

	a.setsM.Lock()
	// Check if another goroutines hasn't created the value betwen the 'RUnlock' and 'Lock'
	if set, found := a.sets[context]; found {
		set.sample(value)
		a.setsM.Unlock()
		return nil
	}
	a.sets[context] = newSetMetric(name, value, tags, resolvedCardinality)
	a.setsM.Unlock()
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

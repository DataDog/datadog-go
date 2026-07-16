package statsd

import (
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

// keyBufPool pools scratch buffers used to build context keys. Map lookups
// written as m[string(buf)] are allocation-free (the compiler elides the
// conversion), so hot paths build the key in a pooled buffer, probe the map,
// and only materialize a string on the insert path, where the key is retained.
var keyBufPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 0, 128)
		return &b
	},
}

func putKeyBuf(bufp *[]byte, buf []byte) {
	if cap(buf) > 1<<16 {
		// Don't pin pathologically large buffers in the pool.
		return
	}
	*bufp = buf[:0]
	keyBufPool.Put(bufp)
}

// appendContext appends the context key for a metric to buf and returns the
// extended buffer along with the offset at which the tags portion starts.
// The tags portion is empty iff the offset equals len(buf).
func appendContext(buf []byte, name string, tags []string, cardinality Cardinality) ([]byte, int) {
	cardString := cardinality.String()

	buf = append(buf, name...)
	if len(tags) == 0 && cardString == "" {
		return buf, len(buf)
	}
	buf = append(buf, nameSeparatorSymbol...)
	if cardString != "" {
		buf = append(buf, cardString...)
		if len(tags) == 0 {
			return buf, len(buf)
		}
		buf = append(buf, cardSeparatorSymbol...)
	}
	tagsOffset := len(buf)
	buf = append(buf, tags[0]...)
	for _, s := range tags[1:] {
		buf = append(buf, tagSeparatorSymbol...)
		buf = append(buf, s...)
	}
	return buf, tagsOffset
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
		return name + nameSeparatorSymbol + cardString, ""
	}
	n := len(name) + len(nameSeparatorSymbol) + len(tagSeparatorSymbol)*(len(tags)-1)
	for _, s := range tags {
		n += len(s)
	}
	if cardString != "" {
		n += len(cardString) + len(cardSeparatorSymbol)
	}
	buf, tagsOffset := appendContext(make([]byte, 0, n), name, tags, cardinality)
	context := string(buf)
	return context, context[tagsOffset:]
}

func (a *aggregator) count(name string, value int64, tags []string, cardinality Cardinality) error {
	resolvedCardinality := resolveCardinality(cardinality)
	bufp := keyBufPool.Get().(*[]byte)
	buf, _ := appendContext((*bufp)[:0], name, tags, resolvedCardinality)
	a.countsM.RLock()
	if count, found := a.counts[string(buf)]; found {
		count.sample(value)
		a.countsM.RUnlock()
		putKeyBuf(bufp, buf)
		return nil
	}
	a.countsM.RUnlock()

	a.countsM.Lock()
	// Check if another goroutines hasn't created the value betwen the RUnlock and 'Lock'
	if count, found := a.counts[string(buf)]; found {
		count.sample(value)
		a.countsM.Unlock()
		putKeyBuf(bufp, buf)
		return nil
	}

	a.counts[string(buf)] = newCountMetric(name, value, tags, resolvedCardinality)
	a.countsM.Unlock()
	putKeyBuf(bufp, buf)
	return nil
}

func (a *aggregator) gauge(name string, value float64, tags []string, cardinality Cardinality) error {
	resolvedCardinality := resolveCardinality(cardinality)
	bufp := keyBufPool.Get().(*[]byte)
	buf, _ := appendContext((*bufp)[:0], name, tags, resolvedCardinality)
	a.gaugesM.RLock()
	if gauge, found := a.gauges[string(buf)]; found {
		gauge.sample(value)
		a.gaugesM.RUnlock()
		putKeyBuf(bufp, buf)
		return nil
	}
	a.gaugesM.RUnlock()

	gauge := newGaugeMetric(name, value, tags, resolvedCardinality)

	a.gaugesM.Lock()
	// Check if another goroutines hasn't created the value betwen the 'RUnlock' and 'Lock'
	if gauge, found := a.gauges[string(buf)]; found {
		gauge.sample(value)
		a.gaugesM.Unlock()
		putKeyBuf(bufp, buf)
		return nil
	}
	a.gauges[string(buf)] = gauge
	a.gaugesM.Unlock()
	putKeyBuf(bufp, buf)
	return nil
}

func (a *aggregator) set(name string, value string, tags []string, cardinality Cardinality) error {
	resolvedCardinality := resolveCardinality(cardinality)
	bufp := keyBufPool.Get().(*[]byte)
	buf, _ := appendContext((*bufp)[:0], name, tags, resolvedCardinality)
	a.setsM.RLock()
	if set, found := a.sets[string(buf)]; found {
		set.sample(value)
		a.setsM.RUnlock()
		putKeyBuf(bufp, buf)
		return nil
	}
	a.setsM.RUnlock()

	a.setsM.Lock()
	// Check if another goroutines hasn't created the value betwen the 'RUnlock' and 'Lock'
	if set, found := a.sets[string(buf)]; found {
		set.sample(value)
		a.setsM.Unlock()
		putKeyBuf(bufp, buf)
		return nil
	}
	a.sets[string(buf)] = newSetMetric(name, value, tags, resolvedCardinality)
	a.setsM.Unlock()
	putKeyBuf(bufp, buf)
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

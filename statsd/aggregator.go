package statsd

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const ddInternalCardPrefix = "dd.internal.card:"

type (
	countsMap         map[string]*countMetric
	gaugesMap         map[string]*gaugeMetric
	setsMap           map[string]*setMetric
	bufferedMetricMap map[string]*bufferedMetric
)

type Clock interface {
	Now() time.Time
	Since(t time.Time) time.Duration
	Sleep(d time.Duration)
}

type RealClock struct{}

func (RealClock) Now() time.Time                  { return time.Now() }
func (RealClock) Since(t time.Time) time.Duration { return time.Since(t) }
func (RealClock) Sleep(d time.Duration)           { time.Sleep(d) }

type aggregator struct {
	nbContextGauge       uint64
	nbContextCount       uint64
	nbCountWithTimestamp uint64
	nbGaugeWithTimestamp uint64
	nbContextSet         uint64

	countsM sync.RWMutex
	gaugesM sync.RWMutex
	setsM   sync.RWMutex

	gauges        gaugesMap
	counts        countsMap
	prevCounts    countsMap
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

	sendWithTimestamps bool
	sendSmoothly       bool

	clock Clock
}

func newAggregatorWithClock(c *Client, maxSamplesPerContext int64, sendWithTimestamps bool, sendSmoothly bool, clock Clock) *aggregator {
	return &aggregator{
		client:             c,
		counts:             countsMap{},
		prevCounts:         countsMap{},
		gauges:             gaugesMap{},
		sets:               setsMap{},
		histograms:         newBufferedContexts(newHistogramMetric, maxSamplesPerContext),
		distributions:      newBufferedContexts(newDistributionMetric, maxSamplesPerContext),
		timings:            newBufferedContexts(newTimingMetric, maxSamplesPerContext),
		closed:             make(chan struct{}),
		stopChannelMode:    make(chan struct{}),
		sendWithTimestamps: sendWithTimestamps,
		sendSmoothly:       sendSmoothly,
		clock:              clock,
	}
}

func newAggregator(c *Client, maxSamplesPerContext int64, sendWithTimestamps bool, sendSmoothly bool) *aggregator {
	return newAggregatorWithClock(c, maxSamplesPerContext, sendWithTimestamps, sendSmoothly, RealClock{})
}

func (a *aggregator) sleepUntilFirstTick(interval time.Duration, offset time.Duration) {
	now := a.clock.Now().UnixNano()
	intNs := interval.Nanoseconds()
	offNs := offset.Nanoseconds()
	elapsed := now % intNs // since the last interval boundary
	sleep := 0 * time.Nanosecond
	if elapsed <= offNs { // we haven't passed the offset, sleep till it
		sleep = time.Duration(offNs - elapsed)
	} else { // we passed the offset, sleep till next interval+offset
		sleep = time.Duration((intNs - elapsed) + offNs)
	}
	a.clock.Sleep(sleep)
}

func (a *aggregator) start(interval time.Duration, offset time.Duration) {
	go func() {
		if offset >= 0 { // Sleep till the specified offset past the interval.
			a.sleepUntilFirstTick(interval, offset)
		}
		ticker := time.NewTicker(interval)

		for {
			select {
			case <-ticker.C:
				a.flush(interval)
			case <-a.closed:
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

func (a *aggregator) canSendWithTimestamp(m metric) bool {
	if !a.sendWithTimestamps {
		return false
	}

	if m.metricType == count {
		// For counts we already expect to have either a unique origin tag and we told the user
		// to enable origin detection. But if `dd.internal.card` is specified we're very likely to break
		// something so we don't use a timestamp and ask the agent to do count aggregation as usual.
		for _, tag := range m.tags {
			if strings.HasPrefix(tag, ddInternalCardPrefix) {
				return false
			}
		}
		return true
	}
	if m.metricType == gauge {
		// We assume that for gauges it's fine, because inside they agent they
		// would have be overwritten too.
		return true
	}

	return false
}

func getThrottleDuration(reported, total int, elapsed, target time.Duration) time.Duration {
	if reported == total {
		return 0
	}
	t := time.Millisecond * time.Duration(int(float64(target.Milliseconds())/float64(total)*float64(reported)))
	remaining := t - elapsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (a *aggregator) throttleReporting(reported, total int, elapsed, target time.Duration) time.Duration {
	remaining := getThrottleDuration(reported, total, elapsed, target)
	if remaining > 0 {
		a.clock.Sleep(remaining)
		return remaining
	}
	return 0
}

func (a *aggregator) flush(targetDuration time.Duration) {
	metrics := a.flushMetrics()
	total := len(metrics)
	reported := 0
	start := a.clock.Now()

	ts := start.Unix()

	for _, m := range metrics {
		// FIXME: do something better.
		if m.timestamp == -1 {
			m.timestamp = ts
		}
		a.client.sendBlocking(m)

		reported++

		// If we are throttling, we should sleep to make sure we report the metrics evenly
		// over the reporting interval. Only do this if we have a target duration and every few metrics.
		if a.sendSmoothly && targetDuration > 0 && total%100 == 0 {
			elapsed := time.Since(start)
			a.throttleReporting(total, reported, elapsed, targetDuration)
		}

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
	t.AggregationNbCountWithTimestamp = atomic.LoadUint64(&a.nbCountWithTimestamp)
	t.AggregationNbGaugeWithTimestamp = atomic.LoadUint64(&a.nbGaugeWithTimestamp)
}

// flushMetricWithTimestamp handles the common pattern of flushing a metric and handling timestamps
func (a *aggregator) flushMetricWithTimestamp(m metric) (metric, int) {
	if a.canSendWithTimestamp(m) {
		m.timestamp = -1
		return m, 1
	}
	return m, 0
}

func (a *aggregator) flushMetrics() []metric {
	metrics := []metric{}
	nbCountWithTimestamp := 0
	nbGaugeWithTimestamp := 0

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
		m, hasTs := a.flushMetricWithTimestamp(g.flushUnsafe())
		nbGaugeWithTimestamp += hasTs
		metrics = append(metrics, m)
	}

	a.countsM.Lock()
	// We need to keep the previous counts to eventually send trailing 0s.
	prevCounts := a.prevCounts
	counts := a.counts
	a.prevCounts = a.counts
	a.counts = countsMap{}
	a.countsM.Unlock()

	for _, c := range counts {
		m, hasTs := a.flushMetricWithTimestamp(c.flushUnsafe())
		nbCountWithTimestamp += hasTs
		metrics = append(metrics, m)
	}
	// We only send trailing 0s if we are sending with timestamps because that is what the agent would have done.
	if a.sendWithTimestamps {
		// We know that counts can't be in both counts and prevCounts, we still need to flush
		// them.
		for _, c := range prevCounts {
			m, hasTs := a.flushMetricWithTimestamp(c.flushUnsafe())
			nbCountWithTimestamp += hasTs
			metrics = append(metrics, m)
		}
	}

	metrics = a.histograms.flush(metrics)
	metrics = a.distributions.flush(metrics)
	metrics = a.timings.flush(metrics)

	atomic.AddUint64(&a.nbContextCount, uint64(len(counts)))
	atomic.AddUint64(&a.nbContextGauge, uint64(len(gauges)))
	atomic.AddUint64(&a.nbContextSet, uint64(len(sets)))
	atomic.AddUint64(&a.nbCountWithTimestamp, uint64(nbCountWithTimestamp))
	atomic.AddUint64(&a.nbGaugeWithTimestamp, uint64(nbGaugeWithTimestamp))
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

	if a.sendWithTimestamps {
		// If we are sending with timestamps, we need to keep the previous counts to eventually send trailing 0s.
		// Meaning we can re-use the previous count metric instead of creating a new one.
		if prevCount, found := a.prevCounts[context]; found {
			prevCount.set(value)
			a.counts[context] = prevCount
			delete(a.prevCounts, context)
			a.countsM.Unlock()
			return nil
		}
	}
	a.counts[context] = newCountMetric(name, value, tags)
	a.countsM.Unlock()
	return nil
}

func (a *aggregator) gauge(name string, value float64, tags []string) error {
	context := getContext(name, tags)
	a.gaugesM.RLock()
	if gauge, found := a.gauges[context]; found {
		gauge.sample(value)
		a.gaugesM.RUnlock()
		return nil
	}
	a.gaugesM.RUnlock()

	gauge := newGaugeMetric(name, value, tags)

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

func (a *aggregator) set(name string, value string, tags []string) error {
	context := getContext(name, tags)
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
	a.sets[context] = newSetMetric(name, value, tags)
	a.setsM.Unlock()
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

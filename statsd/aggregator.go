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

const (
	smallContextBufferSize = 512
	largeContextBufferSize = 4 * 1024
)

type countShard struct {
	sync.RWMutex
	counts countsMap
}

type gaugeShard struct {
	sync.RWMutex
	gauges gaugesMap
}

type setShard struct {
	sync.RWMutex
	sets setsMap
}

type aggregator struct {
	nbContextGauge uint64
	nbContextCount uint64
	nbContextSet   uint64

	shardsCount int
	countShards []countShard
	gaugeShards []gaugeShard
	setShards   []setShard

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

func newAggregator(c *ClientEx, maxSamplesPerContext int64, shardsCount int) *aggregator {
	agg := &aggregator{
		client:          c,
		shardsCount:     shardsCount,
		countShards:     make([]countShard, shardsCount),
		gaugeShards:     make([]gaugeShard, shardsCount),
		setShards:       make([]setShard, shardsCount),
		histograms:      newBufferedContexts(newHistogramMetric, maxSamplesPerContext),
		distributions:   newBufferedContexts(newDistributionMetric, maxSamplesPerContext),
		timings:         newBufferedContexts(newTimingMetric, maxSamplesPerContext),
		closed:          make(chan struct{}),
		stopChannelMode: make(chan struct{}),
	}
	return agg
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
				a.histogram(m.name, m.fvalue, m.tags, m.rate, m.cardinality)
			case distribution:
				a.distribution(m.name, m.fvalue, m.tags, m.rate, m.cardinality)
			case timing:
				a.timing(m.name, m.fvalue, m.tags, m.rate, m.cardinality)
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

	for i := range a.setShards {
		shard := &a.setShards[i]
		shard.Lock()
		sets := shard.sets
		if len(sets) == 0 {
			shard.Unlock()
			continue
		}
		shard.sets = nil
		shard.Unlock()
		for _, s := range sets {
			metrics = append(metrics, s.flushUnsafe()...)
		}
		atomic.AddUint64(&a.nbContextSet, uint64(len(sets)))
	}

	for i := range a.gaugeShards {
		shard := &a.gaugeShards[i]
		shard.Lock()
		gauges := shard.gauges
		if len(gauges) == 0 {
			shard.Unlock()
			continue
		}
		shard.gauges = nil
		shard.Unlock()
		for _, g := range gauges {
			metrics = append(metrics, g.flushUnsafe())
		}
		atomic.AddUint64(&a.nbContextGauge, uint64(len(gauges)))
	}

	for i := range a.countShards {
		shard := &a.countShards[i]
		shard.Lock()
		counts := shard.counts
		if len(counts) == 0 {
			shard.Unlock()
			continue
		}
		shard.counts = nil
		shard.Unlock()
		for _, c := range counts {
			metrics = append(metrics, c.flushUnsafe())
		}
		atomic.AddUint64(&a.nbContextCount, uint64(len(counts)))
	}

	metrics = a.histograms.flush(metrics)
	metrics = a.distributions.flush(metrics)
	metrics = a.timings.flush(metrics)

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
		return name + nameSeparatorSymbol + cardString, ""
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

func getContextLength(name string, tags []string, cardString string) int {
	if len(tags) == 0 {
		if cardString == "" {
			return len(name)
		}
		return len(name) + len(nameSeparatorSymbol) + len(cardString)
	}

	n := len(name) + len(nameSeparatorSymbol) + len(tagSeparatorSymbol)*(len(tags)-1)
	for _, tag := range tags {
		n += len(tag)
	}
	if cardString != "" {
		n += len(cardString) + len(cardSeparatorSymbol)
	}
	return n
}

func appendContextAndHash(buf []byte, name string, tags []string, cardString string) ([]byte, uint32) {
	h := init32

	buf, h = appendString32(buf, h, name)
	if len(tags) == 0 {
		if cardString == "" {
			return buf, h
		}
		buf, h = appendString32(buf, h, nameSeparatorSymbol)
		buf, h = appendString32(buf, h, cardString)
		return buf, h
	}

	buf, h = appendString32(buf, h, nameSeparatorSymbol)
	if cardString != "" {
		buf, h = appendString32(buf, h, cardString)
		buf, h = appendString32(buf, h, cardSeparatorSymbol)
	}
	buf, h = appendString32(buf, h, tags[0])
	for _, tag := range tags[1:] {
		buf, h = appendString32(buf, h, tagSeparatorSymbol)
		buf, h = appendString32(buf, h, tag)
	}
	return buf, h
}

func appendContext(buf []byte, name string, tags []string, cardString string) ([]byte, int) {
	tagsStart := -1

	buf = append(buf, name...)
	if len(tags) == 0 {
		if cardString == "" {
			return buf, tagsStart
		}
		buf = append(buf, nameSeparatorSymbol...)
		buf = append(buf, cardString...)
		return buf, tagsStart
	}

	buf = append(buf, nameSeparatorSymbol...)
	if cardString != "" {
		buf = append(buf, cardString...)
		buf = append(buf, cardSeparatorSymbol...)
	}
	tagsStart = len(buf)
	buf = append(buf, tags[0]...)
	for _, tag := range tags[1:] {
		buf = append(buf, tagSeparatorSymbol...)
		buf = append(buf, tag...)
	}
	return buf, tagsStart
}

func getShardIndexFromHash(shardsCount int, contextHash uint32) int {
	if shardsCount <= 1 {
		return 0
	}
	return int(contextHash % uint32(shardsCount))
}

func (a *aggregator) count(name string, value int64, tags []string, cardinality Cardinality) error {
	if len(tags) == 0 && cardinality == CardinalityNotSet {
		contextHash := uint32(0)
		if a.shardsCount > 1 {
			contextHash = hashString32(name)
		}
		return a.countWithStringContext(name, contextHash, name, value)
	}

	cardString := cardinality.String()
	contextLen := getContextLength(name, tags, cardString)
	if contextLen <= smallContextBufferSize {
		var contextBuffer [smallContextBufferSize]byte
		return a.countWithContextBuffer(contextBuffer[:0], name, value, tags, cardinality, cardString)
	}
	return a.countWithLargeContextBuffer(contextLen, name, value, tags, cardinality, cardString)
}

// countWithLargeContextBuffer keeps the 4 KiB stack array out of count's frame
// so the common small-context path is not penalized by the larger frame.
func (a *aggregator) countWithLargeContextBuffer(contextLen int, name string, value int64, tags []string, cardinality Cardinality, cardString string) error {
	if contextLen <= largeContextBufferSize {
		var contextBuffer [largeContextBufferSize]byte
		return a.countWithContextBuffer(contextBuffer[:0], name, value, tags, cardinality, cardString)
	}
	return a.countWithContextBuffer(make([]byte, 0, contextLen), name, value, tags, cardinality, cardString)
}

func (a *aggregator) countWithContextBuffer(contextBuffer []byte, name string, value int64, tags []string, cardinality Cardinality, cardString string) error {
	contextHash := uint32(0)
	if a.shardsCount > 1 {
		contextBuffer, contextHash = appendContextAndHash(contextBuffer, name, tags, cardString)
	} else {
		contextBuffer, _ = appendContext(contextBuffer, name, tags, cardString)
	}
	shard := &a.countShards[getShardIndexFromHash(a.shardsCount, contextHash)]
	shard.RLock()
	if count, found := shard.counts[string(contextBuffer)]; found {
		count.sample(value)
		shard.RUnlock()
		return nil
	}
	shard.RUnlock()

	metric := newCountMetric(name, value, tags, cardinality)

	shard.Lock()
	// Check if another goroutines hasn't created the value between the RUnlock and 'Lock'
	if count, found := shard.counts[string(contextBuffer)]; found {
		count.sample(value)
		shard.Unlock()
		return nil
	}

	if shard.counts == nil {
		shard.counts = countsMap{}
	}
	context := string(contextBuffer)
	shard.counts[context] = metric
	shard.Unlock()
	return nil
}

// handles the no-tags/no-cardinality fast path where the context key is the metric name itself.
func (a *aggregator) countWithStringContext(context string, contextHash uint32, name string, value int64) error {
	shard := &a.countShards[getShardIndexFromHash(a.shardsCount, contextHash)]
	shard.RLock()
	if count, found := shard.counts[context]; found {
		count.sample(value)
		shard.RUnlock()
		return nil
	}
	shard.RUnlock()

	metric := newCountMetric(name, value, nil, CardinalityNotSet)

	shard.Lock()
	// Check if another goroutines hasn't created the value between the RUnlock and 'Lock'
	if count, found := shard.counts[context]; found {
		count.sample(value)
		shard.Unlock()
		return nil
	}

	if shard.counts == nil {
		shard.counts = countsMap{}
	}
	shard.counts[context] = metric
	shard.Unlock()
	return nil
}

func (a *aggregator) gauge(name string, value float64, tags []string, cardinality Cardinality) error {
	if len(tags) == 0 && cardinality == CardinalityNotSet {
		contextHash := uint32(0)
		if a.shardsCount > 1 {
			contextHash = hashString32(name)
		}
		return a.gaugeWithStringContext(name, contextHash, name, value)
	}

	cardString := cardinality.String()
	contextLen := getContextLength(name, tags, cardString)
	if contextLen <= smallContextBufferSize {
		var contextBuffer [smallContextBufferSize]byte
		return a.gaugeWithContextBuffer(contextBuffer[:0], name, value, tags, cardinality, cardString)
	}
	return a.gaugeWithLargeContextBuffer(contextLen, name, value, tags, cardinality, cardString)
}

// gaugeWithLargeContextBuffer keeps the 4 KiB stack array out of gauge's frame
// so the common small-context path is not penalized by the larger frame.
func (a *aggregator) gaugeWithLargeContextBuffer(contextLen int, name string, value float64, tags []string, cardinality Cardinality, cardString string) error {
	if contextLen <= largeContextBufferSize {
		var contextBuffer [largeContextBufferSize]byte
		return a.gaugeWithContextBuffer(contextBuffer[:0], name, value, tags, cardinality, cardString)
	}
	return a.gaugeWithContextBuffer(make([]byte, 0, contextLen), name, value, tags, cardinality, cardString)
}

func (a *aggregator) gaugeWithContextBuffer(contextBuffer []byte, name string, value float64, tags []string, cardinality Cardinality, cardString string) error {
	contextHash := uint32(0)
	if a.shardsCount > 1 {
		contextBuffer, contextHash = appendContextAndHash(contextBuffer, name, tags, cardString)
	} else {
		contextBuffer, _ = appendContext(contextBuffer, name, tags, cardString)
	}
	shard := &a.gaugeShards[getShardIndexFromHash(a.shardsCount, contextHash)]
	shard.RLock()
	if gauge, found := shard.gauges[string(contextBuffer)]; found {
		gauge.sample(value)
		shard.RUnlock()
		return nil
	}
	shard.RUnlock()

	gauge := newGaugeMetric(name, value, tags, cardinality)

	shard.Lock()
	// Check if another goroutines hasn't created the value between the 'RUnlock' and 'Lock'
	if gauge, found := shard.gauges[string(contextBuffer)]; found {
		gauge.sample(value)
		shard.Unlock()
		return nil
	}
	if shard.gauges == nil {
		shard.gauges = gaugesMap{}
	}
	context := string(contextBuffer)
	shard.gauges[context] = gauge
	shard.Unlock()
	return nil
}

// handles the no-tags/no-cardinality fast path where the context key is the metric name itself.
func (a *aggregator) gaugeWithStringContext(context string, contextHash uint32, name string, value float64) error {
	shard := &a.gaugeShards[getShardIndexFromHash(a.shardsCount, contextHash)]
	shard.RLock()
	if gauge, found := shard.gauges[context]; found {
		gauge.sample(value)
		shard.RUnlock()
		return nil
	}
	shard.RUnlock()

	gauge := newGaugeMetric(name, value, nil, CardinalityNotSet)

	shard.Lock()
	// Check if another goroutines hasn't created the value between the 'RUnlock' and 'Lock'
	if gauge, found := shard.gauges[context]; found {
		gauge.sample(value)
		shard.Unlock()
		return nil
	}
	if shard.gauges == nil {
		shard.gauges = gaugesMap{}
	}
	shard.gauges[context] = gauge
	shard.Unlock()
	return nil
}

func (a *aggregator) set(name string, value string, tags []string, cardinality Cardinality) error {
	if len(tags) == 0 && cardinality == CardinalityNotSet {
		contextHash := uint32(0)
		if a.shardsCount > 1 {
			contextHash = hashString32(name)
		}
		return a.setWithStringContext(name, contextHash, name, value)
	}

	cardString := cardinality.String()
	contextLen := getContextLength(name, tags, cardString)
	if contextLen <= smallContextBufferSize {
		var contextBuffer [smallContextBufferSize]byte
		return a.setWithContextBuffer(contextBuffer[:0], name, value, tags, cardinality, cardString)
	}
	return a.setWithLargeContextBuffer(contextLen, name, value, tags, cardinality, cardString)
}

// setWithLargeContextBuffer keeps the 4 KiB stack array out of set's frame so
// the common small-context path is not penalized by the larger frame.
func (a *aggregator) setWithLargeContextBuffer(contextLen int, name string, value string, tags []string, cardinality Cardinality, cardString string) error {
	if contextLen <= largeContextBufferSize {
		var contextBuffer [largeContextBufferSize]byte
		return a.setWithContextBuffer(contextBuffer[:0], name, value, tags, cardinality, cardString)
	}
	return a.setWithContextBuffer(make([]byte, 0, contextLen), name, value, tags, cardinality, cardString)
}

func (a *aggregator) setWithContextBuffer(contextBuffer []byte, name string, value string, tags []string, cardinality Cardinality, cardString string) error {
	contextHash := uint32(0)
	if a.shardsCount > 1 {
		contextBuffer, contextHash = appendContextAndHash(contextBuffer, name, tags, cardString)
	} else {
		contextBuffer, _ = appendContext(contextBuffer, name, tags, cardString)
	}
	shard := &a.setShards[getShardIndexFromHash(a.shardsCount, contextHash)]
	shard.RLock()
	if set, found := shard.sets[string(contextBuffer)]; found {
		set.sample(value)
		shard.RUnlock()
		return nil
	}
	shard.RUnlock()

	metric := newSetMetric(name, value, tags, cardinality)

	shard.Lock()
	// Check if another goroutines hasn't created the value between the 'RUnlock' and 'Lock'
	if set, found := shard.sets[string(contextBuffer)]; found {
		set.sample(value)
		shard.Unlock()
		return nil
	}
	if shard.sets == nil {
		shard.sets = setsMap{}
	}
	context := string(contextBuffer)
	shard.sets[context] = metric
	shard.Unlock()
	return nil
}

// handles the no-tags/no-cardinality fast path where the context key is the metric name itself.
func (a *aggregator) setWithStringContext(context string, contextHash uint32, name string, value string) error {
	shard := &a.setShards[getShardIndexFromHash(a.shardsCount, contextHash)]
	shard.RLock()
	if set, found := shard.sets[context]; found {
		set.sample(value)
		shard.RUnlock()
		return nil
	}
	shard.RUnlock()

	metric := newSetMetric(name, value, nil, CardinalityNotSet)

	shard.Lock()
	// Check if another goroutines hasn't created the value between the 'RUnlock' and 'Lock'
	if set, found := shard.sets[context]; found {
		set.sample(value)
		shard.Unlock()
		return nil
	}
	if shard.sets == nil {
		shard.sets = setsMap{}
	}
	shard.sets[context] = metric
	shard.Unlock()
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

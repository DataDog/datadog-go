package statsd

import (
	"math/rand"
	"sync"
	"time"
)

type worker struct {
	pool       *bufferPool
	buffer     *statsdBuffer
	sender     *sender
	random     *rand.Rand
	randomLock sync.Mutex
	sync.Mutex

	inputMetrics chan metric
	stop         chan struct{}
}

func newWorker(pool *bufferPool, sender *sender) *worker {
	// Each worker uses its own random source and random lock to prevent
	// workers in separate goroutines from contending for the lock on the
	// "math/rand" package-global random source (e.g. calls like
	// "rand.Float64()" must acquire a shared lock to get the next
	// pseudorandom number).
	// Note that calling "time.Now().UnixNano()" repeatedly quickly may return
	// very similar values. That's fine for seeding the worker-specific random
	// source because we just need an evenly distributed stream of float values.
	// Do not use this random source for cryptographic randomness.
	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	return &worker{
		pool:   pool,
		sender: sender,
		buffer: pool.borrowBuffer(),
		random: random,
		stop:   make(chan struct{}),
	}
}

func (w *worker) startReceivingMetric(bufferSize int) {
	w.inputMetrics = make(chan metric, bufferSize)
	go w.pullMetric()
}

func (w *worker) stopReceivingMetric() {
	w.stop <- struct{}{}
}

func (w *worker) pullMetric() {
	for {
		select {
		case m := <-w.inputMetrics:
			w.processMetric(m)
		case <-w.stop:
			return
		}
	}
}

func (w *worker) processMetric(m metric) error {
	// Aggregated metrics are already sampled.
	if m.metricType != distributionAggregated && m.metricType != histogramAggregated && m.metricType != timingAggregated {
		if !shouldSample(m.rate, w.random, &w.randomLock) {
			return nil
		}
	}
	w.Lock()
	var err error
	if err = w.writeMetricUnsafe(m); err == errBufferFull {
		w.flushUnsafe()
		err = w.writeMetricUnsafe(m)
	}
	w.Unlock()
	return err
}

func (w *worker) writeAggregatedMetricUnsafe(m metric, metricSymbol []byte, precision int, rate float64) error {
	globalPos := 0

	// first check how much data we can write to the buffer:
	//   +3 + len(metricSymbol) because the message will include '|<metricSymbol>|#' before the tags
	//   +1 for the potential line break at the start of the metric
	extraSize := len(m.stags) + 4 + len(metricSymbol)
	if m.rate < 1 {
		// +2 for "|@"
		// + the maximum size of a rate (https://en.wikipedia.org/wiki/IEEE_754-1985)
		extraSize += 2 + 18
	}
	for _, t := range m.globalTags {
		extraSize += len(t) + 1
	}

	for {
		pos, err := w.buffer.writeAggregated(metricSymbol, m.namespace, m.globalTags, m.name, m.fvalues[globalPos:], m.stags, extraSize, precision, rate, m.originDetection, m.overrideCard)
		if err == errPartialWrite {
			// We successfully wrote part of the histogram metrics.
			// We flush the current buffer and finish the histogram
			// in a new one.
			w.flushUnsafe()
			globalPos += pos
		} else {
			return err
		}
	}
}

func (w *worker) writeMetricUnsafe(m metric) error {
	switch m.metricType {
	case gauge:
		return w.buffer.writeGauge(m.namespace, m.globalTags, m.name, m.fvalue, m.tags, m.rate, m.timestamp, m.originDetection, m.overrideCard)
	case count:
		return w.buffer.writeCount(m.namespace, m.globalTags, m.name, m.ivalue, m.tags, m.rate, m.timestamp, m.originDetection, m.overrideCard)
	case histogram:
		return w.buffer.writeHistogram(m.namespace, m.globalTags, m.name, m.fvalue, m.tags, m.rate, m.originDetection, m.overrideCard)
	case distribution:
		return w.buffer.writeDistribution(m.namespace, m.globalTags, m.name, m.fvalue, m.tags, m.rate, m.originDetection, m.overrideCard)
	case set:
		return w.buffer.writeSet(m.namespace, m.globalTags, m.name, m.svalue, m.tags, m.rate, m.originDetection, m.overrideCard)
	case timing:
		return w.buffer.writeTiming(m.namespace, m.globalTags, m.name, m.fvalue, m.tags, m.rate, m.originDetection, m.overrideCard)
	case event:
		return w.buffer.writeEvent(m.evalue, m.globalTags, m.originDetection, m.overrideCard)
	case serviceCheck:
		return w.buffer.writeServiceCheck(m.scvalue, m.globalTags, m.originDetection, m.overrideCard)
	case histogramAggregated:
		return w.writeAggregatedMetricUnsafe(m, histogramSymbol, -1, m.rate)
	case distributionAggregated:
		return w.writeAggregatedMetricUnsafe(m, distributionSymbol, -1, m.rate)
	case timingAggregated:
		return w.writeAggregatedMetricUnsafe(m, timingSymbol, 6, m.rate)
	default:
		return nil
	}
}

func (w *worker) flush() {
	w.Lock()
	w.flushUnsafe()
	w.Unlock()
}

func (w *worker) pause() {
	w.Lock()
}

func (w *worker) unpause() {
	w.Unlock()
}

// flush the current buffer. Lock must be held by caller.
// flushed buffer written to the network asynchronously.
func (w *worker) flushUnsafe() {
	if len(w.buffer.bytes()) > 0 {
		w.sender.send(w.buffer)
		w.buffer = w.pool.borrowBuffer()
	}
}

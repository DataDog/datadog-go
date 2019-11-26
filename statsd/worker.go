package statsd

import (
	"math/rand"
	"sync"
)

type worker struct {
	pool   *bufferPool
	buffer *statsdBuffer
	sender *sender
	sync.Mutex
}

func newWorker(pool *bufferPool, sender *sender) *worker {
	return &worker{
		pool:   pool,
		sender: sender,
		buffer: pool.borrowBuffer(),
	}
}

func (w *worker) processMetric(m metric) error {
	if w.shouldSample(m.rate) {
		return nil
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

func (w *worker) shouldSample(rate float64) bool {
	if rate < 1 && rand.Float64() > rate {
		return true
	}
	return false
}

func (w *worker) writeMetricUnsafe(m metric) error {
	switch m.metricType {
	case gauge:
		return w.buffer.writeGauge(m.namespace, m.globalTags, m.name, m.fvalue, m.tags, m.rate)
	case count:
		return w.buffer.writeCount(m.namespace, m.globalTags, m.name, m.ivalue, m.tags, m.rate)
	case histogram:
		return w.buffer.writeHistogram(m.namespace, m.globalTags, m.name, m.fvalue, m.tags, m.rate)
	case distribution:
		return w.buffer.writeDistribution(m.namespace, m.globalTags, m.name, m.fvalue, m.tags, m.rate)
	case set:
		return w.buffer.writeSet(m.namespace, m.globalTags, m.name, m.svalue, m.tags, m.rate)
	case timing:
		return w.buffer.writeTiming(m.namespace, m.globalTags, m.name, m.fvalue, m.tags, m.rate)
	case event:
		return w.buffer.writeEvent(*m.evalue, m.globalTags)
	case serviceCheck:
		return w.buffer.writeServiceCheck(*m.scvalue, m.globalTags)
	default:
		return nil
	}
}

func (w *worker) flush() {
	w.Lock()
	w.flushUnsafe()
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

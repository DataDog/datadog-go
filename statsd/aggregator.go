package statsd

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"github.com/twmb/murmur3"
)

const (
	contextKeySeparator = ","
)

type contextKey [2]uint64

type aggregator struct {
	keyBuffer   *bytes.Buffer
	metrics     map[contextKey]*aggregatedMetric
	worker      *worker
	flushTicker *time.Ticker
	stop        chan struct{}
	sync.Mutex
}

type aggregatedMetric struct {
	name       string
	tags       []string
	metricType metricType
	fvalue     float64
	ivalue     int64
}

func newAggregator(worker *worker, flushInterval time.Duration) *aggregator {
	aggregator := &aggregator{
		keyBuffer:   bytes.NewBuffer(make([]byte, 1024)),
		metrics:     make(map[contextKey]*aggregatedMetric),
		worker:      worker,
		flushTicker: time.NewTicker(flushInterval),
		stop:        make(chan struct{}),
	}
	go aggregator.flushLoop()
	return aggregator
}

func (a *aggregator) computeKey(name string, tags []string) contextKey {
	a.keyBuffer.Reset()
	a.keyBuffer.WriteString(name)
	a.keyBuffer.WriteString(contextKeySeparator)
	for i := 0; i < len(tags); i++ {
		a.keyBuffer.WriteString(tags[i])
		a.keyBuffer.WriteString(contextKeySeparator)
	}
	var key contextKey
	key[0], key[1] = murmur3.Sum128(a.keyBuffer.Bytes())
	return key
}

func (a *aggregator) addSample(sample metric) error {
	a.Lock()
	// compute the hash
	key := a.computeKey(sample.name, sample.tags)
	aggregate, exists := a.metrics[key]
	if !exists {
		aggregate = &aggregatedMetric{
			name:       sample.name,
			tags:       make([]string, len(sample.tags)),
			metricType: sample.metricType,
		}
		copy(aggregate.tags, sample.tags)
		a.metrics[key] = aggregate
	}
	err := aggregate.addSample(sample)
	a.Unlock()
	return err
}

func (am *aggregatedMetric) addSample(sample metric) error {
	if am.metricType != sample.metricType {
		return fmt.Errorf("metric named %s was sent with two different types", sample.name)
	}
	switch sample.metricType {
	case gauge:
		am.fvalue = sample.fvalue
	case count:
		am.ivalue += sample.ivalue
	default:
		return fmt.Errorf("could not aggregate unknown metric type with enum number: %d", sample.metricType)
	}
	return nil
}

func (a *aggregator) flushLoop() {
	for {
		select {
		case <-a.flushTicker.C:
			a.flush()
		case <-a.stop:
			a.flushTicker.Stop()
			return
		}
	}
}

func (a *aggregator) flush() {
	newMetrics := make(map[contextKey]*aggregatedMetric)
	a.Lock()
	flushedMetrics := a.metrics
	a.metrics = newMetrics
	a.Unlock()
	for _, m := range flushedMetrics {
		a.worker.processMetric(metric{
			name:       m.name,
			tags:       m.tags,
			metricType: m.metricType,
			rate:       1,
		})
	}
	a.worker.flush()
}

func canAggregate(sample metric) bool {
	return (sample.metricType == gauge || sample.metricType == count) && sample.rate == 1
}

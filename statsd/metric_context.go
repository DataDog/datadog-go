package statsd

import (
	"sync/atomic"
	"time"
)

// MetricContext is a reusable, precomputed metric context: a metric name, a set
// of tags and a tag cardinality. Building the aggregation key for a metric means
// copying the name and every tag into a single buffer (the memmove seen in
// profiles) and hashing it. A MetricContext does that work once, when it is
// created, and reuses the result for every subsequent sample of the same series.
//
// Reuse a MetricContext for series that are sampled repeatedly (the common case
// for counters, gauges and timers on a hot path):
//
//	ctx := client.NewMetricContext("requests", []string{"endpoint:/health"})
//	for {
//	    ctx.Incr()
//	}
//
// A MetricContext is bound to the Client (or ClientEx) that created it and is
// safe for concurrent use by multiple goroutines.
//
// The context only accelerates the aggregated code paths, enabled with
// WithClientSideAggregation (count/gauge/set) and WithExtendedClientSideAggregation
// (histogram/distribution/timing). When the relevant aggregation is disabled the
// sample methods fall back to the regular Client methods and rebuild the payload
// on every call, sampling at rate 1.
type MetricContext struct {
	client *ClientEx

	name        string
	tags        []string
	cardinality Cardinality

	// key is the prebuilt context key used for map lookups. hash is the FNV-1a
	// hash of key used to pick the aggregator shard. tagsStart is the offset of
	// the tags substring inside key, or -1 when there are no tags. stringTags is
	// key[tagsStart:] (or "") and is stored when a new buffered metric is created.
	key        string
	hash       uint32
	tagsStart  int
	stringTags string
}

// NewMetricContext builds a reusable MetricContext for the given metric name,
// tags and (optional) tag cardinality. The tags slice is copied, so the caller
// is free to mutate or reuse it afterwards.
func (c *ClientEx) NewMetricContext(name string, tags []string, parameters ...Parameter) *MetricContext {
	if c == nil {
		return &MetricContext{}
	}

	cardinality := parameterCardinality(parameters, c.defaultCardinality)
	cardString := cardinality.String()

	contextLen := getContextLength(name, tags, cardString)
	buf, hash := appendContextAndHash(make([]byte, 0, contextLen), name, tags, cardString)
	key := string(buf)

	// tagsStart mirrors the offset computed by appendContext so that
	// key[tagsStart:] is exactly the tags portion of the key.
	tagsStart := -1
	stringTags := ""
	if len(tags) > 0 {
		tagsStart = len(name) + len(nameSeparatorSymbol)
		if cardString != "" {
			tagsStart += len(cardString) + len(cardSeparatorSymbol)
		}
		stringTags = key[tagsStart:]
	}

	return &MetricContext{
		client:      c,
		name:        name,
		tags:        copySlice(tags),
		cardinality: cardinality,
		key:         key,
		hash:        hash,
		tagsStart:   tagsStart,
		stringTags:  stringTags,
	}
}

// NewMetricContext builds a reusable MetricContext bound to this Client. See
// (*ClientEx).NewMetricContext for details.
func (c *Client) NewMetricContext(name string, tags []string, parameters ...Parameter) *MetricContext {
	if c == nil {
		return &MetricContext{}
	}
	return c.clientEx.NewMetricContext(name, tags, parameters...)
}

// Count tracks how many times something happened per second.
func (mc *MetricContext) Count(value int64) error {
	if mc == nil || mc.client == nil {
		return ErrNoClient
	}
	c := mc.client
	atomic.AddUint64(&c.telemetry.totalMetricsCount, 1)
	if c.agg != nil {
		return c.agg.countPrebuiltContext(mc.key, mc.hash, mc.name, value, mc.tags, mc.cardinality)
	}
	return c.send(metric{metricType: count, name: mc.name, ivalue: value, tags: mc.tags, rate: 1, globalTags: c.tags, namespace: c.namespace, originDetection: c.originDetection, cardinality: mc.cardinality})
}

// Incr is just Count of 1.
func (mc *MetricContext) Incr() error {
	return mc.Count(1)
}

// Decr is just Count of -1.
func (mc *MetricContext) Decr() error {
	return mc.Count(-1)
}

// Gauge measures the value of a metric at a particular time.
func (mc *MetricContext) Gauge(value float64) error {
	if mc == nil || mc.client == nil {
		return ErrNoClient
	}
	c := mc.client
	atomic.AddUint64(&c.telemetry.totalMetricsGauge, 1)
	if c.agg != nil {
		return c.agg.gaugePrebuiltContext(mc.key, mc.hash, mc.name, value, mc.tags, mc.cardinality)
	}
	return c.send(metric{metricType: gauge, name: mc.name, fvalue: value, tags: mc.tags, rate: 1, globalTags: c.tags, namespace: c.namespace, originDetection: c.originDetection, cardinality: mc.cardinality})
}

// Set counts the number of unique elements in a group.
func (mc *MetricContext) Set(value string) error {
	if mc == nil || mc.client == nil {
		return ErrNoClient
	}
	c := mc.client
	atomic.AddUint64(&c.telemetry.totalMetricsSet, 1)
	if c.agg != nil {
		return c.agg.setPrebuiltContext(mc.key, mc.hash, mc.name, value, mc.tags, mc.cardinality)
	}
	return c.send(metric{metricType: set, name: mc.name, svalue: value, tags: mc.tags, rate: 1, globalTags: c.tags, namespace: c.namespace, originDetection: c.originDetection, cardinality: mc.cardinality})
}

// Histogram tracks the statistical distribution of a set of values on each host.
func (mc *MetricContext) Histogram(value float64, rate float64) error {
	if mc == nil || mc.client == nil {
		return ErrNoClient
	}
	c := mc.client
	atomic.AddUint64(&c.telemetry.totalMetricsHistogram, 1)
	if c.aggExtended != nil {
		return c.sampleBufferedContext(histogram, mc, value, rate)
	}
	return c.send(metric{metricType: histogram, name: mc.name, fvalue: value, tags: mc.tags, rate: rate, globalTags: c.tags, namespace: c.namespace, originDetection: c.originDetection, cardinality: mc.cardinality})
}

// Distribution tracks the statistical distribution of a set of values across your infrastructure.
func (mc *MetricContext) Distribution(value float64, rate float64) error {
	if mc == nil || mc.client == nil {
		return ErrNoClient
	}
	c := mc.client
	atomic.AddUint64(&c.telemetry.totalMetricsDistribution, 1)
	if c.aggExtended != nil {
		return c.sampleBufferedContext(distribution, mc, value, rate)
	}
	return c.send(metric{metricType: distribution, name: mc.name, fvalue: value, tags: mc.tags, rate: rate, globalTags: c.tags, namespace: c.namespace, originDetection: c.originDetection, cardinality: mc.cardinality})
}

// Timing sends timing information, it is an alias for TimeInMilliseconds.
func (mc *MetricContext) Timing(value time.Duration, rate float64) error {
	return mc.TimeInMilliseconds(value.Seconds()*1000, rate)
}

// TimeInMilliseconds sends timing information in milliseconds.
func (mc *MetricContext) TimeInMilliseconds(value float64, rate float64) error {
	if mc == nil || mc.client == nil {
		return ErrNoClient
	}
	c := mc.client
	atomic.AddUint64(&c.telemetry.totalMetricsTiming, 1)
	if c.aggExtended != nil {
		return c.sampleBufferedContext(timing, mc, value, rate)
	}
	return c.send(metric{metricType: timing, name: mc.name, fvalue: value, tags: mc.tags, rate: rate, globalTags: c.tags, namespace: c.namespace, originDetection: c.originDetection, cardinality: mc.cardinality})
}

// sampleBufferedContext samples a buffered metric (histogram/distribution/timing)
// through the extended aggregator using a prebuilt context. It mirrors
// sendToAggregator's channelMode / mutexMode handling.
func (c *ClientEx) sampleBufferedContext(mType metricType, mc *MetricContext, value float64, rate float64) error {
	agg := c.aggExtended
	if c.aggregatorMode == channelMode {
		m := metric{
			metricType:       mType,
			name:             mc.name,
			fvalue:           value,
			rate:             rate,
			cardinality:      mc.cardinality,
			prebuilt:         true,
			prebuiltContext:  mc.key,
			prebuiltTagStart: mc.tagsStart,
		}
		input := agg.inputMetrics[mc.hash%uint32(len(agg.inputMetrics))]
		select {
		case input <- m:
		default:
			atomic.AddUint64(&c.telemetry.totalDroppedOnReceive, 1)
			err := &ErrorInputChannelFull{m, len(input), "Aggregator input channel full"}
			if c.errorHandler != nil {
				c.errorHandler(err)
			}
			if c.errorOnBlockedChannel {
				return err
			}
		}
		return nil
	}
	return agg.samplePrebuiltBuffered(mType, mc.key, mc.tagsStart, mc.name, value, rate, mc.cardinality)
}

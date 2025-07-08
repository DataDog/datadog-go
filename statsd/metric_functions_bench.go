package statsd

import (
	"sync/atomic"
	"time"
)

func (c *Client) Gauge_Bench(name string, value float64, tags []string, rate float64, parameters ...Parameter) error {
	if c == nil {
		return ErrNoClient
	}
	atomic.AddUint64(&c.telemetry.totalMetricsGauge, 1)
	if c.agg != nil {
		return c.agg.gauge(name, value, tags)
	}

	// If the user has provided a cardinality parameter, the user-provided value will override the global setting.
	var cardinality = tagCardinality
	for _, o := range parameters {
		c, ok := o.(CardinalityParameter)
		if ok {
			cardinality = c
		}
	}
	return c.send(metric{metricType: gauge, name: name, fvalue: value, tags: tags, rate: rate, globalTags: c.tags, namespace: c.namespace, tagCardinality: cardinality})
}

// GaugeWithTimestamp measures the value of a metric at a given time.
// BETA - Please contact our support team for more information to use this feature: https://www.datadoghq.com/support/
// The value will bypass any aggregation on the client side and agent side, this is
// useful when sending points in the past.
//
// Minimum Datadog Agent version: 7.40.0
func (c *Client) GaugeWithTimestamp_Bench(name string, value float64, tags []string, rate float64, timestamp time.Time) error {
	if c == nil {
		return ErrNoClient
	}

	if timestamp.IsZero() || timestamp.Unix() <= noTimestamp {
		return InvalidTimestamp
	}

	atomic.AddUint64(&c.telemetry.totalMetricsGauge, 1)
	return c.send(metric{metricType: gauge, name: name, fvalue: value, tags: tags, rate: rate, globalTags: c.tags, namespace: c.namespace, timestamp: timestamp.Unix()})
}

// Count tracks how many times something happened per second.
func (c *Client) Count_Bench(name string, value int64, tags []string, rate float64) error {
	if c == nil {
		return ErrNoClient
	}
	atomic.AddUint64(&c.telemetry.totalMetricsCount, 1)
	if c.agg != nil {
		return c.agg.count(name, value, tags)
	}
	return c.send(metric{metricType: count, name: name, ivalue: value, tags: tags, rate: rate, globalTags: c.tags, namespace: c.namespace})
}

// CountWithTimestamp tracks how many times something happened at the given second.
// BETA - Please contact our support team for more information to use this feature: https://www.datadoghq.com/support/
// The value will bypass any aggregation on the client side and agent side, this is
// useful when sending points in the past.
//
// Minimum Datadog Agent version: 7.40.0
func (c *Client) CountWithTimestamp_Bench(name string, value int64, tags []string, rate float64, timestamp time.Time) error {
	if c == nil {
		return ErrNoClient
	}

	if timestamp.IsZero() || timestamp.Unix() <= noTimestamp {
		return InvalidTimestamp
	}

	atomic.AddUint64(&c.telemetry.totalMetricsCount, 1)
	return c.send(metric{metricType: count, name: name, ivalue: value, tags: tags, rate: rate, globalTags: c.tags, namespace: c.namespace, timestamp: timestamp.Unix()})
}

// Histogram tracks the statistical distribution of a set of values on each host.
func (c *Client) Histogram_Bench(name string, value float64, tags []string, rate float64) error {
	if c == nil {
		return ErrNoClient
	}
	atomic.AddUint64(&c.telemetry.totalMetricsHistogram, 1)
	if c.aggExtended != nil {
		return c.sendToAggregator(histogram, name, value, tags, rate, c.aggExtended.histogram)
	}
	return c.send(metric{metricType: histogram, name: name, fvalue: value, tags: tags, rate: rate, globalTags: c.tags, namespace: c.namespace})
}

// Distribution tracks the statistical distribution of a set of values across your infrastructure.
func (c *Client) Distribution_Bench(name string, value float64, tags []string, rate float64) error {
	if c == nil {
		return ErrNoClient
	}
	atomic.AddUint64(&c.telemetry.totalMetricsDistribution, 1)
	if c.aggExtended != nil {
		return c.sendToAggregator(distribution, name, value, tags, rate, c.aggExtended.distribution)
	}
	return c.send(metric{metricType: distribution, name: name, fvalue: value, tags: tags, rate: rate, globalTags: c.tags, namespace: c.namespace})
}

// Decr is just Count of -1
func (c *Client) Decr_Bench(name string, tags []string, rate float64) error {
	return c.Count(name, -1, tags, rate)
}

// Incr is just Count of 1
func (c *Client) Incr_Bench(name string, tags []string, rate float64) error {
	return c.Count(name, 1, tags, rate)
}

// Set counts the number of unique elements in a group.
func (c *Client) Set_Bench(name string, value string, tags []string, rate float64) error {
	if c == nil {
		return ErrNoClient
	}
	atomic.AddUint64(&c.telemetry.totalMetricsSet, 1)
	if c.agg != nil {
		return c.agg.set(name, value, tags)
	}
	return c.send(metric{metricType: set, name: name, svalue: value, tags: tags, rate: rate, globalTags: c.tags, namespace: c.namespace})
}

// Timing sends timing information, it is an alias for TimeInMilliseconds
func (c *Client) Timing_Bench(name string, value time.Duration, tags []string, rate float64) error {
	return c.TimeInMilliseconds(name, value.Seconds()*1000, tags, rate)
}

// TimeInMilliseconds sends timing information in milliseconds.
// It is flushed by statsd with percentiles, mean and other info (https://github.com/etsy/statsd/blob/master/docs/metric_types.md#timing)
func (c *Client) TimeInMilliseconds_Bench(name string, value float64, tags []string, rate float64) error {
	if c == nil {
		return ErrNoClient
	}
	atomic.AddUint64(&c.telemetry.totalMetricsTiming, 1)
	if c.aggExtended != nil {
		return c.sendToAggregator(timing, name, value, tags, rate, c.aggExtended.timing)
	}
	return c.send(metric{metricType: timing, name: name, fvalue: value, tags: tags, rate: rate, globalTags: c.tags, namespace: c.namespace})
}

// Event sends the provided Event.
func (c *Client) Event_Bench(e *Event) error {
	if c == nil {
		return ErrNoClient
	}
	atomic.AddUint64(&c.telemetry.totalEvents, 1)
	return c.send(metric{metricType: event, evalue: e, rate: 1, globalTags: c.tags, namespace: c.namespace})
}

// SimpleEvent sends an event with the provided title and text.
func (c *Client) SimpleEvent_Bench(title, text string) error {
	e := NewEvent(title, text)
	return c.Event(e)
}

// ServiceCheck sends the provided ServiceCheck.
func (c *Client) ServiceCheck_Bench(sc *ServiceCheck) error {
	if c == nil {
		return ErrNoClient
	}
	atomic.AddUint64(&c.telemetry.totalServiceChecks, 1)
	return c.send(metric{metricType: serviceCheck, scvalue: sc, rate: 1, globalTags: c.tags, namespace: c.namespace})
}

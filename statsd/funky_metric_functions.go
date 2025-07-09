package statsd

import (
	"sync/atomic"
)

func (c *Client) GaugeFunky(name string, value float64, tags []string, rate float64, parameters ...Parameter) error {
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

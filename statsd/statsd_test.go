package statsd

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type statsdWriterWrapper struct{}

func (statsdWriterWrapper) SetWriteTimeout(time.Duration) error {
	return nil
}

func (statsdWriterWrapper) Close() error {
	return nil
}

func (statsdWriterWrapper) Write(p []byte) (n int, err error) {
	return 0, nil
}

func TestCustomWriterBufferConfiguration(t *testing.T) {
	client, err := NewWithWriter(statsdWriterWrapper{})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	assert.Equal(t, OptimalUDPPayloadSize, client.bufferPool.bufferMaxSize)
	assert.Equal(t, DefaultUDPBufferPoolSize, cap(client.bufferPool.pool))
	assert.Equal(t, DefaultUDPBufferPoolSize, cap(client.sender.queue))
}

func TestTelemetry(t *testing.T) {
	// disabling autoflush of the telemetry
	client, err := New("localhost:8125", WithoutTelemetry())
	if err != nil {
		t.Fatal(err)
	}

	client.Gauge("Gauge", 21, nil, 1)
	client.Count("Count", 21, nil, 1)
	client.Histogram("Histogram", 21, nil, 1)
	client.Distribution("Distribution", 21, nil, 1)
	client.Decr("Decr", nil, 1)
	client.Incr("Incr", nil, 1)
	client.Set("Set", "value", nil, 1)
	client.Timing("Timing", 21, nil, 1)
	client.TimeInMilliseconds("TimeInMilliseconds", 21, nil, 1)
	client.SimpleEvent("hello", "world")
	client.SimpleServiceCheck("hello", Warn)

	metrics := client.flushTelemetry()

	expectedMetricsName := map[string]int64{
		"datadog.dogstatsd.client.metrics":                9,
		"datadog.dogstatsd.client.events":                 1,
		"datadog.dogstatsd.client.service_checks":         1,
		"datadog.dogstatsd.client.packets_sent":           0,
		"datadog.dogstatsd.client.bytes_sent":             0,
		"datadog.dogstatsd.client.packets_dropped":        0,
		"datadog.dogstatsd.client.bytes_dropped":          0,
		"datadog.dogstatsd.client.packets_dropped_queue":  0,
		"datadog.dogstatsd.client.bytes_dropped_queue":    0,
		"datadog.dogstatsd.client.packets_dropped_writer": 0,
		"datadog.dogstatsd.client.bytes_dropped_writer":   0,
	}

	telemetryTags := []string{clientTelemetryTag, clientVersionTelemetryTag, "client_transport:udp"}

	assert.Equal(t, len(expectedMetricsName), len(metrics))
	for _, m := range metrics {
		expectedValue, found := expectedMetricsName[m.name]
		if !found {
			assert.Fail(t, fmt.Sprintf("Unknown metrics: %s", m.name))
		}

		assert.Equal(t, expectedValue, m.ivalue, fmt.Sprintf("wrong ivalue for '%s'", m.name))
		assert.Equal(t, count, m.metricType, fmt.Sprintf("wrong metricTypefor '%s'", m.name))
		assert.Equal(t, telemetryTags, m.tags, fmt.Sprintf("wrong tags for '%s'", m.name))
		assert.Equal(t, float64(1), m.rate, fmt.Sprintf("wrong rate for '%s'", m.name))
	}
}

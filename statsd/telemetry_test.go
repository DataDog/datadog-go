package statsd

import (
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var basicExpectedTags = []string{clientTelemetryTag, clientVersionTelemetryTag, "client_transport:test_transport"}
var basicExpectedMetrics = map[string]int64{
	"datadog.dogstatsd.client.metrics":                   9,
	"datadog.dogstatsd.client.events":                    1,
	"datadog.dogstatsd.client.service_checks":            1,
	"datadog.dogstatsd.client.metric_dropped_on_receive": 0,
	"datadog.dogstatsd.client.packets_sent":              0,
	"datadog.dogstatsd.client.bytes_sent":                0,
	"datadog.dogstatsd.client.packets_dropped":           0,
	"datadog.dogstatsd.client.bytes_dropped":             0,
	"datadog.dogstatsd.client.packets_dropped_queue":     0,
	"datadog.dogstatsd.client.bytes_dropped_queue":       0,
	"datadog.dogstatsd.client.packets_dropped_writer":    0,
	"datadog.dogstatsd.client.bytes_dropped_writer":      0,
}

var devModeExpectedMetrics = map[string]int64{
	"datadog.dogstatsd.client.metricsGauge":        1,
	"datadog.dogstatsd.client.metricsCount":        3,
	"datadog.dogstatsd.client.metricsHistogram":    1,
	"datadog.dogstatsd.client.metricsDistribution": 1,
	"datadog.dogstatsd.client.metricsSet":          1,
	"datadog.dogstatsd.client.metricsTiming":       2,
}

var devModeAggregationExpectedMetrics = map[string]int64{
	"datadog.dogstatsd.client.aggregated_context_gauge": 1,
	"datadog.dogstatsd.client.aggregated_context_set":   1,
	"datadog.dogstatsd.client.aggregated_context_count": 3,
}

func TestNewTelemetry(t *testing.T) {
	client, err := New("localhost:8125", WithoutTelemetry(), WithNamespace("test_namespace"))
	require.Nil(t, err)

	telemetry := NewTelemetryClient(client, "test_transport", false)
	assert.NotNil(t, telemetry)

	assert.Equal(t, telemetry.c, client)
	assert.Equal(t, telemetry.tags, basicExpectedTags)
	assert.Nil(t, telemetry.sender)
	assert.Nil(t, telemetry.worker)
}

func submitTestMetrics(c *Client) {
	c.Gauge("Gauge", 21, nil, 1)
	c.Count("Count", 21, nil, 1)
	c.Histogram("Histogram", 21, nil, 1)
	c.Distribution("Distribution", 21, nil, 1)
	c.Decr("Decr", nil, 1)
	c.Incr("Incr", nil, 1)
	c.Set("Set", "value", nil, 1)
	c.Timing("Timing", 21, nil, 1)
	c.TimeInMilliseconds("TimeInMilliseconds", 21, nil, 1)
	c.SimpleEvent("hello", "world")
	c.SimpleServiceCheck("hello", Warn)
}

func testTelemetry(t *testing.T, telemetry *telemetryClient, expectedMetrics map[string]int64, expectedTelemetryTags []string) {
	assert.NotNil(t, telemetry)

	submitTestMetrics(telemetry.c)
	if telemetry.c.agg != nil {
		telemetry.c.agg.sendMetrics()
	}
	metrics := telemetry.flush()

	assert.Equal(t, len(expectedMetrics), len(metrics))
	for _, m := range metrics {
		expectedValue, found := expectedMetrics[m.name]
		assert.True(t, found, fmt.Sprintf("Unknown metrics: %s", m.name))

		assert.Equal(t, expectedValue, m.ivalue, fmt.Sprintf("wrong ivalue for '%s'", m.name))
		assert.Equal(t, count, m.metricType, fmt.Sprintf("wrong metricTypefor '%s'", m.name))
		assert.Equal(t, expectedTelemetryTags, m.tags, fmt.Sprintf("wrong tags for '%s'", m.name))
		assert.Equal(t, float64(1), m.rate, fmt.Sprintf("wrong rate for '%s'", m.name))
	}
}

func TestTelemetry(t *testing.T) {
	// disabling autoflush of the telemetry
	client, err := New("localhost:8125", WithoutTelemetry())
	require.Nil(t, err)

	telemetry := NewTelemetryClient(client, "test_transport", false)
	testTelemetry(t, telemetry, basicExpectedMetrics, basicExpectedTags)
}

func TestTelemetryDevMode(t *testing.T) {
	// disabling autoflush of the telemetry
	client, err := New("localhost:8125", WithoutTelemetry(), WithDevMode())
	require.Nil(t, err)

	expectedMetrics := map[string]int64{}
	for k, v := range basicExpectedMetrics {
		expectedMetrics[k] = v
	}
	for k, v := range devModeExpectedMetrics {
		expectedMetrics[k] = v
	}

	telemetry := NewTelemetryClient(client, "test_transport", true)
	testTelemetry(t, telemetry, expectedMetrics, basicExpectedTags)
}

func TestTelemetryChannelMode(t *testing.T) {
	// disabling autoflush of the telemetry
	client, err := New("localhost:8125", WithoutTelemetry(), WithChannelMode())
	require.Nil(t, err)

	telemetry := NewTelemetryClient(client, "test_transport", false)
	testTelemetry(t, telemetry, basicExpectedMetrics, basicExpectedTags)
}

func TestTelemetryWithGlobalTags(t *testing.T) {
	os.Setenv("DD_ENV", "test")
	defer os.Unsetenv("DD_ENV")

	// disabling autoflush of the telemetry
	client, err := New("localhost:8125", WithoutTelemetry(), WithTags([]string{"tag1", "tag2"}))
	require.Nil(t, err)

	telemetry := NewTelemetryClient(client, "test_transport", false)

	expectedTelemetryTags := append([]string{"tag1", "tag2", "env:test"}, basicExpectedTags...)
	testTelemetry(t, telemetry, basicExpectedMetrics, expectedTelemetryTags)
}

func TestTelemetryWithAggregation(t *testing.T) {
	// disabling autoflush of the telemetry
	client, err := New("localhost:8125", WithoutTelemetry(), WithClientSideAggregation())
	require.Nil(t, err)

	telemetry := NewTelemetryClient(client, "test_transport", false)

	expectedMetrics := map[string]int64{
		"datadog.dogstatsd.client.aggregated_context": 5,
	}
	for k, v := range basicExpectedMetrics {
		expectedMetrics[k] = v
	}

	testTelemetry(t, telemetry, expectedMetrics, basicExpectedTags)
}

func TestTelemetryWithAggregationDevMode(t *testing.T) {
	// disabling autoflush of the telemetry
	client, err := New("localhost:8125", WithoutTelemetry(), WithClientSideAggregation(), WithDevMode())
	require.Nil(t, err)

	telemetry := NewTelemetryClient(client, "test_transport", true)

	expectedMetrics := map[string]int64{
		"datadog.dogstatsd.client.aggregated_context": 5,
	}
	for k, v := range basicExpectedMetrics {
		expectedMetrics[k] = v
	}
	for k, v := range devModeExpectedMetrics {
		expectedMetrics[k] = v
	}
	for k, v := range devModeAggregationExpectedMetrics {
		expectedMetrics[k] = v
	}

	testTelemetry(t, telemetry, expectedMetrics, basicExpectedTags)
}

func TestTelemetryCustomAddr(t *testing.T) {
	buffer := make([]byte, 4096)
	addr := "localhost:1201"
	server := getTestServer(t, addr)
	defer server.Close()

	client, err := New("localhost:9876", WithTelemetryAddr(addr), WithNamespace("test_namespace"))
	require.Nil(t, err, fmt.Sprintf("failed to create client: %s", err))
	readDone := make(chan struct{})
	n := 0
	go func() {
		n, _ = io.ReadAtLeast(server, buffer, 1)
		close(readDone)
	}()

	submitTestMetrics(client)
	client.telemetry.sendTelemetry()

	select {
	case <-readDone:
	case <-time.After(2 * time.Second):
		require.Fail(t, "No data was flush on Close")
	}

	result := string(buffer[:n])

	expectedPayload := "datadog.dogstatsd.client.metrics:9|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp\n" +
		"datadog.dogstatsd.client.events:1|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp\n" +
		"datadog.dogstatsd.client.service_checks:1|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp\n" +
		"datadog.dogstatsd.client.metric_dropped_on_receive:0|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp\n" +
		"datadog.dogstatsd.client.packets_sent:0|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp\n" +
		"datadog.dogstatsd.client.bytes_sent:0|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp\n" +
		"datadog.dogstatsd.client.packets_dropped:0|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp\n" +
		"datadog.dogstatsd.client.bytes_dropped:0|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp\n" +
		"datadog.dogstatsd.client.packets_dropped_queue:0|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp\n" +
		"datadog.dogstatsd.client.bytes_dropped_queue:0|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp\n" +
		"datadog.dogstatsd.client.packets_dropped_writer:0|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp\n" +
		"datadog.dogstatsd.client.bytes_dropped_writer:0|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp"
	assert.Equal(t, expectedPayload, result)
}

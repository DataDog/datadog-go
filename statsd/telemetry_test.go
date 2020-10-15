package statsd

import (
	"fmt"
	"io"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var basicExpectedTags = []string{clientTelemetryTag, clientVersionTelemetryTag, "client_transport:test_transport"}

func appendBasicMetrics(metrics []metric, tags []string) []metric {
	basicExpectedMetrics := map[string]int64{
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

	for name, value := range basicExpectedMetrics {
		metrics = append(metrics, metric{
			name:       name,
			ivalue:     value,
			metricType: count,
			tags:       append(tags, basicExpectedTags...),
			rate:       float64(1),
		})
	}
	return metrics
}

func appendAggregationMetrics(metrics []metric, tags []string, devMode bool, extendedAggregation bool) []metric {
	if extendedAggregation {
		metrics = append(metrics, metric{
			name:       "datadog.dogstatsd.client.aggregated_context",
			ivalue:     9,
			metricType: count,
			tags:       append(tags, basicExpectedTags...),
			rate:       float64(1),
		})
	} else {
		metrics = append(metrics, metric{
			name:       "datadog.dogstatsd.client.aggregated_context",
			ivalue:     5,
			metricType: count,
			tags:       append(tags, basicExpectedTags...),
			rate:       float64(1),
		})
	}

	if devMode {
		contextByTypeName := "datadog.dogstatsd.client.aggregated_context_by_type"
		devModeAggregationExpectedMetrics := map[string]int64{
			"metrics_type:gauge":        1,
			"metrics_type:set":          1,
			"metrics_type:count":        3,
			"metrics_type:histogram":    0,
			"metrics_type:distribution": 0,
			"metrics_type:timing":       0,
		}
		if extendedAggregation {
			devModeAggregationExpectedMetrics["metrics_type:histogram"] = 1
			devModeAggregationExpectedMetrics["metrics_type:distribution"] = 1
			devModeAggregationExpectedMetrics["metrics_type:timing"] = 2
		}

		for typeTag, value := range devModeAggregationExpectedMetrics {
			metrics = append(metrics, metric{
				name:       contextByTypeName,
				ivalue:     value,
				metricType: count,
				tags:       append(tags, append(basicExpectedTags, typeTag)...),
				rate:       float64(1),
			})
		}
	}
	return metrics
}

func appendDevModeMetrics(metrics []metric, tags []string) []metric {
	metricByTypeName := "datadog.dogstatsd.client.metrics_by_type"
	devModeExpectedMetrics := map[string]int64{
		"metrics_type:gauge":        1,
		"metrics_type:count":        3,
		"metrics_type:set":          1,
		"metrics_type:timing":       2,
		"metrics_type:histogram":    1,
		"metrics_type:distribution": 1,
	}

	for typeTag, value := range devModeExpectedMetrics {
		metrics = append(metrics, metric{
			name:       metricByTypeName,
			ivalue:     value,
			metricType: count,
			tags:       append(tags, append(basicExpectedTags, typeTag)...),
			rate:       float64(1),
		})
	}
	return metrics
}

func TestNewTelemetry(t *testing.T) {
	client, err := New("localhost:8125", WithoutTelemetry(), WithNamespace("test_namespace"))
	require.Nil(t, err)

	telemetry := newTelemetryClient(client, "test_transport", false)
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

type metricSorted []metric

func (s metricSorted) Len() int      { return len(s) }
func (s metricSorted) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

func (s metricSorted) Less(i, j int) bool {
	if s[i].name == s[j].name {
		if len(s[i].tags) != len(s[j].tags) {
			return len(s[i].tags) < len(s[j].tags)
		}

		sort.Strings(s[i].tags)
		sort.Strings(s[j].tags)

		for idx := range s[i].tags {
			if s[i].tags[idx] != s[j].tags[idx] {
				return s[i].tags[idx] < s[j].tags[idx]
			}
		}
		return false
	}
	return s[i].name < s[j].name
}

func testTelemetry(t *testing.T, telemetry *telemetryClient, expectedMetrics []metric) {
	assert.NotNil(t, telemetry)

	submitTestMetrics(telemetry.c)
	if telemetry.c.agg != nil {
		telemetry.c.agg.sendMetrics()
	}
	metrics := telemetry.flush()

	require.Equal(t, len(expectedMetrics), len(metrics), fmt.Sprintf("expected:\n%v\nactual:\n%v", expectedMetrics, metrics))

	sort.Sort(metricSorted(metrics))
	sort.Sort(metricSorted(expectedMetrics))

	for idx := range metrics {
		m := metrics[idx]
		expected := expectedMetrics[idx]

		assert.Equal(t, expected.ivalue, m.ivalue, fmt.Sprintf("wrong ivalue for '%s' with tags '%v'", m.name, m.tags))
		assert.Equal(t, expected.metricType, m.metricType, fmt.Sprintf("wrong metricType for '%s'", m.name))

		assert.Equal(t, expected.tags, m.tags, fmt.Sprintf("wrong tags for '%s'", m.name))
		assert.Equal(t, expected.rate, m.rate, fmt.Sprintf("wrong rate for '%s'", m.name))
	}
}

func TestTelemetry(t *testing.T) {
	// disabling autoflush of the telemetry
	client, err := New("localhost:8125", WithoutTelemetry())
	require.Nil(t, err)

	expectedMetrics := []metric{}
	expectedMetrics = appendBasicMetrics(expectedMetrics, nil)

	telemetry := newTelemetryClient(client, "test_transport", false)
	testTelemetry(t, telemetry, expectedMetrics)
}

func TestTelemetryDevMode(t *testing.T) {
	// disabling autoflush of the telemetry
	client, err := New("localhost:8125", WithoutTelemetry(), WithDevMode())
	require.Nil(t, err)

	expectedMetrics := []metric{}
	expectedMetrics = appendBasicMetrics(expectedMetrics, nil)
	expectedMetrics = appendDevModeMetrics(expectedMetrics, nil)

	telemetry := newTelemetryClient(client, "test_transport", true)
	testTelemetry(t, telemetry, expectedMetrics)
}

func TestTelemetryChannelMode(t *testing.T) {
	// disabling autoflush of the telemetry
	client, err := New("localhost:8125", WithoutTelemetry(), WithChannelMode())
	require.Nil(t, err)

	telemetry := newTelemetryClient(client, "test_transport", false)

	expectedMetrics := []metric{}
	expectedMetrics = appendBasicMetrics(expectedMetrics, nil)

	testTelemetry(t, telemetry, expectedMetrics)
}

func TestTelemetryWithGlobalTags(t *testing.T) {
	orig := os.Getenv("DD_ENV")
	os.Setenv("DD_ENV", "test")
	defer os.Setenv("DD_ENV", orig)

	// disabling autoflush of the telemetry
	client, err := New("localhost:8125", WithoutTelemetry(), WithTags([]string{"tag1", "tag2"}))
	require.Nil(t, err)

	telemetry := newTelemetryClient(client, "test_transport", false)

	expectedMetrics := []metric{}
	expectedMetrics = appendBasicMetrics(expectedMetrics, []string{"tag1", "tag2", "env:test"})

	testTelemetry(t, telemetry, expectedMetrics)
}

func TestTelemetryWithAggregationBasic(t *testing.T) {
	// disabling autoflush of the telemetry
	client, err := New("localhost:8125", WithoutTelemetry(), WithClientSideAggregation())
	require.Nil(t, err)

	telemetry := newTelemetryClient(client, "test_transport", false)

	expectedMetrics := []metric{}
	expectedMetrics = appendBasicMetrics(expectedMetrics, nil)
	expectedMetrics = appendAggregationMetrics(expectedMetrics, nil, false, false)

	testTelemetry(t, telemetry, expectedMetrics)
}

func TestTelemetryWithAggregationAllType(t *testing.T) {
	// disabling autoflush of the telemetry
	client, err := New("localhost:8125", WithoutTelemetry(), WithExtendedClientSideAggregation())
	require.Nil(t, err)

	telemetry := newTelemetryClient(client, "test_transport", false)

	expectedMetrics := []metric{}
	expectedMetrics = appendBasicMetrics(expectedMetrics, nil)
	expectedMetrics = appendAggregationMetrics(expectedMetrics, nil, false, true)

	testTelemetry(t, telemetry, expectedMetrics)
}

func TestTelemetryWithAggregationDevMode(t *testing.T) {
	// disabling autoflush of the telemetry
	client, err := New("localhost:8125", WithoutTelemetry(), WithExtendedClientSideAggregation(), WithDevMode())
	require.Nil(t, err)

	telemetry := newTelemetryClient(client, "test_transport", true)

	expectedMetrics := []metric{}
	expectedMetrics = appendBasicMetrics(expectedMetrics, nil)
	expectedMetrics = appendAggregationMetrics(expectedMetrics, nil, true, true)
	expectedMetrics = appendDevModeMetrics(expectedMetrics, nil)

	testTelemetry(t, telemetry, expectedMetrics)
}

func TestTelemetryWithAggregationDevModeWithGlobalTags(t *testing.T) {
	orig := os.Getenv("DD_ENV")
	os.Setenv("DD_ENV", "test")
	defer os.Setenv("DD_ENV", orig)

	// disabling autoflush of the telemetry
	client, err := New("localhost:8125", WithoutTelemetry(), WithClientSideAggregation(), WithDevMode(), WithTags([]string{"tag1", "tag2"}))
	require.Nil(t, err)

	telemetry := newTelemetryClient(client, "test_transport", true)

	expectedMetrics := []metric{}
	expectedMetrics = appendBasicMetrics(expectedMetrics, []string{"tag1", "tag2", "env:test"})
	expectedMetrics = appendAggregationMetrics(expectedMetrics, []string{"tag1", "tag2", "env:test"}, true, false)
	expectedMetrics = appendDevModeMetrics(expectedMetrics, []string{"tag1", "tag2", "env:test"})

	testTelemetry(t, telemetry, expectedMetrics)
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

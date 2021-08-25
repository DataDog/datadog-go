package statsd

import (
	"io"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTelemetry(t *testing.T) {
	ts, client := newClientAndTestServer(t,
		"udp",
		"localhost:8765",
		nil,
	)

	ts.sendAllAndAssert(t, client)
}

func TestTelemetryWithNamespace(t *testing.T) {
	ts, client := newClientAndTestServer(t,
		"udp",
		"localhost:8765",
		nil,
		WithNamespace("test_namespace"),
	)

	ts.sendAllAndAssert(t, client)
}

func TestTelemetryChannelMode(t *testing.T) {
	ts, client := newClientAndTestServer(t,
		"udp",
		"localhost:8765",
		nil,
		WithChannelMode(),
	)

	ts.sendAllAndAssert(t, client)
}

func TestTelemetryWithGlobalTags(t *testing.T) {
	orig := os.Getenv("DD_ENV")
	os.Setenv("DD_ENV", "test")
	defer os.Setenv("DD_ENV", orig)

	ts, client := newClientAndTestServer(t,
		"udp",
		"localhost:8765",
		[]string{"tag1", "tag2", "env:test"},
		WithTags([]string{"tag1", "tag2"}),
	)

	ts.sendAllAndAssert(t, client)
}

func TestTelemetryWithAggregationBasic(t *testing.T) {
	ts, client := newClientAndTestServer(t,
		"udp",
		"localhost:8765",
		nil,
		WithClientSideAggregation(),
	)

	ts.sendAllAndAssert(t, client)
}

func TestTelemetryWithExtendedAggregation(t *testing.T) {
	ts, client := newClientAndTestServer(t,
		"udp",
		"localhost:8765",
		nil,
		WithExtendedClientSideAggregation(),
	)

	ts.sendAllAndAssert(t, client)
}

func TestTelemetryAllOptions(t *testing.T) {
	orig := os.Getenv("DD_ENV")
	os.Setenv("DD_ENV", "test")
	defer os.Setenv("DD_ENV", orig)

	ts, client := newClientAndTestServer(t,
		"udp",
		"localhost:8765",
		[]string{"tag1", "tag2", "env:test"},
		WithClientSideAggregation(),
		WithExtendedClientSideAggregation(),
		WithTags([]string{"tag1", "tag2"}),
		WithNamespace("test_namespace"),
	)

	ts.sendAllAndAssert(t, client)
}

func TestTelemetryCustomAddr(t *testing.T) {
	telAddr := "localhost:8764"
	ts, client := newClientAndTestServer(t,
		"udp",
		"localhost:8765",
		nil,
		WithTelemetryAddr(telAddr),
		WithNamespace("test_namespace"),
	)

	server := getTestServer(t, telAddr)
	defer server.Close()

	expectedResult := []string{
		"datadog.dogstatsd.client.metrics:9|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp",
		"datadog.dogstatsd.client.metrics_by_type:1|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp,metrics_type:gauge",
		"datadog.dogstatsd.client.metrics_by_type:3|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp,metrics_type:count",
		"datadog.dogstatsd.client.metrics_by_type:1|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp,metrics_type:histogram",
		"datadog.dogstatsd.client.metrics_by_type:1|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp,metrics_type:distribution",
		"datadog.dogstatsd.client.metrics_by_type:1|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp,metrics_type:set",
		"datadog.dogstatsd.client.metrics_by_type:2|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp,metrics_type:timing",
		"datadog.dogstatsd.client.events:1|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp",
		"datadog.dogstatsd.client.service_checks:1|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp",
		"datadog.dogstatsd.client.metric_dropped_on_receive:0|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp",
		"datadog.dogstatsd.client.packets_sent:10|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp",
		"datadog.dogstatsd.client.bytes_sent:473|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp",
		"datadog.dogstatsd.client.packets_dropped:0|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp",
		"datadog.dogstatsd.client.bytes_dropped:0|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp",
		"datadog.dogstatsd.client.packets_dropped_queue:0|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp",
		"datadog.dogstatsd.client.bytes_dropped_queue:0|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp",
		"datadog.dogstatsd.client.packets_dropped_writer:0|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp",
		"datadog.dogstatsd.client.bytes_dropped_writer:0|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp",
	}
	expectedSize := 0
	for _, s := range expectedResult {
		expectedSize += len(s)
	}
	sort.Strings(expectedResult)

	readDone := make(chan struct{})
	buffer := make([]byte, 10000)
	n := 0
	go func() {
		n, _ = io.ReadAtLeast(server, buffer, expectedSize)
		close(readDone)
	}()

	ts.sendAllType(client)
	client.Flush()
	client.telemetryClient.sendTelemetry()

	select {
	case <-readDone:
	case <-time.After(2 * time.Second):
		require.Fail(t, "No data was flush on Close")
	}

	result := []string{}
	for _, s := range strings.Split(string(buffer[:n]), "\n") {
		if s != "" {
			result = append(result, s)
		}
	}
	sort.Strings(result)

	assert.Equal(t, expectedResult, result)
}

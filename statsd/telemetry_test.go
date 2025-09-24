package statsd

import (
	"fmt"
	"io"
	"net"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//
// Most of the behavior of the telemetry is tested in the end_to_end_test.go file
//

func TestTelemetryCustomAddr(t *testing.T) {
	telAddr := "localhost:8764"
	ts, client := newClientAndTestServer(t,
		"udp",
		"localhost:8765",
		nil,
		WithTelemetryAddr(telAddr),
		WithNamespace("test_namespace"),
	)

	udpAddr, err := net.ResolveUDPAddr("udp", telAddr)
	require.Nil(t, err, fmt.Sprintf("could not resolve udp '%s': %s", telAddr, err))
	server, err := net.ListenUDP("udp", udpAddr)
	require.Nil(t, err, fmt.Sprintf("could not listen to UDP addr: %s", err))
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
		"datadog.dogstatsd.client.aggregated_context:5|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp",
		"datadog.dogstatsd.client.aggregated_context_by_type:0|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp,metrics_type:distribution",
		"datadog.dogstatsd.client.aggregated_context_by_type:0|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp,metrics_type:histogram",
		"datadog.dogstatsd.client.aggregated_context_by_type:0|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp,metrics_type:timing",
		"datadog.dogstatsd.client.aggregated_context_by_type:1|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp,metrics_type:gauge",
		"datadog.dogstatsd.client.aggregated_context_by_type:1|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp,metrics_type:set",
		"datadog.dogstatsd.client.aggregated_context_by_type:3|c|#client:go," + clientVersionTelemetryTag + ",client_transport:udp,metrics_type:count",
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
	err = client.Flush()
	require.NoError(t, err)

	client.clientEx.telemetryClient.sendTelemetry()

	select {
	case <-readDone:
	case <-time.After(2 * time.Second):
		require.Fail(t, "No data was flushed on Close")
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

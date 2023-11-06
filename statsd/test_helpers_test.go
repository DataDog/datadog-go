package statsd

import (
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testTelemetryData struct {
	gauge         int
	count         int
	histogram     int
	distribution  int
	set           int
	timing        int
	event         int
	service_check int

	aggregated_context      int
	aggregated_gauge        int
	aggregated_set          int
	aggregated_count        int
	aggregated_histogram    int
	aggregated_distribution int
	aggregated_timing       int

	metric_dropped_on_receive int
	packets_sent              int
	packets_dropped           int
	packets_dropped_queue     int
	packets_dropped_writer    int
	bytes_sent                int
	bytes_dropped             int
	bytes_dropped_queue       int
	bytes_dropped_writer      int
}

// testServer acts as a fake server and keep track of what was sent to a client. This allows end-to-end testing of the
// dogstatsd client
type testServer struct {
	sync.Mutex

	conn        io.ReadCloser
	data        []string
	errors      []string
	readData    []string
	proto       string
	addr        string
	stopped     chan struct{}
	tags        string
	namespace   string
	containerID string

	aggregation         bool
	extendedAggregation bool
	telemetry           testTelemetryData
	telemetryEnabled    bool
}

func newClientAndTestServer(t *testing.T, proto string, addr string, tags []string, options ...Option) (*testServer, *Client) {

	opt, err := resolveOptions(options)
	require.NoError(t, err)

	ts := &testServer{
		proto:               proto,
		data:                []string{},
		addr:                addr,
		stopped:             make(chan struct{}),
		aggregation:         opt.aggregation,
		extendedAggregation: opt.extendedAggregation,
		telemetryEnabled:    opt.telemetry,
		telemetry:           testTelemetryData{},
		namespace:           opt.namespace,
	}

	if tags != nil {
		ts.tags = strings.Join(tags, ",")
	}

	switch proto {
	case "udp":
		udpAddr, err := net.ResolveUDPAddr("udp", addr)
		require.NoError(t, err)

		conn, err := net.ListenUDP("udp", udpAddr)
		require.NoError(t, err)
		ts.conn = conn
	case "uds":
		socketPath := addr[7:]
		address, err := net.ResolveUnixAddr("unixgram", socketPath)
		require.NoError(t, err)
		conn, err := net.ListenUnixgram("unixgram", address)
		require.NoError(t, err)
		err = os.Chmod(socketPath, 0722)
		require.NoError(t, err)
		ts.conn = conn
	default:
		require.FailNow(t, "unknown proto '%s'", proto)
	}

	client, err := New(addr, options...)
	require.NoError(t, err)

	ts.containerID = getContainerID()

	go ts.start()
	return ts, client
}

func (ts *testServer) start() {
	buffer := make([]byte, 2048)
	for {
		n, err := ts.conn.Read(buffer)
		if err != nil {
			// connection has been closed
			if strings.HasSuffix(err.Error(), " use of closed network connection") {
				return
			}
			ts.errors = append(ts.errors, err.Error())
			continue
		}
		readData := string(buffer[:n])
		if n != 0 {
			ts.readData = append(ts.readData, readData)
		}

		payload := strings.Split(readData, "\n")
		ts.Lock()
		for _, s := range payload {
			if s != "" {
				ts.data = append(ts.data, s)
			}
		}
		ts.Unlock()
	}
}

func (ts *testServer) assertMetric(t *testing.T, received []string, expected []string) {
	sort.Strings(expected)
	sort.Strings(received)

	assert.Equal(t, len(expected), len(received), fmt.Sprintf("expected %d metrics but got actual %d", len(expected), len(received)))

	if os.Getenv("PRINT_METRICS") != "" && len(expected) != len(received) {
		fmt.Printf("received:\n")
		for _, m := range received {
			fmt.Printf("	%s\n", m)
		}

		fmt.Printf("\nexpected:\n")
		for _, m := range expected {
			fmt.Printf("	%s\n", m)
		}
	}

	min := len(received)
	if len(expected) < min {
		min = len(expected)
	}

	for idx := 0; idx < min; idx++ {
		if strings.HasPrefix(expected[idx], "datadog.dogstatsd.client.bytes_sent") {
			continue
		}
		if strings.HasPrefix(expected[idx], "datadog.dogstatsd.client.packets_sent") {
			continue
		}
		assert.Equal(t, expected[idx], received[idx])
	}
}

func (ts *testServer) stop() {
	ts.conn.Close()
	close(ts.stopped)
}

func (ts *testServer) wait(t *testing.T, nbExpectedMetric int, timeout int, waitForTelemetry bool) {
	start := time.Now()
	for {
		ts.Lock()
		if nbExpectedMetric <= len(ts.data) {
			ts.Unlock()
			return
		} else if time.Now().Sub(start) > time.Duration(timeout)*time.Second {
			ts.Unlock()
			require.FailNowf(t, "timeout while waiting for metrics", "%d metrics expected but only %d were received after %s\n", nbExpectedMetric, len(ts.data), time.Now().Sub(start))
			return
		}
		ts.Unlock()
		time.Sleep(100 * time.Millisecond)
	}
}

func (ts *testServer) assertNbRead(t *testing.T, expectedNbRead int) {
	errorMsg := ""
	for idx, s := range ts.readData {
		errorMsg += fmt.Sprintf("read %d:\n%s\n\n", idx, s)
	}
	assert.Equal(t, expectedNbRead, len(ts.readData), "expected %d read but got %d:\n%s", expectedNbRead, len(ts.readData), errorMsg)
}

// meta helper: take a list of expected metrics and assert
func (ts *testServer) assert(t *testing.T, client *Client, expectedMetrics []string) {
	// First wait for all the metrics to be sent. This is important when using channel mode + aggregation as we
	// don't know when all the metrics will be fully aggregated
	ts.wait(t, len(expectedMetrics), 5, false)

	if ts.telemetryEnabled {
		// Now that all the metrics have been handled we can flush the telemetry before the default interval of
		// 10s
		client.telemetryClient.sendTelemetry()
		expectedMetrics = append(expectedMetrics, ts.getTelemetry()...)
		// Wait for the telemetry to arrive
		ts.wait(t, len(expectedMetrics), 5, true)
	}

	client.Close()
	ts.stop()
	received := ts.getData()
	ts.assertMetric(t, received, expectedMetrics)
	assert.Empty(t, ts.errors)
}

func (ts *testServer) assertContainerID(t *testing.T, expected string) {
	assert.Equal(t, expected, ts.containerID)
}

// meta helper: most test send all types and then assert
func (ts *testServer) sendAllAndAssert(t *testing.T, client *Client) {
	expectedMetrics := ts.sendAllType(client)
	ts.assert(t, client, expectedMetrics)
}

func (ts *testServer) getData() []string {
	ts.Lock()
	defer ts.Unlock()

	data := make([]string, len(ts.data))
	copy(data, ts.data)
	return data
}

func (ts *testServer) getTelemetry() []string {
	ts.Lock()
	defer ts.Unlock()

	tags := ts.getFinalTelemetryTags()

	totalMetrics := ts.telemetry.gauge +
		ts.telemetry.count +
		ts.telemetry.histogram +
		ts.telemetry.distribution +
		ts.telemetry.set +
		ts.telemetry.timing

	containerID := ts.getContainerID()

	metrics := []string{
		fmt.Sprintf("datadog.dogstatsd.client.metrics:%d|c%s", totalMetrics, tags) + containerID,
		fmt.Sprintf("datadog.dogstatsd.client.events:%d|c%s", ts.telemetry.event, tags) + containerID,
		fmt.Sprintf("datadog.dogstatsd.client.service_checks:%d|c%s", ts.telemetry.service_check, tags) + containerID,
		fmt.Sprintf("datadog.dogstatsd.client.metric_dropped_on_receive:%d|c%s", ts.telemetry.metric_dropped_on_receive, tags) + containerID,
		fmt.Sprintf("datadog.dogstatsd.client.packets_sent:%d|c%s", ts.telemetry.packets_sent, tags) + containerID,
		fmt.Sprintf("datadog.dogstatsd.client.packets_dropped:%d|c%s", ts.telemetry.packets_dropped, tags) + containerID,
		fmt.Sprintf("datadog.dogstatsd.client.packets_dropped_queue:%d|c%s", ts.telemetry.packets_dropped_queue, tags) + containerID,
		fmt.Sprintf("datadog.dogstatsd.client.packets_dropped_writer:%d|c%s", ts.telemetry.packets_dropped_writer, tags) + containerID,
		fmt.Sprintf("datadog.dogstatsd.client.bytes_sent:%d|c%s", ts.telemetry.bytes_sent, tags) + containerID,
		fmt.Sprintf("datadog.dogstatsd.client.bytes_dropped:%d|c%s", ts.telemetry.bytes_dropped, tags) + containerID,
		fmt.Sprintf("datadog.dogstatsd.client.bytes_dropped_queue:%d|c%s", ts.telemetry.bytes_dropped_queue, tags) + containerID,
		fmt.Sprintf("datadog.dogstatsd.client.bytes_dropped_writer:%d|c%s", ts.telemetry.bytes_dropped_writer, tags) + containerID,
		fmt.Sprintf("datadog.dogstatsd.client.metrics_by_type:%d|c%s,metrics_type:gauge", ts.telemetry.gauge, tags) + containerID,
		fmt.Sprintf("datadog.dogstatsd.client.metrics_by_type:%d|c%s,metrics_type:count", ts.telemetry.count, tags) + containerID,
		fmt.Sprintf("datadog.dogstatsd.client.metrics_by_type:%d|c%s,metrics_type:histogram", ts.telemetry.histogram, tags) + containerID,
		fmt.Sprintf("datadog.dogstatsd.client.metrics_by_type:%d|c%s,metrics_type:distribution", ts.telemetry.distribution, tags) + containerID,
		fmt.Sprintf("datadog.dogstatsd.client.metrics_by_type:%d|c%s,metrics_type:set", ts.telemetry.set, tags) + containerID,
		fmt.Sprintf("datadog.dogstatsd.client.metrics_by_type:%d|c%s,metrics_type:timing", ts.telemetry.timing, tags) + containerID,
	}

	if ts.aggregation {
		metrics = append(metrics, []string{
			fmt.Sprintf("datadog.dogstatsd.client.aggregated_context:%d|c%s", ts.telemetry.aggregated_context, tags) + containerID,
			fmt.Sprintf("datadog.dogstatsd.client.aggregated_context_by_type:%d|c%s,metrics_type:gauge", ts.telemetry.aggregated_gauge, tags) + containerID,
			fmt.Sprintf("datadog.dogstatsd.client.aggregated_context_by_type:%d|c%s,metrics_type:count", ts.telemetry.aggregated_count, tags) + containerID,
			fmt.Sprintf("datadog.dogstatsd.client.aggregated_context_by_type:%d|c%s,metrics_type:set", ts.telemetry.aggregated_set, tags) + containerID,
			fmt.Sprintf("datadog.dogstatsd.client.aggregated_context_by_type:%d|c%s,metrics_type:distribution", ts.telemetry.aggregated_distribution, tags) + containerID,
			fmt.Sprintf("datadog.dogstatsd.client.aggregated_context_by_type:%d|c%s,metrics_type:histogram", ts.telemetry.aggregated_histogram, tags) + containerID,
			fmt.Sprintf("datadog.dogstatsd.client.aggregated_context_by_type:%d|c%s,metrics_type:timing", ts.telemetry.aggregated_timing, tags) + containerID,
		}...)
	}
	return metrics
}

// Default testing scenarios

func (ts *testServer) getFinalTags(t ...string) string {
	if t == nil && ts.tags == "" {
		return ""
	}

	res := "|#"
	if ts.tags != "" {
		res += ts.tags
	}

	if t != nil {
		if ts.tags != "" {
			res += ","
		}
		res += strings.Join(t, ",")
	}
	return res
}

func (ts *testServer) getContainerID() string {
	if ts.containerID == "" {
		return ""
	}
	return "|c:" + ts.containerID
}

func (ts *testServer) getFinalTelemetryTags() string {
	base := "|#"
	if ts.tags != "" {
		base += ts.tags + ","
	}
	return base + strings.Join(
		[]string{clientTelemetryTag, clientVersionTelemetryTag, "client_transport:" + ts.proto},
		",")
}

func (ts *testServer) sendAllMetrics(c *Client) []string {
	tags := []string{"custom:1", "custom:2"}
	c.Gauge("Gauge", 1, tags, 1)
	c.Count("Count", 2, tags, 1)
	c.Histogram("Histogram", 3, tags, 1)
	c.Distribution("Distribution", 4, tags, 1)
	c.Decr("Decr", tags, 1)
	c.Incr("Incr", tags, 1)
	c.Set("Set", "value", tags, 1)
	c.Timing("Timing", 5*time.Second, tags, 1)
	c.TimeInMilliseconds("TimeInMilliseconds", 6, tags, 1)

	ts.telemetry.gauge += 1
	ts.telemetry.histogram += 1
	ts.telemetry.distribution += 1
	ts.telemetry.count += 3
	ts.telemetry.set += 1
	ts.telemetry.timing += 2

	if ts.aggregation {
		ts.telemetry.aggregated_context += 5
		ts.telemetry.aggregated_gauge += 1
		ts.telemetry.aggregated_count += 3
		ts.telemetry.aggregated_set += 1
	}
	if ts.extendedAggregation {
		ts.telemetry.aggregated_context += 4
		ts.telemetry.aggregated_histogram += 1
		ts.telemetry.aggregated_distribution += 1
		ts.telemetry.aggregated_timing += 2
	}

	finalTags := ts.getFinalTags(tags...)
	containerID := ts.getContainerID()

	return []string{
		ts.namespace + "Gauge:1|g" + finalTags + containerID,
		ts.namespace + "Count:2|c" + finalTags + containerID,
		ts.namespace + "Histogram:3|h" + finalTags + containerID,
		ts.namespace + "Distribution:4|d" + finalTags + containerID,
		ts.namespace + "Decr:-1|c" + finalTags + containerID,
		ts.namespace + "Incr:1|c" + finalTags + containerID,
		ts.namespace + "Set:value|s" + finalTags + containerID,
		ts.namespace + "Timing:5000.000000|ms" + finalTags + containerID,
		ts.namespace + "TimeInMilliseconds:6.000000|ms" + finalTags + containerID,
	}
}

func (ts *testServer) sendAllMetricsForBasicAggregation(c *Client) []string {
	tags := []string{"custom:1", "custom:2"}
	c.Gauge("Gauge", 1, tags, 1)
	c.Gauge("Gauge", 2, tags, 1)
	c.Count("Count", 2, tags, 1)
	c.Count("Count", 2, tags, 1)
	c.Histogram("Histogram", 3, tags, 1)
	c.Distribution("Distribution", 4, tags, 1)
	c.Decr("Decr", tags, 1)
	c.Decr("Decr", tags, 1)
	c.Incr("Incr", tags, 1)
	c.Incr("Incr", tags, 1)
	c.Set("Set", "value", tags, 1)
	c.Set("Set", "value", tags, 1)
	c.Timing("Timing", 5*time.Second, tags, 1)
	c.TimeInMilliseconds("TimeInMilliseconds", 6, tags, 1)

	ts.telemetry.gauge += 2
	ts.telemetry.histogram += 1
	ts.telemetry.distribution += 1
	ts.telemetry.count += 6
	ts.telemetry.set += 2
	ts.telemetry.timing += 2

	if ts.aggregation {
		ts.telemetry.aggregated_context += 5
		ts.telemetry.aggregated_gauge += 1
		ts.telemetry.aggregated_count += 3
		ts.telemetry.aggregated_set += 1
	}
	if ts.extendedAggregation {
		ts.telemetry.aggregated_context += 4
		ts.telemetry.aggregated_histogram += 1
		ts.telemetry.aggregated_distribution += 1
		ts.telemetry.aggregated_timing += 2
	}

	finalTags := ts.getFinalTags(tags...)
	containerID := ts.getContainerID()

	return []string{
		ts.namespace + "Gauge:2|g" + finalTags + containerID,
		ts.namespace + "Count:4|c" + finalTags + containerID,
		ts.namespace + "Histogram:3|h" + finalTags + containerID,
		ts.namespace + "Distribution:4|d" + finalTags + containerID,
		ts.namespace + "Decr:-2|c" + finalTags + containerID,
		ts.namespace + "Incr:2|c" + finalTags + containerID,
		ts.namespace + "Set:value|s" + finalTags + containerID,
		ts.namespace + "Timing:5000.000000|ms" + finalTags + containerID,
		ts.namespace + "TimeInMilliseconds:6.000000|ms" + finalTags + containerID,
	}
}

func (ts *testServer) sendAllMetricsForExtendedAggregation(c *Client) []string {
	tags := []string{"custom:1", "custom:2"}
	c.Gauge("Gauge", 1, tags, 1)
	c.Gauge("Gauge", 2, tags, 1)
	c.Count("Count", 2, tags, 1)
	c.Count("Count", 2, tags, 1)
	c.Histogram("Histogram", 3, tags, 1)
	c.Histogram("Histogram", 3, tags, 1)
	c.Distribution("Distribution", 4, tags, 1)
	c.Distribution("Distribution", 4, tags, 1)
	c.Decr("Decr", tags, 1)
	c.Decr("Decr", tags, 1)
	c.Incr("Incr", tags, 1)
	c.Incr("Incr", tags, 1)
	c.Set("Set", "value", tags, 1)
	c.Set("Set", "value", tags, 1)
	c.Timing("Timing", 5*time.Second, tags, 1)
	c.Timing("Timing", 5*time.Second, tags, 1)
	c.TimeInMilliseconds("TimeInMilliseconds", 6, tags, 1)
	c.TimeInMilliseconds("TimeInMilliseconds", 6, tags, 1)

	ts.telemetry.gauge += 2
	ts.telemetry.histogram += 2
	ts.telemetry.distribution += 2
	ts.telemetry.count += 6
	ts.telemetry.set += 2
	ts.telemetry.timing += 4

	if ts.aggregation {
		ts.telemetry.aggregated_context += 5
		ts.telemetry.aggregated_gauge += 1
		ts.telemetry.aggregated_count += 3
		ts.telemetry.aggregated_set += 1
	}
	if ts.extendedAggregation {
		ts.telemetry.aggregated_context += 4
		ts.telemetry.aggregated_histogram += 1
		ts.telemetry.aggregated_distribution += 1
		ts.telemetry.aggregated_timing += 2
	}

	finalTags := ts.getFinalTags(tags...)
	containerID := ts.getContainerID()

	return []string{
		ts.namespace + "Gauge:2|g" + finalTags + containerID,
		ts.namespace + "Count:4|c" + finalTags + containerID,
		ts.namespace + "Histogram:3:3|h" + finalTags + containerID,
		ts.namespace + "Distribution:4:4|d" + finalTags + containerID,
		ts.namespace + "Decr:-2|c" + finalTags + containerID,
		ts.namespace + "Incr:2|c" + finalTags + containerID,
		ts.namespace + "Set:value|s" + finalTags + containerID,
		ts.namespace + "Timing:5000.000000:5000.000000|ms" + finalTags + containerID,
		ts.namespace + "TimeInMilliseconds:6.000000:6.000000|ms" + finalTags + containerID,
	}
}

func (ts *testServer) sendAllMetricsForExtendedAggregationAndMaxSamples(c *Client) []string {
	tags := []string{"custom:1", "custom:2"}
	c.Gauge("Gauge", 1, tags, 1)
	c.Gauge("Gauge", 2, tags, 1)
	c.Count("Count", 2, tags, 1)
	c.Count("Count", 2, tags, 1)
	c.Histogram("Histogram", 3, tags, 1)
	c.Histogram("Histogram", 3, tags, 1)
	c.Histogram("Histogram", 3, tags, 1)
	c.Distribution("Distribution", 4, tags, 1)
	c.Distribution("Distribution", 4, tags, 1)
	c.Distribution("Distribution", 4, tags, 1)
	c.Decr("Decr", tags, 1)
	c.Decr("Decr", tags, 1)
	c.Incr("Incr", tags, 1)
	c.Incr("Incr", tags, 1)
	c.Set("Set", "value", tags, 1)
	c.Set("Set", "value", tags, 1)
	c.Timing("Timing", 5*time.Second, tags, 1)
	c.Timing("Timing", 5*time.Second, tags, 1)
	c.Timing("Timing", 5*time.Second, tags, 1)
	c.TimeInMilliseconds("TimeInMilliseconds", 6, tags, 1)

	ts.telemetry.gauge += 2
	ts.telemetry.histogram += 3
	ts.telemetry.distribution += 3
	ts.telemetry.count += 6
	ts.telemetry.set += 2
	ts.telemetry.timing += 4

	if ts.aggregation {
		ts.telemetry.aggregated_context += 5
		ts.telemetry.aggregated_gauge += 1
		ts.telemetry.aggregated_count += 3
		ts.telemetry.aggregated_set += 1
	}
	if ts.extendedAggregation {
		ts.telemetry.aggregated_context += 4
		ts.telemetry.aggregated_histogram += 1
		ts.telemetry.aggregated_distribution += 1
		ts.telemetry.aggregated_timing += 2
	}

	finalTags := ts.getFinalTags(tags...)
	containerID := ts.getContainerID()

	// Even though we recorded 3 samples, we will send only 2
	return []string{
		ts.namespace + "Gauge:2|g" + finalTags + containerID,
		ts.namespace + "Count:4|c" + finalTags + containerID,
		ts.namespace + "Histogram:3:3|h|@0.6666666666666666" + finalTags + containerID,
		ts.namespace + "Distribution:4:4|d|@0.6666666666666666" + finalTags + containerID,
		ts.namespace + "Decr:-2|c" + finalTags + containerID,
		ts.namespace + "Incr:2|c" + finalTags + containerID,
		ts.namespace + "Set:value|s" + finalTags + containerID,
		ts.namespace + "Timing:5000.000000:5000.000000|ms|@0.6666666666666666" + finalTags + containerID,
		ts.namespace + "TimeInMilliseconds:6.000000|ms" + finalTags + containerID,
	}
}

func (ts *testServer) sendAllType(c *Client) []string {
	res := ts.sendAllMetrics(c)
	c.SimpleEvent("hello", "world")
	c.SimpleServiceCheck("hello", Warn)

	ts.telemetry.event += 1
	ts.telemetry.service_check += 1

	finalTags := ts.getFinalTags()
	containerID := ts.getContainerID()

	return append(
		res,
		"_e{5,5}:hello|world"+finalTags+containerID,
		"_sc|hello|1"+finalTags+containerID,
	)
}

func (ts *testServer) sendBasicAggregationMetrics(client *Client) []string {
	tags := []string{"custom:1", "custom:2"}
	client.Gauge("gauge", 1, tags, 1)
	client.Gauge("gauge", 21, tags, 1)
	client.Count("count", 1, tags, 1)
	client.Count("count", 3, tags, 1)
	client.Set("set", "my_id", tags, 1)
	client.Set("set", "my_id", tags, 1)

	finalTags := ts.getFinalTags(tags...)
	containerID := ts.getContainerID()
	return []string{
		ts.namespace + "set:my_id|s" + finalTags + containerID,
		ts.namespace + "gauge:21|g" + finalTags + containerID,
		ts.namespace + "count:4|c" + finalTags + containerID,
	}
}

func (ts *testServer) sendExtendedBasicAggregationMetrics(client *Client) []string {
	tags := []string{"custom:1", "custom:2"}
	client.Gauge("gauge", 1, tags, 1)
	client.Count("count", 2, tags, 1)
	client.Set("set", "3_id", tags, 1)
	client.Histogram("histo", 4, tags, 1)
	client.Distribution("distro", 5, tags, 1)
	client.Timing("timing", 6*time.Second, tags, 1)

	finalTags := ts.getFinalTags(tags...)
	containerID := ts.getContainerID()
	return []string{
		ts.namespace + "gauge:1|g" + finalTags + containerID,
		ts.namespace + "count:2|c" + finalTags + containerID,
		ts.namespace + "set:3_id|s" + finalTags + containerID,
		ts.namespace + "histo:4|h" + finalTags + containerID,
		ts.namespace + "distro:5|d" + finalTags + containerID,
		ts.namespace + "timing:6000.000000|ms" + finalTags + containerID,
	}
}

func patchContainerID(id string) { containerID = id }

func resetContainerID() {
	containerID = ""
	initOnce = sync.Once{}
}

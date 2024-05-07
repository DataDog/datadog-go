package statsd

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertNotPanics(t *testing.T, f func()) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatal(r)
		}
	}()
	f()
}

func TestNilError(t *testing.T) {
	var c *Client
	tests := []func() error{
		func() error { return c.Flush() },
		func() error { return c.Close() },
		func() error { return c.Count("", 0, nil, 1) },
		func() error { return c.Incr("", nil, 1) },
		func() error { return c.Decr("", nil, 1) },
		func() error { return c.Histogram("", 0, nil, 1) },
		func() error { return c.Distribution("", 0, nil, 1) },
		func() error { return c.Gauge("", 0, nil, 1) },
		func() error { return c.Set("", "", nil, 1) },
		func() error { return c.Timing("", time.Second, nil, 1) },
		func() error { return c.TimeInMilliseconds("", 1, nil, 1) },
		func() error { return c.Event(NewEvent("", "")) },
		func() error { return c.SimpleEvent("", "") },
		func() error { return c.ServiceCheck(NewServiceCheck("", Ok)) },
		func() error { return c.SimpleServiceCheck("", Ok) },
		func() error {
			_, err := CloneWithExtraOptions(nil, WithChannelMode())
			return err
		},
	}
	for i, f := range tests {
		var err error
		assertNotPanics(t, func() { err = f() })
		if err != ErrNoClient {
			t.Errorf("Test case %d: expected ErrNoClient, got %#v", i, err)
		}
	}
}

func TestDoubleClosePanic(t *testing.T) {
	c, err := New("localhost:8125")
	assert.NoError(t, err)
	c.Close()
	c.Close()
}

type statsdWriterWrapper struct {
	data []string
}

func (s *statsdWriterWrapper) Close() error {
	return nil
}

func (s *statsdWriterWrapper) Write(p []byte) (n int, err error) {
	for _, m := range strings.Split(string(p), "\n") {
		if m != "" {
			s.data = append(s.data, m)
		}
	}
	return len(p), nil
}

func TestNewWithWriter(t *testing.T) {
	w := statsdWriterWrapper{}
	client, err := NewWithWriter(&w, WithoutTelemetry())
	require.Nil(t, err)

	ts := &testServer{}
	expected := ts.sendAllType(client)
	client.Close()

	ts.assertMetric(t, w.data, expected)
}

// TestConcurrentSend sends various metric types in separate goroutines to
// trigger any possible data races. It is intended to be run with the data race
// detector enabled.
func TestConcurrentSend(t *testing.T) {
	tests := []struct {
		description   string
		clientOptions []Option
	}{
		{
			description:   "Client with default options",
			clientOptions: []Option{},
		},
		{
			description:   "Client with mutex mode enabled",
			clientOptions: []Option{WithMutexMode()},
		},
		{
			description:   "Client with channel mode enabled",
			clientOptions: []Option{WithChannelMode()},
		},
	}

	for _, test := range tests {
		test := test // Capture range variable.
		t.Run(test.description, func(t *testing.T) {
			t.Parallel()

			client, err := New("localhost:9876", test.clientOptions...)
			require.Nil(t, err, fmt.Sprintf("failed to create client: %s", err))

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				client.Gauge("name", 1, []string{"tag"}, 0.1)
				wg.Done()
			}()

			wg.Add(1)
			go func() {
				client.Count("name", 1, []string{"tag"}, 0.1)
				wg.Done()
			}()

			wg.Add(1)
			go func() {
				client.Timing("name", 1, []string{"tag"}, 0.1)
				wg.Done()
			}()

			wg.Wait()
			err = client.Close()
			require.Nil(t, err, fmt.Sprintf("failed to close client: %s", err))
		})
	}
}

// TestCloseRace close the client multiple times in separate goroutines to
// trigger any possible data races. It is intended to be run with the data race
// detector enabled.
func TestCloseRace(t *testing.T) {
	c, err := New("localhost:8125")
	assert.NoError(t, err)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for j := 0; j < 100; j++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			c.Close()
		}()
	}
	close(start)
	wg.Wait()
}

func TestCloseWithClientAlreadyClosed(t *testing.T) {
	c, err := New("localhost:8125")
	assert.NoError(t, err)
	assert.False(t, c.IsClosed())

	assert.NoError(t, c.Close())
	assert.True(t, c.IsClosed())

	assert.NoError(t, c.Close())
	assert.True(t, c.IsClosed())
}

func TestIsClosed(t *testing.T) {
	c, err := New("localhost:8125")
	assert.NoError(t, err)
	assert.False(t, c.IsClosed())

	assert.NoError(t, c.Close())
	assert.True(t, c.IsClosed())
}

func TestCloneWithExtraOptions(t *testing.T) {
	client, err := New("localhost:1201", WithTags([]string{"tag1", "tag2"}))
	require.Nil(t, err, fmt.Sprintf("failed to create client: %s", err))

	assert.Equal(t, client.tags, []string{"tag1", "tag2"})
	assert.Equal(t, client.namespace, "")
	assert.Equal(t, client.workersMode, mutexMode)
	assert.Equal(t, "localhost:1201", client.addrOption)
	assert.Len(t, client.options, 1)

	cloneClient, err := CloneWithExtraOptions(client, WithNamespace("test"), WithChannelMode())
	require.Nil(t, err, fmt.Sprintf("failed to clone client: %s", err))

	assert.Equal(t, cloneClient.tags, []string{"tag1", "tag2"})
	assert.Equal(t, cloneClient.namespace, "test.")
	assert.Equal(t, cloneClient.workersMode, channelMode)
	assert.Equal(t, "localhost:1201", cloneClient.addrOption)
	assert.Len(t, cloneClient.options, 3)
}

func TestResolveAddressFromEnvironment(t *testing.T) {
	hostInitialValue, hostInitiallySet := os.LookupEnv(agentHostEnvVarName)
	if hostInitiallySet {
		defer os.Setenv(agentHostEnvVarName, hostInitialValue)
	} else {
		defer os.Unsetenv(agentHostEnvVarName)
	}
	portInitialValue, portInitiallySet := os.LookupEnv(agentPortEnvVarName)
	if portInitiallySet {
		defer os.Setenv(agentPortEnvVarName, portInitialValue)
	} else {
		defer os.Unsetenv(agentPortEnvVarName)
	}
	urlInitialValue, urlInitiallySet := os.LookupEnv(agentURLEnvVarName)
	if urlInitiallySet {
		defer os.Setenv(agentURLEnvVarName, urlInitialValue)
	} else {
		defer os.Unsetenv(agentURLEnvVarName)
	}

	for _, tc := range []struct {
		name         string
		addrParam    string
		hostEnv      string
		portEnv      string
		urlEnv       string
		expectedAddr string
	}{
		{"UPD Nominal case", "127.0.0.1:1234", "", "", "", "127.0.0.1:1234"},
		{"UPD Parameter overrides environment", "127.0.0.1:8125", "10.12.16.9", "1234", "", "127.0.0.1:8125"},
		{"UPD Host and port passed as env", "", "10.12.16.9", "1234", "", "10.12.16.9:1234"},
		{"UPD Host env, default port", "", "10.12.16.9", "", "", "10.12.16.9:8125"},
		{"UPD Host passed, ignore env port", "10.12.16.9", "", "1234", "", "10.12.16.9:8125"},

		{"UDS socket passed", "unix://test/path.socket", "", "", "", "unix://test/path.socket"},
		{"UDS socket env", "", "unix://test/path.socket", "", "", "unix://test/path.socket"},
		{"UDS socket env with port", "", "unix://test/path.socket", "8125", "", "unix://test/path.socket"},

		{"Pipe passed", "\\\\.\\pipe\\my_pipe", "", "", "", "\\\\.\\pipe\\my_pipe"},
		{"Pipe env", "", "\\\\.\\pipe\\my_pipe", "", "", "\\\\.\\pipe\\my_pipe"},
		{"Pipe env with port", "", "\\\\.\\pipe\\my_pipe", "8125", "", "\\\\.\\pipe\\my_pipe"},

		{"DD_DOGSTATSD_URL UDP", "", "", "", "udp://localhost:1234", "localhost:1234"},
		{"DD_DOGSTATSD_URL UDP, default port", "", "", "", "udp://localhost", "localhost:8125"},
		{"DD_DOGSTATSD_URL UDS", "", "", "", "unix://test/path.socket", "unix://test/path.socket"},
		{"DD_DOGSTATSD_URL UDS, ignore env port", "", "", "1234", "udp://198.51.100.123:4321", "198.51.100.123:4321"},
		{"DD_DOGSTATSD_URL UDS, ignore env host", "", "localhost", "", "udp://198.51.100.123:4321", "198.51.100.123:4321"},
		{"DD_DOGSTATSD_URL Pipe", "", "", "", "\\\\.\\pipe\\my_pipe", "\\\\.\\pipe\\my_pipe"},
		{"DD_DOGSTATSD_URL with no valid scheme", "", "", "", "localhost:1234", ""},

		{"No autodetection failed", "", "", "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_ = os.Setenv(agentHostEnvVarName, tc.hostEnv)
			_ = os.Setenv(agentPortEnvVarName, tc.portEnv)
			_ = os.Setenv(agentURLEnvVarName, tc.urlEnv)

			addr := resolveAddr(tc.addrParam)
			assert.Equal(t, tc.expectedAddr, addr)
		})
	}
}

func TestGetTelemetry(t *testing.T) {
	ts, client := newClientAndTestServer(t,
		"udp",
		"localhost:8765",
		nil,
		WithExtendedClientSideAggregation(),
	)

	ts.sendAllAndAssert(t, client)
	tlm := client.GetTelemetry()

	assert.Equal(t, uint64(9), tlm.TotalMetrics, "telmetry TotalMetrics was wrong")
	assert.Equal(t, uint64(1), tlm.TotalMetricsGauge, "telmetry TotalMetricsGauge was wrong")
	assert.Equal(t, uint64(3), tlm.TotalMetricsCount, "telmetry TotalMetricsCount was wrong")
	assert.Equal(t, uint64(1), tlm.TotalMetricsHistogram, "telmetry TotalMetricsHistogram was wrong")
	assert.Equal(t, uint64(1), tlm.TotalMetricsDistribution, "telmetry TotalMetricsDistribution was wrong")
	assert.Equal(t, uint64(1), tlm.TotalMetricsSet, "telmetry TotalMetricsSet was wrong")
	assert.Equal(t, uint64(2), tlm.TotalMetricsTiming, "telmetry TotalMetricsTiming was wrong")
	assert.Equal(t, uint64(1), tlm.TotalEvents, "telmetry TotalEvents was wrong")
	assert.Equal(t, uint64(1), tlm.TotalServiceChecks, "telmetry TotalServiceChecks was wrong")
	assert.Equal(t, uint64(0), tlm.TotalDroppedOnReceive, "telmetry TotalDroppedOnReceive was wrong")
	assert.Equal(t, uint64(22), tlm.TotalPayloadsSent, "telmetry TotalPayloadsSent was wrong")
	assert.Equal(t, uint64(0), tlm.TotalPayloadsDropped, "telmetry TotalPayloadsDropped was wrong")
	assert.Equal(t, uint64(0), tlm.TotalPayloadsDroppedWriter, "telmetry TotalPayloadsDroppedWriter was wrong")
	assert.Equal(t, uint64(0), tlm.TotalPayloadsDroppedQueueFull, "telmetry TotalPayloadsDroppedQueueFull was wrong")
	assert.Equal(t, uint64(3112), tlm.TotalBytesSent, "telmetry TotalBytesSent was wrong")
	assert.Equal(t, uint64(0), tlm.TotalBytesDropped, "telmetry TotalBytesDropped was wrong")
	assert.Equal(t, uint64(0), tlm.TotalBytesDroppedWriter, "telmetry TotalBytesDroppedWriter was wrong")
	assert.Equal(t, uint64(0), tlm.TotalBytesDroppedQueueFull, "telmetry TotalBytesDroppedQueueFull was wrong")
	assert.Equal(t, uint64(9), tlm.AggregationNbContext, "telmetry AggregationNbContext was wrong")
	assert.Equal(t, uint64(1), tlm.AggregationNbContextGauge, "telmetry AggregationNbContextGauge was wrong")
	assert.Equal(t, uint64(3), tlm.AggregationNbContextCount, "telmetry AggregationNbContextCount was wrong")
	assert.Equal(t, uint64(1), tlm.AggregationNbContextSet, "telmetry AggregationNbContextSet was wrong")
	assert.Equal(t, uint64(1), tlm.AggregationNbContextHistogram, "telmetry AggregationNbContextHistogram was wrong")
	assert.Equal(t, uint64(1), tlm.AggregationNbContextDistribution, "telmetry AggregationNbContextDistribution was wrong")
	assert.Equal(t, uint64(2), tlm.AggregationNbContextTiming, "telmetry AggregationNbContextTiming was wrong")
}

func Test_isOriginDetectionEnabled(t *testing.T) {
	tests := []struct {
		name              string
		o                 *Options
		configEnvVarValue string
		want              bool
	}{
		{
			name:              "nominal case",
			o:                 &Options{originDetection: defaultOriginDetection},
			configEnvVarValue: "",
			want:              true,
		},
		{
			name:              "has user-provided container ID",
			o:                 &Options{containerID: "user-provided"},
			configEnvVarValue: "",
			want:              false,
		},
		{
			name:              "originDetection option disabled",
			o:                 &Options{originDetection: false},
			configEnvVarValue: "",
			want:              false,
		},
		{
			name:              "DD_ORIGIN_DETECTION_ENABLED=false",
			o:                 &Options{originDetection: defaultOriginDetection},
			configEnvVarValue: "false",
			want:              false,
		},
		{
			name:              "invalid DD_ORIGIN_DETECTION_ENABLED value",
			o:                 &Options{originDetection: defaultOriginDetection},
			configEnvVarValue: "invalid",
			want:              true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("DD_ORIGIN_DETECTION_ENABLED", tt.configEnvVarValue)
			defer os.Unsetenv("DD_ORIGIN_DETECTION_ENABLED")

			assert.Equal(t, tt.want, isOriginDetectionEnabled(tt.o))
		})
	}
}

func TestMessageTooLongError(t *testing.T) {
	client, err := New("localhost:8765", WithMaxBytesPerPayload(10), WithoutClientSideAggregation())
	require.NoError(t, err)

	err = client.Gauge("fake_name_", 21, nil, 1)
	require.Error(t, err)
	assert.IsType(t, MessageTooLongError{}, err)
}

func withNoWorkers() Option {
	return func(o *Options) error {
		o.workersCount = 0
		return nil
	}
}
func TestErrorsReturnedWithAggregator(t *testing.T) {
	client, err := New("localhost:8765",
		WithChannelMode(), WithExtendedClientSideAggregation(),
		withNoWorkers(), WithChannelModeBufferSize(1),
		WithErrorHandler(LoggingErrorHandler), WithChannelModeErrorsWhenFull(),
	)
	require.NoError(t, err)

	err = client.Distribution("fake_name_", 21, nil, 1)
	require.NoError(t, err)

	err = client.Distribution("fake_name_", 21, nil, 1)
	require.Error(t, err)
	assert.IsType(t, &ErrorInputChannelFull{}, err)

	err = client.Close()
	require.NoError(t, err)
}

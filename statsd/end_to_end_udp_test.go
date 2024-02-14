package statsd

import (
	"os"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPipelineWithGlobalTags(t *testing.T) {
	ts, client := newClientAndTestServer(t,
		"udp",
		"localhost:8765",
		[]string{"tag1", "tag2"},
		WithTags([]string{"tag1", "tag2"}),
	)

	ts.sendAllAndAssert(t, client)
}

func TestKownEnvTags(t *testing.T) {
	entityIDEnvName := "DD_ENTITY_ID"
	ddEnvName := "DD_ENV"
	ddServiceName := "DD_SERVICE"
	ddVersionName := "DD_VERSION"

	defer func() { os.Unsetenv(entityIDEnvName) }()
	defer func() { os.Unsetenv(ddEnvName) }()
	defer func() { os.Unsetenv(ddServiceName) }()
	defer func() { os.Unsetenv(ddVersionName) }()

	os.Setenv(entityIDEnvName, "test_id")
	os.Setenv(ddEnvName, "test_env")
	os.Setenv(ddServiceName, "test_service")
	os.Setenv(ddVersionName, "test_version")

	expectedTags := []string{"dd.internal.entity_id:test_id", "env:test_env", "service:test_service", "version:test_version"}
	ts, client := newClientAndTestServer(t,
		"udp",
		"localhost:8765",
		expectedTags,
	)

	sort.Strings(client.tags)
	assert.Equal(t, expectedTags, client.tags)
	ts.sendAllAndAssert(t, client)
}

func TestKnownEnvTagsWithCustomTags(t *testing.T) {
	entityIDEnvName := "DD_ENTITY_ID"
	ddEnvName := "DD_ENV"
	ddServiceName := "DD_SERVICE"
	ddVersionName := "DD_VERSION"

	defer func() { os.Unsetenv(entityIDEnvName) }()
	defer func() { os.Unsetenv(ddEnvName) }()
	defer func() { os.Unsetenv(ddServiceName) }()
	defer func() { os.Unsetenv(ddVersionName) }()

	os.Setenv(entityIDEnvName, "test_id")
	os.Setenv(ddEnvName, "test_env")
	os.Setenv(ddServiceName, "test_service")
	os.Setenv(ddVersionName, "test_version")

	expectedTags := []string{"tag1", "tag2", "dd.internal.entity_id:test_id", "env:test_env",
		"service:test_service", "version:test_version"}
	ts, client := newClientAndTestServer(t,
		"udp",
		"localhost:8765",
		expectedTags,
		WithTags([]string{"tag1", "tag2"}),
	)

	ts.sendAllAndAssert(t, client)

	sort.Strings(expectedTags)
	sort.Strings(client.tags)
	assert.Equal(t, expectedTags, client.tags)
}

func TestKnownEnvTagsEmptyString(t *testing.T) {
	entityIDEnvName := "DD_ENTITY_ID"
	ddEnvName := "DD_ENV"
	ddServiceName := "DD_SERVICE"
	ddVersionName := "DD_VERSION"

	defer func() { os.Unsetenv(entityIDEnvName) }()
	defer func() { os.Unsetenv(ddEnvName) }()
	defer func() { os.Unsetenv(ddServiceName) }()
	defer func() { os.Unsetenv(ddVersionName) }()

	os.Setenv(entityIDEnvName, "")
	os.Setenv(ddEnvName, "")
	os.Setenv(ddServiceName, "")
	os.Setenv(ddVersionName, "")

	ts, client := newClientAndTestServer(t,
		"udp",
		"localhost:8765",
		nil,
	)

	assert.Len(t, client.tags, 0)
	ts.sendAllAndAssert(t, client)
}

func TestContainerIDWithEntityID(t *testing.T) {
	resetContainerID()

	entityIDEnvName := "DD_ENTITY_ID"
	defer func() {
		os.Unsetenv(entityIDEnvName)
		resetContainerID()
	}()
	os.Setenv(entityIDEnvName, "pod-uid")

	expectedTags := []string{"dd.internal.entity_id:pod-uid"}
	ts, client := newClientAndTestServer(t,
		"udp",
		"localhost:8765",
		expectedTags,
		WithContainerID("fake-container-id"),
	)

	sort.Strings(client.tags)
	assert.Equal(t, expectedTags, client.tags)
	ts.assertContainerID(t, "fake-container-id")
	ts.sendAllAndAssert(t, client)
}

func TestContainerIDWithoutEntityID(t *testing.T) {
	resetContainerID()
	os.Unsetenv("DD_ENTITY_ID")

	defer func() {
		resetContainerID()
	}()

	ts, client := newClientAndTestServer(t,
		"udp",
		"localhost:8765",
		[]string{},
		WithContainerID("fake-container-id"),
	)

	ts.assertContainerID(t, "fake-container-id")
	ts.sendAllAndAssert(t, client)
}

func TestOriginDetectionDisabled(t *testing.T) {
	resetContainerID()
	os.Unsetenv("DD_ENTITY_ID")

	originDetectionEnvName := "DD_ORIGIN_DETECTION_ENABLED"
	defer func() { os.Unsetenv(originDetectionEnvName) }()
	os.Setenv(originDetectionEnvName, "false")

	ts, client := newClientAndTestServer(t,
		"udp",
		"localhost:8765",
		[]string{},
	)

	ts.assertContainerID(t, "")
	ts.sendAllAndAssert(t, client)
}

func TestOriginDetectionEnabledWithEntityID(t *testing.T) {
	resetContainerID()

	entityIDEnvName := "DD_ENTITY_ID"
	defer func() {
		os.Unsetenv(entityIDEnvName)
		resetContainerID()
	}()
	os.Setenv(entityIDEnvName, "pod-uid")

	originDetectionEnvName := "DD_ORIGIN_DETECTION_ENABLED"
	defer func() { os.Unsetenv(originDetectionEnvName) }()
	os.Setenv(originDetectionEnvName, "true")

	expectedTags := []string{"dd.internal.entity_id:pod-uid"}
	ts, client := newClientAndTestServer(t,
		"udp",
		"localhost:8765",
		expectedTags,
		WithContainerID("fake-container-id"),
	)

	sort.Strings(client.tags)
	assert.Equal(t, expectedTags, client.tags)
	ts.assertContainerID(t, "fake-container-id")
	ts.sendAllAndAssert(t, client)
}

func TestPipelineWithGlobalTagsAndEnv(t *testing.T) {
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

func TestTelemetryAllOptions(t *testing.T) {
	orig := os.Getenv("DD_ENV")
	os.Setenv("DD_ENV", "test")
	defer os.Setenv("DD_ENV", orig)

	ts, client := newClientAndTestServer(t,
		"udp",
		"localhost:8765",
		[]string{"tag1", "tag2", "env:test"},
		WithExtendedClientSideAggregation(),
		WithTags([]string{"tag1", "tag2"}),
		WithNamespace("test_namespace"),
	)

	ts.sendAllAndAssert(t, client)
}

type testCase struct {
	opt      []Option
	testFunc func(*testing.T, *testServer, *Client)
}

func getTestMap() map[string]testCase {
	return map[string]testCase{
		"Default": testCase{
			[]Option{},
			func(t *testing.T, ts *testServer, client *Client) {
				ts.sendAllAndAssert(t, client)
			},
		},
		"Default without aggregation": testCase{
			[]Option{
				WithoutClientSideAggregation(),
			},
			func(t *testing.T, ts *testServer, client *Client) {
				ts.sendAllAndAssert(t, client)
			},
		},
		"With namespace": testCase{
			[]Option{
				WithNamespace("test_namespace"),
			},
			func(t *testing.T, ts *testServer, client *Client) {
				ts.sendAllAndAssert(t, client)
			},
		},
		"With namespace dot": testCase{
			[]Option{
				WithNamespace("test_namespace."),
			},
			func(t *testing.T, ts *testServer, client *Client) {
				ts.sendAllAndAssert(t, client)
			},
		},
		"With max messages per payload": testCase{
			[]Option{
				WithMaxMessagesPerPayload(5),
				// Make sure we hit the maxMessagesPerPayload before hitting the flush timeout
				WithBufferFlushInterval(3 * time.Second),
				WithWorkersCount(1),
			},
			func(t *testing.T, ts *testServer, client *Client) {
				ts.sendAllAndAssert(t, client)
				// We send 4 non aggregated metrics, 1 service_check and 1 event. So 2 reads (5 items per
				// payload). Then we flush the aggregator that will send 5 metrics, so 1 read. Finally,
				// the telemetry is 18 metrics flushed at a different time so 4 more payload for a
				// total of 8 reads on the network
				ts.assertNbRead(t, 8)
			},
		},
		"With max messages per payload + WithoutClientSideAggregation": testCase{
			[]Option{
				WithMaxMessagesPerPayload(5),
				// Make sure we hit the maxMessagesPerPayload before hitting the flush timeout
				WithBufferFlushInterval(3 * time.Second),
				WithoutClientSideAggregation(),
				WithWorkersCount(1),
			},
			func(t *testing.T, ts *testServer, client *Client) {
				ts.sendAllAndAssert(t, client)
				// We send 9 non aggregated metrics, 1 service_check and 1 event. So 3 reads (5 items
				// per payload). Then the telemetry is 18 metrics flushed at a different time so 4 more
				// payload for a total of 8 reads on the network
				ts.assertNbRead(t, 7)
			},
		},
		"ChannelMode without client side aggregation": testCase{
			[]Option{
				WithoutClientSideAggregation(),
				WithChannelMode(),
			},
			func(t *testing.T, ts *testServer, client *Client) {
				ts.sendAllAndAssert(t, client)
			},
		},
		"Basic client side aggregation": testCase{
			[]Option{},
			func(t *testing.T, ts *testServer, client *Client) {
				expectedMetrics := ts.sendAllMetricsForBasicAggregation(client)
				ts.assert(t, client, expectedMetrics)
			},
		},
		"Extended client side aggregation": testCase{
			[]Option{
				WithExtendedClientSideAggregation(),
			},
			func(t *testing.T, ts *testServer, client *Client) {
				expectedMetrics := ts.sendAllMetricsForExtendedAggregation(client)
				ts.assert(t, client, expectedMetrics)
			},
		},
		"Extended client side aggregation + Maximum number of Samples": testCase{
			[]Option{
				WithExtendedClientSideAggregation(),
				WithMaxSamplesPerContext(2),
			},
			func(t *testing.T, ts *testServer, client *Client) {
				expectedMetrics := ts.sendAllMetricsForExtendedAggregationAndMaxSamples(client)
				ts.assert(t, client, expectedMetrics)
			},
		},
		"Basic client side aggregation + ChannelMode": testCase{
			[]Option{
				WithChannelMode(),
			},
			func(t *testing.T, ts *testServer, client *Client) {
				expectedMetrics := ts.sendAllMetricsForBasicAggregation(client)
				ts.assert(t, client, expectedMetrics)
			},
		},
		"Extended client side aggregation + ChannelMode": testCase{
			[]Option{
				WithExtendedClientSideAggregation(),
				WithChannelMode(),
			},
			func(t *testing.T, ts *testServer, client *Client) {
				expectedMetrics := ts.sendAllMetricsForExtendedAggregation(client)
				ts.assert(t, client, expectedMetrics)
			},
		},
		"Basic Extended client side aggregation + Maximum number of Samples + ChannelMode": testCase{
			[]Option{
				WithExtendedClientSideAggregation(),
				WithMaxSamplesPerContext(2),
				WithChannelMode(),
				WithoutTelemetry(),
			},
			func(t *testing.T, ts *testServer, client *Client) {
				expectedMetrics := ts.sendExtendedBasicAggregationMetrics(client)
				ts.assert(t, client, expectedMetrics)
			},
		},
	}
}

type testCaseDirect struct {
	opt      []Option
	testFunc func(*testing.T, *testServer, *ClientDirect)
}

func getTestMapDirect() map[string]testCaseDirect {
	return map[string]testCaseDirect{
		"Basic Extended client side aggregation + Maximum number of Samples + ChannelMode + pre-sampled distributions": testCaseDirect{
			[]Option{
				WithExtendedClientSideAggregation(),
				WithMaxSamplesPerContext(2),
				WithChannelMode(),
				WithoutTelemetry(),
			},
			func(t *testing.T, ts *testServer, client *ClientDirect) {
				expectedMetrics := ts.sendExtendedBasicAggregationMetricsWithPreAggregatedSamples(client)
				ts.assert(t, client.Client, expectedMetrics)
			},
		},
	}
}

func TestFullPipelineUDP(t *testing.T) {
	for testName, c := range getTestMap() {
		t.Run(testName, func(t *testing.T) {
			ts, client := newClientAndTestServer(t,
				"udp",
				"localhost:8765",
				nil,
				c.opt...,
			)
			c.testFunc(t, ts, client)
		})
	}
}

func TestFullPipelineUDPDirectClient(t *testing.T) {
	for testName, c := range getTestMapDirect() {
		t.Run(testName, func(t *testing.T) {
			ts, client := newClientDirectAndTestServer(t,
				"udp",
				"localhost:8765",
				nil,
				c.opt...,
			)
			c.testFunc(t, ts, client)
		})
	}
}

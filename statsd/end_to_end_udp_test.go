package statsd

import (
	"os"
	"sort"
	"testing"

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
				WithWorkersCount(1),
			},
			func(t *testing.T, ts *testServer, client *Client) {
				ts.sendAllAndAssert(t, client)
				// we send 11 messages, with a max of 5 per payload we expect 3 reads from the network.
				// The telemetry is 18 metrics flushed at a different interval so 4 more payload for a
				// total of 7 reads on the network
				ts.assertNbRead(t, 7)
			},
		},
		"ChannelMode": testCase{
			[]Option{
				WithChannelMode(),
			},
			func(t *testing.T, ts *testServer, client *Client) {
				ts.sendAllAndAssert(t, client)
			},
		},
		"Basic client side aggregation": testCase{
			[]Option{
				WithClientSideAggregation(),
			},
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
		"Basic client side aggregation + ChannelMode": testCase{
			[]Option{
				WithClientSideAggregation(),
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

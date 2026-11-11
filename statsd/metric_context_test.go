package statsd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newAggClientExForTest builds a minimal ClientEx wired to an aggregator in
// mutex mode. It is enough to exercise the MetricContext sample paths without
// any networking.
func newAggClientExForTest(shards int, maxSamples int64) *ClientEx {
	c := &ClientEx{
		telemetry:          &statsdTelemetry{},
		defaultCardinality: CardinalityNotSet,
		aggregatorMode:     mutexMode,
	}
	c.agg = newAggregator(c, maxSamples, shards)
	c.aggExtended = c.agg
	return c
}

// TestMetricContextKeyConstruction ensures that the key, hash, tags offset and
// tags substring precomputed by NewMetricContext match exactly what the
// per-sample appendContext path produces. If these ever diverge, a handle and a
// direct call for the same series would land on different map entries.
func TestMetricContextKeyConstruction(t *testing.T) {
	tests := []struct {
		name        string
		metric      string
		tags        []string
		cardinality Cardinality
	}{
		{"no tags no cardinality", "metric.name", nil, CardinalityNotSet},
		{"tags no cardinality", "metric.name", []string{"a:1", "b:2"}, CardinalityNotSet},
		{"single tag", "metric.name", []string{"only:1"}, CardinalityNotSet},
		{"cardinality no tags", "metric.name", nil, CardinalityHigh},
		{"cardinality with tags", "metric.name", []string{"a:1", "b:2"}, CardinalityLow},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &ClientEx{defaultCardinality: CardinalityNotSet}
			mc := c.NewMetricContext(tc.metric, tc.tags, tc.cardinality)

			cardString := tc.cardinality.String()
			wantBuf, wantTagsStart := appendContext(nil, tc.metric, tc.tags, cardString)
			wantKey := string(wantBuf)

			assert.Equal(t, wantKey, mc.key, "key")
			assert.Equal(t, hashString32(wantKey), mc.hash, "hash")
			assert.Equal(t, wantTagsStart, mc.tagsStart, "tagsStart")

			wantStringTags := ""
			if wantTagsStart >= 0 {
				wantStringTags = wantKey[wantTagsStart:]
			}
			assert.Equal(t, wantStringTags, mc.stringTags, "stringTags")
			assert.Equal(t, tc.cardinality, mc.cardinality, "cardinality")
		})
	}
}

// TestMetricContextAggregatorInterop verifies that samples sent through a
// MetricContext aggregate into the same map entries as direct aggregator calls,
// for every metric type.
func TestMetricContextAggregatorInterop(t *testing.T) {
	c := newAggClientExForTest(8, 0)
	a := c.agg
	tags := []string{"tag1", "tag2"}

	// Count: one direct call and one via the handle must sum in a single entry.
	mcCount := c.NewMetricContext("countTest", tags)
	require.NoError(t, mcCount.Count(21))
	require.NoError(t, a.count("countTest", 21, tags, CardinalityNotSet))
	counts := getAllCounts(a)
	require.Len(t, counts, 1)
	require.Contains(t, counts, "countTest:tag1,tag2")
	assert.Equal(t, int64(42), counts["countTest:tag1,tag2"].value)

	// Incr / Decr are Count(1) / Count(-1).
	require.NoError(t, mcCount.Incr())
	require.NoError(t, mcCount.Decr())
	assert.Equal(t, int64(42), getAllCounts(a)["countTest:tag1,tag2"].value)

	// Gauge keeps the last sample.
	mcGauge := c.NewMetricContext("gaugeTest", tags)
	require.NoError(t, mcGauge.Gauge(1))
	require.NoError(t, a.gauge("gaugeTest", 21, tags, CardinalityNotSet))
	require.NoError(t, mcGauge.Gauge(7))
	gauges := getAllGauges(a)
	require.Len(t, gauges, 1)
	require.Contains(t, gauges, "gaugeTest:tag1,tag2")

	// Set aggregates unique values.
	mcSet := c.NewMetricContext("setTest", tags)
	require.NoError(t, mcSet.Set("v1"))
	require.NoError(t, a.set("setTest", "v2", tags, CardinalityNotSet))
	require.NoError(t, mcSet.Set("v1"))
	sets := getAllSets(a)
	require.Len(t, sets, 1)
	require.Contains(t, sets, "setTest:tag1,tag2")
	assert.Len(t, sets["setTest:tag1,tag2"].data, 2)

	// Buffered types share the context store with direct calls.
	mcHist := c.NewMetricContext("histoTest", tags)
	require.NoError(t, mcHist.Histogram(3, 1))
	require.NoError(t, a.histogram("histoTest", 3, tags, 1, CardinalityNotSet))
	assert.Len(t, a.histograms.values, 1)
	assert.Contains(t, a.histograms.values, "histoTest:tag1,tag2")

	mcDist := c.NewMetricContext("distTest", tags)
	require.NoError(t, mcDist.Distribution(4, 1))
	assert.Len(t, a.distributions.values, 1)
	assert.Contains(t, a.distributions.values, "distTest:tag1,tag2")

	mcTiming := c.NewMetricContext("timingTest", tags)
	require.NoError(t, mcTiming.Timing(5*time.Second, 1))
	require.NoError(t, mcTiming.TimeInMilliseconds(6, 1))
	assert.Len(t, a.timings.values, 1)
	assert.Contains(t, a.timings.values, "timingTest:tag1,tag2")
}

// TestMetricContextNoTagsInterop checks the name-only fast path: a handle with
// no tags must use the metric name as key, matching the direct no-tags path.
func TestMetricContextNoTagsInterop(t *testing.T) {
	c := newAggClientExForTest(8, 0)
	a := c.agg

	mc := c.NewMetricContext("countTest", nil)
	require.Equal(t, "countTest", mc.key)
	require.Equal(t, -1, mc.tagsStart)

	require.NoError(t, mc.Count(2))
	require.NoError(t, a.count("countTest", 3, nil, CardinalityNotSet))
	counts := getAllCounts(a)
	require.Len(t, counts, 1)
	require.Contains(t, counts, "countTest")
	assert.Equal(t, int64(5), counts["countTest"].value)
}

// TestMetricContextEndToEnd exercises the full client path, including the
// channel-mode plumbing for buffered metrics, and asserts the wire output is
// identical to what the regular client methods produce.
func TestMetricContextEndToEnd(t *testing.T) {
	withoutOriginGlobals(t)

	ts, client := newClientAndTestServer(t,
		"udp",
		"localhost:8765",
		nil,
		WithExtendedClientSideAggregation(),
		WithChannelMode(),
		WithoutTelemetry(),
	)

	tags := []string{"custom:1", "custom:2"}

	client.NewMetricContext("gauge", tags).Gauge(1)
	client.NewMetricContext("count", tags).Count(2)
	client.NewMetricContext("set", tags).Set("id")
	client.NewMetricContext("histo", tags).Histogram(4, 1)
	client.NewMetricContext("distro", tags).Distribution(5, 1)
	client.NewMetricContext("timing", tags).TimeInMilliseconds(6, 1)

	finalTags := ts.getFinalTags(tags...)
	containerID := ts.getContainerID()

	expected := []string{
		"gauge:1|g" + finalTags + containerID,
		"count:2|c" + finalTags + containerID,
		"set:id|s" + finalTags + containerID,
		"histo:4|h" + finalTags + containerID,
		"distro:5|d" + finalTags + containerID,
		"timing:6.000000|ms" + finalTags + containerID,
	}

	ts.assert(t, client, expected)
}

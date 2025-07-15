package statsd

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAggregatorSample(t *testing.T) {
	a := newAggregator(nil, 0)

	tags := []string{"tag1", "tag2"}

	for i := 0; i < 2; i++ {
		a.gauge("gaugeTest", 21, tags, defaultTagCardinality)
		assert.Len(t, a.gauges, 1)
		assert.Contains(t, a.gauges, "gaugeTest:tag1,tag2")

		a.count("countTest", 21, tags, defaultTagCardinality)
		assert.Len(t, a.counts, 1)
		assert.Contains(t, a.counts, "countTest:tag1,tag2")

		a.set("setTest", "value1", tags, defaultTagCardinality)
		assert.Len(t, a.sets, 1)
		assert.Contains(t, a.sets, "setTest:tag1,tag2")

		a.set("setTest", "value1", tags, defaultTagCardinality)
		assert.Len(t, a.sets, 1)
		assert.Contains(t, a.sets, "setTest:tag1,tag2")

		a.histogram("histogramTest", 21, tags, 1, defaultTagCardinality)
		assert.Len(t, a.histograms.values, 1)
		assert.Contains(t, a.histograms.values, "histogramTest:tag1,tag2")

		a.distribution("distributionTest", 21, tags, 1, defaultTagCardinality)
		assert.Len(t, a.distributions.values, 1)
		assert.Contains(t, a.distributions.values, "distributionTest:tag1,tag2")

		a.timing("timingTest", 21, tags, 1, defaultTagCardinality)
		assert.Len(t, a.timings.values, 1)
		assert.Contains(t, a.timings.values, "timingTest:tag1,tag2")
	}
}

func TestAggregatorFlush(t *testing.T) {
	a := newAggregator(nil, 0)

	tags := []string{"tag1", "tag2"}

	a.gauge("gaugeTest1", 21, tags, CardinalityParameter{card: "low"})
	a.gauge("gaugeTest1", 10, tags, CardinalityParameter{card: "low"})
	a.gauge("gaugeTest2", 15, tags, CardinalityParameter{card: "low"})

	a.count("countTest1", 21, tags, CardinalityParameter{card: "low"})
	a.count("countTest1", 10, tags, CardinalityParameter{card: "low"})
	a.count("countTest2", 1, tags, CardinalityParameter{card: "low"})

	a.set("setTest1", "value1", tags, CardinalityParameter{card: "low"})
	a.set("setTest1", "value1", tags, CardinalityParameter{card: "low"})
	a.set("setTest1", "value2", tags, CardinalityParameter{card: "low"})
	a.set("setTest2", "value1", tags, CardinalityParameter{card: "low"})

	a.histogram("histogramTest1", 21, tags, 1, CardinalityParameter{card: "low"})
	a.histogram("histogramTest1", 22, tags, 1, CardinalityParameter{card: "low"})
	a.histogram("histogramTest2", 23, tags, 1, CardinalityParameter{card: "low"})

	a.distribution("distributionTest1", 21, tags, 1, CardinalityParameter{card: "low"})
	a.distribution("distributionTest1", 22, tags, 1, CardinalityParameter{card: "low"})
	a.distribution("distributionTest2", 23, tags, 1, CardinalityParameter{card: "low"})

	a.timing("timingTest1", 21, tags, 1, CardinalityParameter{card: "low"})
	a.timing("timingTest1", 22, tags, 1, CardinalityParameter{card: "low"})
	a.timing("timingTest2", 23, tags, 1, CardinalityParameter{card: "low"})

	metrics := a.flushMetrics()

	assert.Len(t, a.gauges, 0)
	assert.Len(t, a.counts, 0)
	assert.Len(t, a.sets, 0)
	assert.Len(t, a.histograms.values, 0)
	assert.Len(t, a.distributions.values, 0)
	assert.Len(t, a.timings.values, 0)

	assert.Len(t, metrics, 13)

	sort.Slice(metrics, func(i, j int) bool {
		if metrics[i].metricType == metrics[j].metricType {
			res := strings.Compare(metrics[i].name, metrics[j].name)
			// this happens fo set
			if res == 0 {
				return strings.Compare(metrics[i].svalue, metrics[j].svalue) != 1
			}
			return res != 1
		}
		return metrics[i].metricType < metrics[j].metricType
	})

	assert.Equal(t, []metric{
		metric{
			metricType:   gauge,
			name:         "gaugeTest1",
			tags:         tags,
			rate:         1,
			fvalue:       float64(10),
			overrideCard: CardinalityParameter{card: "low"},
		},
		metric{
			metricType:   gauge,
			name:         "gaugeTest2",
			tags:         tags,
			rate:         1,
			fvalue:       float64(15),
			overrideCard: CardinalityParameter{card: "low"},
		},
		metric{
			metricType:   count,
			name:         "countTest1",
			tags:         tags,
			rate:         1,
			ivalue:       int64(31),
			overrideCard: CardinalityParameter{card: "low"},
		},
		metric{
			metricType:   count,
			name:         "countTest2",
			tags:         tags,
			rate:         1,
			ivalue:       int64(1),
			overrideCard: CardinalityParameter{card: "low"},
		},
		metric{
			metricType:   histogramAggregated,
			name:         "histogramTest1",
			stags:        strings.Join(tags, tagSeparatorSymbol),
			rate:         1,
			fvalues:      []float64{21.0, 22.0},
			overrideCard: CardinalityParameter{card: "low"},
		},
		metric{
			metricType:   histogramAggregated,
			name:         "histogramTest2",
			stags:        strings.Join(tags, tagSeparatorSymbol),
			rate:         1,
			fvalues:      []float64{23.0},
			overrideCard: CardinalityParameter{card: "low"},
		},
		metric{
			metricType:   distributionAggregated,
			name:         "distributionTest1",
			stags:        strings.Join(tags, tagSeparatorSymbol),
			rate:         1,
			fvalues:      []float64{21.0, 22.0},
			overrideCard: CardinalityParameter{card: "low"},
		},
		metric{
			metricType:   distributionAggregated,
			name:         "distributionTest2",
			stags:        strings.Join(tags, tagSeparatorSymbol),
			rate:         1,
			fvalues:      []float64{23.0},
			overrideCard: CardinalityParameter{card: "low"},
		},
		metric{
			metricType:   set,
			name:         "setTest1",
			tags:         tags,
			rate:         1,
			svalue:       "value1",
			overrideCard: CardinalityParameter{card: "low"},
		},
		metric{
			metricType:   set,
			name:         "setTest1",
			tags:         tags,
			rate:         1,
			svalue:       "value2",
			overrideCard: CardinalityParameter{card: "low"},
		},
		metric{
			metricType:   set,
			name:         "setTest2",
			tags:         tags,
			rate:         1,
			svalue:       "value1",
			overrideCard: CardinalityParameter{card: "low"},
		},
		metric{
			metricType:   timingAggregated,
			name:         "timingTest1",
			stags:        strings.Join(tags, tagSeparatorSymbol),
			rate:         1,
			fvalues:      []float64{21.0, 22.0},
			overrideCard: CardinalityParameter{card: "low"},
		},
		metric{
			metricType:   timingAggregated,
			name:         "timingTest2",
			stags:        strings.Join(tags, tagSeparatorSymbol),
			rate:         1,
			fvalues:      []float64{23.0},
			overrideCard: CardinalityParameter{card: "low"},
		},
	},
		metrics)
}

func TestAggregatorFlushWithMaxSamplesPerContext(t *testing.T) {
	// In this test we keep only 2 samples per context for metrics where it's relevant.
	maxSamples := int64(2)
	a := newAggregator(nil, maxSamples)

	tags := []string{"tag1", "tag2"}

	a.gauge("gaugeTest1", 21, tags, CardinalityParameter{card: "low"})
	a.gauge("gaugeTest1", 10, tags, CardinalityParameter{card: "low"})
	a.gauge("gaugeTest1", 15, tags, CardinalityParameter{card: "low"})

	a.count("countTest1", 21, tags, CardinalityParameter{card: "low"})
	a.count("countTest1", 10, tags, CardinalityParameter{card: "low"})
	a.count("countTest1", 1, tags, CardinalityParameter{card: "low"})

	a.set("setTest1", "value1", tags, CardinalityParameter{card: "low"})
	a.set("setTest1", "value1", tags, CardinalityParameter{card: "low"})
	a.set("setTest1", "value2", tags, CardinalityParameter{card: "low"})

	a.histogram("histogramTest1", 21, tags, 1, CardinalityParameter{card: "low"})
	a.histogram("histogramTest1", 22, tags, 1, CardinalityParameter{card: "low"})
	a.histogram("histogramTest1", 23, tags, 1, CardinalityParameter{card: "low"})

	a.distribution("distributionTest1", 21, tags, 1, CardinalityParameter{card: "low"})
	a.distribution("distributionTest1", 22, tags, 1, CardinalityParameter{card: "low"})
	a.distribution("distributionTest1", 23, tags, 1, CardinalityParameter{card: "low"})

	a.timing("timingTest1", 21, tags, 1, CardinalityParameter{card: "low"})
	a.timing("timingTest1", 22, tags, 1, CardinalityParameter{card: "low"})
	a.timing("timingTest1", 23, tags, 1, CardinalityParameter{card: "low"})

	metrics := a.flushMetrics()

	assert.Len(t, a.gauges, 0)
	assert.Len(t, a.counts, 0)
	assert.Len(t, a.sets, 0)
	assert.Len(t, a.histograms.values, 0)
	assert.Len(t, a.distributions.values, 0)
	assert.Len(t, a.timings.values, 0)

	assert.Len(t, metrics, 7)

	sort.Slice(metrics, func(i, j int) bool {
		if metrics[i].metricType == metrics[j].metricType {
			res := strings.Compare(metrics[i].name, metrics[j].name)
			// this happens fo set
			if res == 0 {
				return strings.Compare(metrics[i].svalue, metrics[j].svalue) != 1
			}
			return res != 1
		}
		return metrics[i].metricType < metrics[j].metricType
	})

	expectedMetrics := []metric{
		metric{
			metricType: gauge,
			name:       "gaugeTest1",
			tags:       tags,
			rate:       1,
			fvalue:     float64(10),
		},
		metric{
			metricType: count,
			name:       "countTest1",
			tags:       tags,
			rate:       1,
			ivalue:     int64(31),
		},
		metric{
			metricType: histogramAggregated,
			name:       "histogramTest1",
			stags:      strings.Join(tags, tagSeparatorSymbol),
			rate:       float64(maxSamples) / 3,
			fvalues:    []float64{21.0, 22.0, 23.0},
		},
		metric{
			metricType: distributionAggregated,
			name:       "distributionTest1",
			stags:      strings.Join(tags, tagSeparatorSymbol),
			rate:       float64(maxSamples) / 3,
			fvalues:    []float64{21.0, 22.0, 23.0},
		},
		metric{
			metricType: set,
			name:       "setTest1",
			tags:       tags,
			rate:       1,
			svalue:     "value1",
		},
		metric{
			metricType: set,
			name:       "setTest1",
			tags:       tags,
			rate:       1,
			svalue:     "value2",
		},
		metric{
			metricType: timingAggregated,
			name:       "timingTest1",
			stags:      strings.Join(tags, tagSeparatorSymbol),
			rate:       float64(maxSamples) / 3,
			fvalues:    []float64{21.0, 22.0, 23.0},
		},
	}

	for i, m := range metrics {
		assert.Equal(t, expectedMetrics[i].metricType, m.metricType)
		assert.Equal(t, expectedMetrics[i].name, m.name)
		assert.Equal(t, expectedMetrics[i].tags, m.tags)
		if m.metricType == timingAggregated || m.metricType == histogramAggregated || m.metricType == distributionAggregated {
			assert.Equal(t, expectedMetrics[i].rate, float64(len(m.fvalues))/float64(len(expectedMetrics[i].fvalues)))
			assert.Subset(t, expectedMetrics[i].fvalues, m.fvalues)
			assert.Len(t, m.fvalues, int(maxSamples))
		} else {
			assert.Equal(t, expectedMetrics[i].rate, m.rate)
			assert.Equal(t, expectedMetrics[i].fvalues, m.fvalues)
		}
	}
}

func TestAggregatorFlushConcurrency(t *testing.T) {
	a := newAggregator(nil, 0)

	var wg sync.WaitGroup
	wg.Add(10)

	tags := []string{"tag1", "tag2"}

	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()

			a.gauge("gaugeTest1", 21, tags, CardinalityParameter{card: "low"})
			a.count("countTest1", 21, tags, CardinalityParameter{card: "low"})
			a.set("setTest1", "value1", tags, CardinalityParameter{card: "low"})
			a.histogram("histogramTest1", 21, tags, 1, CardinalityParameter{card: "low"})
			a.distribution("distributionTest1", 21, tags, 1, CardinalityParameter{card: "low"})
			a.timing("timingTest1", 21, tags, 1, CardinalityParameter{card: "low"})
		}()
	}

	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()

			a.flushMetrics()
		}()
	}

	wg.Wait()
}

func TestAggregatorTagsCopy(t *testing.T) {
	a := newAggregator(nil, 0)
	tags := []string{"tag1", "tag2"}

	a.gauge("gauge", 21, tags, CardinalityParameter{card: "low"})
	a.count("count", 21, tags, CardinalityParameter{card: "low"})
	a.set("set", "test", tags, CardinalityParameter{card: "low"})

	tags[0] = "new_tags"

	metrics := a.flushMetrics()
	require.Len(t, metrics, 3)
	for _, m := range metrics {
		assert.Equal(t, []string{"tag1", "tag2"}, m.tags)
	}
}

func TestGetContextAndTags(t *testing.T) {
	tests := []struct {
		testName    string
		name        string
		tags        []string
		wantContext string
		wantTags    string
	}{
		{
			testName:    "no tags",
			name:        "name",
			tags:        nil,
			wantContext: "name:low",
			wantTags:    "",
		},
		{
			testName:    "one tag",
			name:        "name",
			tags:        []string{"tag1"},
			wantContext: "name:low|tag1",
			wantTags:    "tag1",
		},
		{
			testName:    "two tags",
			name:        "name",
			tags:        []string{"tag1", "tag2"},
			wantContext: "name:low|tag1,tag2",
			wantTags:    "tag1,tag2",
		},
	}
	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			gotContext, gotTags := getContextAndTags(test.name, test.tags, CardinalityParameter{card: "low"})
			assert.Equal(t, test.wantContext, gotContext)
			assert.Equal(t, test.wantTags, gotTags)
		})
	}
}

func BenchmarkGetContext(b *testing.B) {
	name := "test.metric"
	tags := []string{"tag:tag", "foo:bar"}
	for i := 0; i < b.N; i++ {
		getContext(name, tags, CardinalityParameter{card: "low"})
	}
	b.ReportAllocs()
}

func BenchmarkGetContextNoTags(b *testing.B) {
	name := "test.metric"
	var tags []string
	for i := 0; i < b.N; i++ {
		getContext(name, tags, CardinalityParameter{card: "low"})
	}
	b.ReportAllocs()
}

func TestAggregatorCardinalitySeparation(t *testing.T) {
	a := newAggregator(nil, 0)
	tags := []string{"env:prod", "service:api"}

	a.gauge("test.metric", 10, tags, CardinalityParameter{card: "low"})
	a.gauge("test.metric", 20, tags, CardinalityParameter{card: "high"})
	a.gauge("test.metric", 30, tags, CardinalityParameter{card: "low"})

	a.count("test.count", 5, tags, CardinalityParameter{card: "low"})
	a.count("test.count", 15, tags, CardinalityParameter{card: "high"})
	a.count("test.count", 25, tags, CardinalityParameter{card: "low"})

	a.set("test.set", "value1", tags, CardinalityParameter{card: "low"})
	a.set("test.set", "value2", tags, CardinalityParameter{card: "high"})
	a.set("test.set", "value3", tags, CardinalityParameter{card: "low"})

	metrics := a.flushMetrics()

	assert.Len(t, metrics, 7)

	// Verify gauge metrics are separated by cardinality.
	var lowGauge, highGauge metric
	for _, m := range metrics {
		if m.metricType == gauge && m.name == "test.metric" {
			if m.overrideCard.card == "low" {
				lowGauge = m
			} else if m.overrideCard.card == "high" {
				highGauge = m
			}
		}
	}

	assert.Equal(t, float64(30), lowGauge.fvalue)
	assert.Equal(t, float64(20), highGauge.fvalue)

	// Verify count metrics are separated by cardinality.
	var lowCount, highCount metric
	for _, m := range metrics {
		if m.metricType == count && m.name == "test.count" {
			if m.overrideCard.card == "low" {
				lowCount = m
			} else if m.overrideCard.card == "high" {
				highCount = m
			}
		}
	}

	assert.Equal(t, int64(30), lowCount.ivalue)
	assert.Equal(t, int64(15), highCount.ivalue)

	// Verify set metrics are separated by cardinality.
	var lowSetValues, highSetValues []string
	for _, m := range metrics {
		if m.metricType == set && m.name == "test.set" {
			if m.overrideCard.card == "low" {
				lowSetValues = append(lowSetValues, m.svalue)
			} else if m.overrideCard.card == "high" {
				highSetValues = append(highSetValues, m.svalue)
			}
		}
	}

	assert.Len(t, lowSetValues, 2)
	assert.Contains(t, lowSetValues, "value1")
	assert.Contains(t, lowSetValues, "value3")

	assert.Len(t, highSetValues, 1)
	assert.Contains(t, highSetValues, "value2")
}

func TestAggregatorCardinalityPreservation(t *testing.T) {
	a := newAggregator(nil, 0)
	tags := []string{"env:prod"}

	// Test that cardinality is preserved in flushed metrics.
	a.gauge("test.metric", 42, tags, CardinalityParameter{card: "low"})
	a.count("test.count", 100, tags, CardinalityParameter{card: "high"})
	a.set("test.set", "unique_value", tags, CardinalityParameter{card: "orchestrator"})

	metrics := a.flushMetrics()
	assert.Len(t, metrics, 3)

	// Verify cardinality is preserved in each metric type.
	for _, m := range metrics {
		switch m.metricType {
		case gauge:
			assert.Equal(t, "test.metric", m.name)
			assert.Equal(t, CardinalityParameter{card: "low"}, m.overrideCard)
		case count:
			assert.Equal(t, "test.count", m.name)
			assert.Equal(t, CardinalityParameter{card: "high"}, m.overrideCard)
		case set:
			assert.Equal(t, "test.set", m.name)
			assert.Equal(t, CardinalityParameter{card: "orchestrator"}, m.overrideCard)
		}
	}
}

func TestAggregatorCardinalityWithBufferedMetrics(t *testing.T) {
	a := newAggregator(nil, 0)
	tags := []string{"env:prod"}

	a.histogram("test.hist", 10, tags, 1, CardinalityParameter{card: "low"})
	a.histogram("test.hist", 20, tags, 1, CardinalityParameter{card: "high"})
	a.histogram("test.hist", 30, tags, 1, CardinalityParameter{card: "low"})

	a.distribution("test.dist", 15, tags, 1, CardinalityParameter{card: "low"})
	a.distribution("test.dist", 25, tags, 1, CardinalityParameter{card: "high"})
	a.distribution("test.dist", 35, tags, 1, CardinalityParameter{card: "low"})

	a.timing("test.timing", 100, tags, 1, CardinalityParameter{card: "low"})
	a.timing("test.timing", 200, tags, 1, CardinalityParameter{card: "high"})
	a.timing("test.timing", 300, tags, 1, CardinalityParameter{card: "low"})

	metrics := a.flushMetrics()

	assert.Len(t, metrics, 6)

	// Verify histogram metrics are separated by cardinality.
	var lowHist, highHist metric
	for _, m := range metrics {
		if m.metricType == histogramAggregated && m.name == "test.hist" {
			if m.overrideCard.card == "low" {
				lowHist = m
			} else if m.overrideCard.card == "high" {
				highHist = m
			}
		}
	}

	assert.Len(t, lowHist.fvalues, 2)
	assert.Len(t, highHist.fvalues, 1)
	assert.Contains(t, lowHist.fvalues, float64(10))
	assert.Contains(t, lowHist.fvalues, float64(30))
	assert.Contains(t, highHist.fvalues, float64(20))

	// Verify distribution metrics are separated by cardinality.
	var lowDist, highDist metric
	for _, m := range metrics {
		if m.metricType == distributionAggregated && m.name == "test.dist" {
			if m.overrideCard.card == "low" {
				lowDist = m
			} else if m.overrideCard.card == "high" {
				highDist = m
			}
		}
	}

	assert.Len(t, lowDist.fvalues, 2)
	assert.Len(t, highDist.fvalues, 1)
	assert.Contains(t, lowDist.fvalues, float64(15))
	assert.Contains(t, lowDist.fvalues, float64(35))
	assert.Contains(t, highDist.fvalues, float64(25))

	// Verify timing metrics are separated by cardinality.
	var lowTiming, highTiming metric
	for _, m := range metrics {
		if m.metricType == timingAggregated && m.name == "test.timing" {
			if m.overrideCard.card == "low" {
				lowTiming = m
			} else if m.overrideCard.card == "high" {
				highTiming = m
			}
		}
	}

	assert.Len(t, lowTiming.fvalues, 2)
	assert.Len(t, highTiming.fvalues, 1)
	assert.Contains(t, lowTiming.fvalues, float64(100))
	assert.Contains(t, lowTiming.fvalues, float64(300))
	assert.Contains(t, highTiming.fvalues, float64(200))
}

func TestAggregatorCardinalityEmptyVsNonEmpty(t *testing.T) {
	a := newAggregator(nil, 0)
	tags := []string{"env:prod"}

	a.gauge("test.metric", 10, tags, CardinalityParameter{card: ""})
	a.gauge("test.metric", 20, tags, CardinalityParameter{card: "low"})
	a.gauge("test.metric", 30, tags, CardinalityParameter{card: ""})

	metrics := a.flushMetrics()
	assert.Len(t, metrics, 2)

	var emptyCard, lowCard metric
	for _, m := range metrics {
		if m.metricType == gauge && m.name == "test.metric" {
			if m.overrideCard.card == "" {
				emptyCard = m
			} else if m.overrideCard.card == "low" {
				lowCard = m
			}
		}
	}

	assert.Equal(t, float64(30), emptyCard.fvalue)
	assert.Equal(t, float64(20), lowCard.fvalue)
}

func TestAggregatorCardinalityContextGeneration(t *testing.T) {
	// Test that getContextAndTags generates correct contexts for different cardinalities
	tests := []struct {
		name        string
		tags        []string
		cardinality CardinalityParameter
		wantContext string
		wantTags    string
	}{
		{
			name:        "test.metric",
			tags:        []string{"env:prod"},
			cardinality: CardinalityParameter{card: "low"},
			wantContext: "test.metric:low|env:prod",
			wantTags:    "env:prod",
		},
		{
			name:        "test.metric",
			tags:        []string{"env:prod"},
			cardinality: CardinalityParameter{card: "high"},
			wantContext: "test.metric:high|env:prod",
			wantTags:    "env:prod",
		},
		{
			name:        "test.metric",
			tags:        []string{"env:prod", "service:api"},
			cardinality: CardinalityParameter{card: "orchestrator"},
			wantContext: "test.metric:orchestrator|env:prod,service:api",
			wantTags:    "env:prod,service:api",
		},
		{
			name:        "test.metric",
			tags:        []string{},
			cardinality: CardinalityParameter{card: "low"},
			wantContext: "test.metric:low",
			wantTags:    "",
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%s_%s", test.name, test.cardinality.card), func(t *testing.T) {
			gotContext, gotTags := getContextAndTags(test.name, test.tags, test.cardinality)
			assert.Equal(t, test.wantContext, gotContext)
			assert.Equal(t, test.wantTags, gotTags)
		})
	}
}

func TestAggregatorCardinalityGlobalSettingAggregation(t *testing.T) {
	// Test that metrics without explicit cardinality are aggregated with metrics
	// that have the same cardinality as the global setting.

	// Set global cardinality to "low"
	initTagCardinality("low")
	defer resetTagCardinality()

	a := newAggregator(nil, 0)
	tags := []string{"env:prod"}

	// Add metrics with different cardinality scenarios
	// 1. No explicit cardinality (should use global "low")
	a.gauge("test.metric", 10, tags, CardinalityParameter{card: ""})

	// 2. Explicit cardinality matching global setting
	a.gauge("test.metric", 20, tags, CardinalityParameter{card: "low"})

	// 3. Different explicit cardinality
	a.gauge("test.metric", 30, tags, CardinalityParameter{card: "high"})

	// 4. Another metric with no explicit cardinality (should aggregate with first two)
	a.gauge("test.metric", 40, tags, CardinalityParameter{card: ""})

	metrics := a.flushMetrics()

	// Should have 2 metrics: one for "low" cardinality (aggregated), one for "high"
	assert.Len(t, metrics, 2)

	// Find the metrics by cardinality
	var lowCardMetric, highCardMetric metric
	for _, m := range metrics {
		if m.metricType == gauge && m.name == "test.metric" {
			if m.overrideCard.card == "low" {
				lowCardMetric = m
			} else if m.overrideCard.card == "high" {
				highCardMetric = m
			}
		}
	}

	assert.Equal(t, float64(40), lowCardMetric.fvalue)

	assert.Equal(t, float64(30), highCardMetric.fvalue)

	a.count("test.count", 5, tags, CardinalityParameter{card: ""})
	a.count("test.count", 15, tags, CardinalityParameter{card: "low"})
	a.count("test.count", 25, tags, CardinalityParameter{card: "high"})
	a.count("test.count", 35, tags, CardinalityParameter{card: ""})

	metrics = a.flushMetrics()

	assert.Len(t, metrics, 2)

	var lowCardCount, highCardCount metric
	for _, m := range metrics {
		if m.metricType == count && m.name == "test.count" {
			if m.overrideCard.card == "low" {
				lowCardCount = m
			} else if m.overrideCard.card == "high" {
				highCardCount = m
			}
		}
	}

	assert.Equal(t, int64(55), lowCardCount.ivalue)

	assert.Equal(t, int64(25), highCardCount.ivalue)

	a.set("test.set", "value1", tags, CardinalityParameter{card: ""})
	a.set("test.set", "value2", tags, CardinalityParameter{card: "low"})
	a.set("test.set", "value3", tags, CardinalityParameter{card: "high"})
	a.set("test.set", "value4", tags, CardinalityParameter{card: ""})

	metrics = a.flushMetrics()

	assert.Len(t, metrics, 4)

	var lowCardSetCount, highCardSetCount int
	var lowCardSetValues, highCardSetValues []string
	for _, m := range metrics {
		if m.metricType == set && m.name == "test.set" {
			if m.overrideCard.card == "low" {
				lowCardSetCount++
				lowCardSetValues = append(lowCardSetValues, m.svalue)
			} else if m.overrideCard.card == "high" {
				highCardSetCount++
				highCardSetValues = append(highCardSetValues, m.svalue)
			}
		}
	}

	assert.Equal(t, 3, lowCardSetCount)
	assert.Contains(t, lowCardSetValues, "value1")
	assert.Contains(t, lowCardSetValues, "value2")
	assert.Contains(t, lowCardSetValues, "value4")

	assert.Equal(t, 1, highCardSetCount)
	assert.Contains(t, highCardSetValues, "value3")
}

func TestAggregatorCardinalityNoGlobalSetting(t *testing.T) {
	// Test behavior when global cardinality is not set.
	// Reset to ensure no global setting.
	resetTagCardinality()

	a := newAggregator(nil, 0)
	tags := []string{"env:prod"}

	// Test case 1: No cardinality specified (should use empty cardinality).
	a.gauge("test.metric", 10, tags, CardinalityParameter{card: ""})
	a.gauge("test.metric", 20, tags, CardinalityParameter{card: ""})

	// Test case 2: Explicit cardinality specified.
	a.gauge("test.metric", 30, tags, CardinalityParameter{card: "low"})
	a.gauge("test.metric", 40, tags, CardinalityParameter{card: "low"})

	// Test case 3: Different explicit cardinality.
	a.gauge("test.metric", 50, tags, CardinalityParameter{card: "high"})

	metrics := a.flushMetrics()

	assert.Len(t, metrics, 3)

	var emptyCardMetric, lowCardMetric, highCardMetric metric
	for _, m := range metrics {
		if m.metricType == gauge && m.name == "test.metric" {
			if m.overrideCard.card == "" {
				emptyCardMetric = m
			} else if m.overrideCard.card == "low" {
				lowCardMetric = m
			} else if m.overrideCard.card == "high" {
				highCardMetric = m
			}
		}
	}

	assert.Equal(t, float64(20), emptyCardMetric.fvalue)
	assert.Equal(t, float64(40), lowCardMetric.fvalue)
	assert.Equal(t, float64(50), highCardMetric.fvalue)

	a.count("test.count", 5, tags, CardinalityParameter{card: ""})
	a.count("test.count", 15, tags, CardinalityParameter{card: ""})
	a.count("test.count", 25, tags, CardinalityParameter{card: "low"})
	a.count("test.count", 35, tags, CardinalityParameter{card: "low"})
	a.count("test.count", 45, tags, CardinalityParameter{card: "high"})

	metrics = a.flushMetrics()

	assert.Len(t, metrics, 3)

	var emptyCardCount, lowCardCount, highCardCount metric
	for _, m := range metrics {
		if m.metricType == count && m.name == "test.count" {
			if m.overrideCard.card == "" {
				emptyCardCount = m
			} else if m.overrideCard.card == "low" {
				lowCardCount = m
			} else if m.overrideCard.card == "high" {
				highCardCount = m
			}
		}
	}

	assert.Equal(t, int64(20), emptyCardCount.ivalue)
	assert.Equal(t, int64(60), lowCardCount.ivalue)
	assert.Equal(t, int64(45), highCardCount.ivalue)

	a.set("test.set", "value1", tags, CardinalityParameter{card: ""})
	a.set("test.set", "value2", tags, CardinalityParameter{card: ""})
	a.set("test.set", "value3", tags, CardinalityParameter{card: "low"})
	a.set("test.set", "value4", tags, CardinalityParameter{card: "low"})
	a.set("test.set", "value5", tags, CardinalityParameter{card: "high"})

	metrics = a.flushMetrics()

	assert.Len(t, metrics, 5)

	var emptyCardSetCount, lowCardSetCount, highCardSetCount int
	var emptyCardSetValues, lowCardSetValues, highCardSetValues []string
	for _, m := range metrics {
		if m.metricType == set && m.name == "test.set" {
			if m.overrideCard.card == "" {
				emptyCardSetCount++
				emptyCardSetValues = append(emptyCardSetValues, m.svalue)
			} else if m.overrideCard.card == "low" {
				lowCardSetCount++
				lowCardSetValues = append(lowCardSetValues, m.svalue)
			} else if m.overrideCard.card == "high" {
				highCardSetCount++
				highCardSetValues = append(highCardSetValues, m.svalue)
			}
		}
	}

	assert.Equal(t, 2, emptyCardSetCount)
	assert.Contains(t, emptyCardSetValues, "value1")
	assert.Contains(t, emptyCardSetValues, "value2")

	assert.Equal(t, 2, lowCardSetCount)
	assert.Contains(t, lowCardSetValues, "value3")
	assert.Contains(t, lowCardSetValues, "value4")

	assert.Equal(t, 1, highCardSetCount)
	assert.Contains(t, highCardSetValues, "value5")
}

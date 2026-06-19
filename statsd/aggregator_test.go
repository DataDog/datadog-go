package statsd

import (
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getAllCounts(a *aggregator) countsMap {
	counts := countsMap{}
	for i := range a.countShards {
		shard := &a.countShards[i]
		shard.RLock()
		for k, v := range shard.counts {
			counts[k] = v
		}
		shard.RUnlock()
	}
	return counts
}

func getAllGauges(a *aggregator) gaugesMap {
	gauges := gaugesMap{}
	for i := range a.gaugeShards {
		shard := &a.gaugeShards[i]
		shard.RLock()
		for k, v := range shard.gauges {
			gauges[k] = v
		}
		shard.RUnlock()
	}
	return gauges
}

func getAllSets(a *aggregator) setsMap {
	sets := setsMap{}
	for i := range a.setShards {
		shard := &a.setShards[i]
		shard.RLock()
		for k, v := range shard.sets {
			sets[k] = v
		}
		shard.RUnlock()
	}
	return sets
}

func TestAggregatorSample(t *testing.T) {
	a := newAggregator(nil, 0, 8)

	tags := []string{"tag1", "tag2"}

	for i := 0; i < 2; i++ {
		a.gauge("gaugeTest", 21, tags, CardinalityNotSet)
		gauges := getAllGauges(a)
		assert.Len(t, gauges, 1)
		assert.Contains(t, gauges, "gaugeTest:tag1,tag2")

		a.count("countTest", 21, tags, CardinalityNotSet)
		counts := getAllCounts(a)
		assert.Len(t, counts, 1)
		assert.Contains(t, counts, "countTest:tag1,tag2")

		a.set("setTest", "value1", tags, CardinalityNotSet)
		sets := getAllSets(a)
		assert.Len(t, sets, 1)
		assert.Contains(t, sets, "setTest:tag1,tag2")

		a.set("setTest", "value1", tags, CardinalityNotSet)
		sets = getAllSets(a)
		assert.Len(t, sets, 1)
		assert.Contains(t, sets, "setTest:tag1,tag2")

		a.histogram("histogramTest", 21, tags, 1, CardinalityNotSet)
		assert.Len(t, a.histograms.values, 1)
		assert.Contains(t, a.histograms.values, "histogramTest:tag1,tag2")

		a.distribution("distributionTest", 21, tags, 1, CardinalityNotSet)
		assert.Len(t, a.distributions.values, 1)
		assert.Contains(t, a.distributions.values, "distributionTest:tag1,tag2")

		a.timing("timingTest", 21, tags, 1, CardinalityNotSet)
		assert.Len(t, a.timings.values, 1)
		assert.Contains(t, a.timings.values, "timingTest:tag1,tag2")
	}
}

func TestAggregatorFlush(t *testing.T) {
	a := newAggregator(nil, 0, 8)

	tags := []string{"tag1", "tag2"}

	a.gauge("gaugeTest1", 21, tags, CardinalityLow)
	a.gauge("gaugeTest1", 10, tags, CardinalityLow)
	a.gauge("gaugeTest2", 15, tags, CardinalityLow)

	a.count("countTest1", 21, tags, CardinalityLow)
	a.count("countTest1", 10, tags, CardinalityLow)
	a.count("countTest2", 1, tags, CardinalityLow)

	a.set("setTest1", "value1", tags, CardinalityLow)
	a.set("setTest1", "value1", tags, CardinalityLow)
	a.set("setTest1", "value2", tags, CardinalityLow)
	a.set("setTest2", "value1", tags, CardinalityLow)

	a.histogram("histogramTest1", 21, tags, 1, CardinalityLow)
	a.histogram("histogramTest1", 22, tags, 1, CardinalityLow)
	a.histogram("histogramTest2", 23, tags, 1, CardinalityLow)

	a.distribution("distributionTest1", 21, tags, 1, CardinalityLow)
	a.distribution("distributionTest1", 22, tags, 1, CardinalityLow)
	a.distribution("distributionTest2", 23, tags, 1, CardinalityLow)

	a.timing("timingTest1", 21, tags, 1, CardinalityLow)
	a.timing("timingTest1", 22, tags, 1, CardinalityLow)
	a.timing("timingTest2", 23, tags, 1, CardinalityLow)

	metrics := a.flushMetrics()

	assert.Len(t, getAllGauges(a), 0)
	assert.Len(t, getAllCounts(a), 0)
	assert.Len(t, getAllSets(a), 0)
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
			metricType:  gauge,
			name:        "gaugeTest1",
			tags:        tags,
			rate:        1,
			fvalue:      float64(10),
			cardinality: CardinalityLow,
		},
		metric{
			metricType:  gauge,
			name:        "gaugeTest2",
			tags:        tags,
			rate:        1,
			fvalue:      float64(15),
			cardinality: CardinalityLow,
		},
		metric{
			metricType:  count,
			name:        "countTest1",
			tags:        tags,
			rate:        1,
			ivalue:      int64(31),
			cardinality: CardinalityLow,
		},
		metric{
			metricType:  count,
			name:        "countTest2",
			tags:        tags,
			rate:        1,
			ivalue:      int64(1),
			cardinality: CardinalityLow,
		},
		metric{
			metricType:  histogramAggregated,
			name:        "histogramTest1",
			stags:       strings.Join(tags, tagSeparatorSymbol),
			rate:        1,
			fvalues:     []float64{21.0, 22.0},
			cardinality: CardinalityLow,
		},
		metric{
			metricType:  histogramAggregated,
			name:        "histogramTest2",
			stags:       strings.Join(tags, tagSeparatorSymbol),
			rate:        1,
			fvalues:     []float64{23.0},
			cardinality: CardinalityLow,
		},
		metric{
			metricType:  distributionAggregated,
			name:        "distributionTest1",
			stags:       strings.Join(tags, tagSeparatorSymbol),
			rate:        1,
			fvalues:     []float64{21.0, 22.0},
			cardinality: CardinalityLow,
		},
		metric{
			metricType:  distributionAggregated,
			name:        "distributionTest2",
			stags:       strings.Join(tags, tagSeparatorSymbol),
			rate:        1,
			fvalues:     []float64{23.0},
			cardinality: CardinalityLow,
		},
		metric{
			metricType:  set,
			name:        "setTest1",
			tags:        tags,
			rate:        1,
			svalue:      "value1",
			cardinality: CardinalityLow,
		},
		metric{
			metricType:  set,
			name:        "setTest1",
			tags:        tags,
			rate:        1,
			svalue:      "value2",
			cardinality: CardinalityLow,
		},
		metric{
			metricType:  set,
			name:        "setTest2",
			tags:        tags,
			rate:        1,
			svalue:      "value1",
			cardinality: CardinalityLow,
		},
		metric{
			metricType:  timingAggregated,
			name:        "timingTest1",
			stags:       strings.Join(tags, tagSeparatorSymbol),
			rate:        1,
			fvalues:     []float64{21.0, 22.0},
			cardinality: CardinalityLow,
		},
		metric{
			metricType:  timingAggregated,
			name:        "timingTest2",
			stags:       strings.Join(tags, tagSeparatorSymbol),
			rate:        1,
			fvalues:     []float64{23.0},
			cardinality: CardinalityLow,
		},
	},
		metrics)
}

func TestAggregatorFlushWithMaxSamplesPerContext(t *testing.T) {
	// In this test we keep only 2 samples per context for metrics where it's relevant.
	maxSamples := int64(2)
	a := newAggregator(nil, maxSamples, 8)

	tags := []string{"tag1", "tag2"}

	a.gauge("gaugeTest1", 21, tags, CardinalityLow)
	a.gauge("gaugeTest1", 10, tags, CardinalityLow)
	a.gauge("gaugeTest1", 15, tags, CardinalityLow)

	a.count("countTest1", 21, tags, CardinalityLow)
	a.count("countTest1", 10, tags, CardinalityLow)
	a.count("countTest1", 1, tags, CardinalityLow)

	a.set("setTest1", "value1", tags, CardinalityLow)
	a.set("setTest1", "value1", tags, CardinalityLow)
	a.set("setTest1", "value2", tags, CardinalityLow)

	a.histogram("histogramTest1", 21, tags, 1, CardinalityLow)
	a.histogram("histogramTest1", 22, tags, 1, CardinalityLow)
	a.histogram("histogramTest1", 23, tags, 1, CardinalityLow)

	a.distribution("distributionTest1", 21, tags, 1, CardinalityLow)
	a.distribution("distributionTest1", 22, tags, 1, CardinalityLow)
	a.distribution("distributionTest1", 23, tags, 1, CardinalityLow)

	a.timing("timingTest1", 21, tags, 1, CardinalityLow)
	a.timing("timingTest1", 22, tags, 1, CardinalityLow)
	a.timing("timingTest1", 23, tags, 1, CardinalityLow)

	metrics := a.flushMetrics()

	assert.Len(t, getAllGauges(a), 0)
	assert.Len(t, getAllCounts(a), 0)
	assert.Len(t, getAllSets(a), 0)
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
	a := newAggregator(nil, 0, 8)

	var wg sync.WaitGroup
	wg.Add(10)

	tags := []string{"tag1", "tag2"}

	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()

			a.gauge("gaugeTest1", 21, tags, CardinalityLow)
			a.count("countTest1", 21, tags, CardinalityLow)
			a.set("setTest1", "value1", tags, CardinalityLow)
			a.histogram("histogramTest1", 21, tags, 1, CardinalityLow)
			a.distribution("distributionTest1", 21, tags, 1, CardinalityLow)
			a.timing("timingTest1", 21, tags, 1, CardinalityLow)
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
	a := newAggregator(nil, 0, 8)
	tags := []string{"tag1", "tag2"}

	a.gauge("gauge", 21, tags, CardinalityLow)
	a.count("count", 21, tags, CardinalityLow)
	a.set("set", "test", tags, CardinalityLow)

	tags[0] = "new_tags"

	metrics := a.flushMetrics()
	require.Len(t, metrics, 3)
	for _, m := range metrics {
		assert.Equal(t, []string{"tag1", "tag2"}, m.tags)
	}
}

func TestAggregatorCardinalitySeparation(t *testing.T) {
	a := newAggregator(nil, 0, 8)
	tags := []string{"env:prod", "service:api"}

	a.gauge("test.metric", 10, tags, CardinalityLow)
	a.gauge("test.metric", 20, tags, CardinalityHigh)
	a.gauge("test.metric", 30, tags, CardinalityLow)

	a.count("test.count", 5, tags, CardinalityLow)
	a.count("test.count", 15, tags, CardinalityHigh)
	a.count("test.count", 25, tags, CardinalityLow)

	a.set("test.set", "value1", tags, CardinalityLow)
	a.set("test.set", "value2", tags, CardinalityHigh)
	a.set("test.set", "value3", tags, CardinalityLow)

	metrics := a.flushMetrics()

	assert.Len(t, metrics, 7)

	// Verify gauge metrics are separated by cardinality.
	var lowGauge, highGauge metric
	for _, m := range metrics {
		if m.metricType == gauge && m.name == "test.metric" {
			if m.cardinality == CardinalityLow {
				lowGauge = m
			} else if m.cardinality == CardinalityHigh {
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
			if m.cardinality == CardinalityLow {
				lowCount = m
			} else if m.cardinality == CardinalityHigh {
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
			if m.cardinality == CardinalityLow {
				lowSetValues = append(lowSetValues, m.svalue)
			} else if m.cardinality == CardinalityHigh {
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

func TestAggregatorSampleNoTagsWithCardinality(t *testing.T) {
	a := newAggregator(nil, 0, 8)

	for i := 0; i < 2; i++ {
		a.gauge("gaugeTest", 21, nil, CardinalityLow)
		gauges := getAllGauges(a)
		assert.Len(t, gauges, 1)
		assert.Contains(t, gauges, "gaugeTest:low")

		a.count("countTest", 21, nil, CardinalityLow)
		counts := getAllCounts(a)
		assert.Len(t, counts, 1)
		assert.Contains(t, counts, "countTest:low")

		a.set("setTest", "value1", nil, CardinalityLow)
		sets := getAllSets(a)
		assert.Len(t, sets, 1)
		assert.Contains(t, sets, "setTest:low")

		a.histogram("histogramTest", 21, nil, 1, CardinalityLow)
		assert.Len(t, a.histograms.values, 1)
		assert.Contains(t, a.histograms.values, "histogramTest:low")

		a.distribution("distributionTest", 21, nil, 1, CardinalityLow)
		assert.Len(t, a.distributions.values, 1)
		assert.Contains(t, a.distributions.values, "distributionTest:low")

		a.timing("timingTest", 21, nil, 1, CardinalityLow)
		assert.Len(t, a.timings.values, 1)
		assert.Contains(t, a.timings.values, "timingTest:low")
	}
}

func TestAggregatorCardinalityPreservation(t *testing.T) {
	a := newAggregator(nil, 0, 8)
	tags := []string{"env:prod"}

	// Test that cardinality is preserved in flushed metrics.
	a.gauge("test.metric", 42, tags, CardinalityLow)
	a.count("test.count", 100, tags, CardinalityHigh)
	a.set("test.set", "unique_value", tags, CardinalityOrchestrator)

	metrics := a.flushMetrics()
	assert.Len(t, metrics, 3)

	// Verify cardinality is preserved in each metric type.
	for _, m := range metrics {
		switch m.metricType {
		case gauge:
			assert.Equal(t, "test.metric", m.name)
			assert.Equal(t, CardinalityLow, m.cardinality)
		case count:
			assert.Equal(t, "test.count", m.name)
			assert.Equal(t, CardinalityHigh, m.cardinality)
		case set:
			assert.Equal(t, "test.set", m.name)
			assert.Equal(t, CardinalityOrchestrator, m.cardinality)
		}
	}
}

func TestAggregatorCardinalityWithBufferedMetrics(t *testing.T) {
	a := newAggregator(nil, 0, 8)
	tags := []string{"env:prod"}

	a.histogram("test.hist", 10, tags, 1, CardinalityLow)
	a.histogram("test.hist", 20, tags, 1, CardinalityHigh)
	a.histogram("test.hist", 30, tags, 1, CardinalityLow)

	a.distribution("test.dist", 15, tags, 1, CardinalityLow)
	a.distribution("test.dist", 25, tags, 1, CardinalityHigh)
	a.distribution("test.dist", 35, tags, 1, CardinalityLow)

	a.timing("test.timing", 100, tags, 1, CardinalityLow)
	a.timing("test.timing", 200, tags, 1, CardinalityHigh)
	a.timing("test.timing", 300, tags, 1, CardinalityLow)

	metrics := a.flushMetrics()

	assert.Len(t, metrics, 6)

	// Verify histogram metrics are separated by cardinality.
	var lowHist, highHist metric
	for _, m := range metrics {
		if m.metricType == histogramAggregated && m.name == "test.hist" {
			if m.cardinality == CardinalityLow {
				lowHist = m
			} else if m.cardinality == CardinalityHigh {
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
			if m.cardinality == CardinalityLow {
				lowDist = m
			} else if m.cardinality == CardinalityHigh {
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
			if m.cardinality == CardinalityLow {
				lowTiming = m
			} else if m.cardinality == CardinalityHigh {
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
	a := newAggregator(nil, 0, 8)
	tags := []string{"env:prod"}

	a.gauge("test.metric", 10, tags, CardinalityNotSet)
	a.gauge("test.metric", 20, tags, CardinalityLow)
	a.gauge("test.metric", 30, tags, CardinalityNotSet)

	metrics := a.flushMetrics()
	assert.Len(t, metrics, 2)

	var emptyCard, lowCard metric
	for _, m := range metrics {
		if m.metricType == gauge && m.name == "test.metric" {
			if m.cardinality == CardinalityNotSet {
				emptyCard = m
			} else if m.cardinality == CardinalityLow {
				lowCard = m
			}
		}
	}

	assert.Equal(t, float64(30), emptyCard.fvalue)
	assert.Equal(t, float64(20), lowCard.fvalue)
}

// Verifies that count, gauge, and set correctly forward tags through all three
// context-buffer allocation paths: the small stack buffer (≤512 B), the large
// stack buffer (≤4 KiB), and heap allocation (>4 KiB)
func TestAggregatorContextBufferTiers(t *testing.T) {
	// Each tag is sized so that len("m") + len(":") + len(tag) lands in the
	// target tier: small ≤512, large 513–4096, heap ≥4097.
	smallTag := strings.Repeat("x", 10)                           // total 12
	largeTag := strings.Repeat("x", smallContextBufferSize+1)     // total 514
	heapTag := strings.Repeat("x", largeContextBufferSize+1)      // total 4098

	tiers := []struct {
		name string
		tags []string
	}{
		{"small", []string{smallTag}},
		{"large", []string{largeTag}},
		{"heap", []string{heapTag}},
	}

	for _, tier := range tiers {
		tier := tier
		t.Run("count/"+tier.name, func(t *testing.T) {
			a := newAggregator(nil, 0, 8)
			require.NoError(t, a.count("m", 3, tier.tags, CardinalityNotSet))
			require.NoError(t, a.count("m", 7, tier.tags, CardinalityNotSet))
			metrics := a.flushMetrics()
			require.Len(t, metrics, 1)
			assert.Equal(t, "m", metrics[0].name)
			assert.Equal(t, int64(10), metrics[0].ivalue)
			assert.Equal(t, tier.tags, metrics[0].tags)
		})
		t.Run("gauge/"+tier.name, func(t *testing.T) {
			a := newAggregator(nil, 0, 8)
			require.NoError(t, a.gauge("m", 1.0, tier.tags, CardinalityNotSet))
			require.NoError(t, a.gauge("m", 2.0, tier.tags, CardinalityNotSet))
			metrics := a.flushMetrics()
			require.Len(t, metrics, 1)
			assert.Equal(t, "m", metrics[0].name)
			assert.Equal(t, 2.0, metrics[0].fvalue)
			assert.Equal(t, tier.tags, metrics[0].tags)
		})
		t.Run("set/"+tier.name, func(t *testing.T) {
			a := newAggregator(nil, 0, 8)
			require.NoError(t, a.set("m", "a", tier.tags, CardinalityNotSet))
			require.NoError(t, a.set("m", "b", tier.tags, CardinalityNotSet))
			metrics := a.flushMetrics()
			require.Len(t, metrics, 2)
			for _, m := range metrics {
				assert.Equal(t, "m", m.name)
				assert.Equal(t, tier.tags, m.tags)
			}
		})
	}
}

// Verifies the StringContext fast path taken when a metric has no tags and
// CardinalityNotSet. The context key must be the bare metric name, aggregation
// semantics must match the regular path, and flushed metrics must carry nil
// tags and CardinalityNotSet.
func TestAggregatorNoTagsFastPath(t *testing.T) {
	a := newAggregator(nil, 0, 8)

	// Count: values accumulate
	require.NoError(t, a.count("my.count", 3, nil, CardinalityNotSet))
	require.NoError(t, a.count("my.count", 7, nil, CardinalityNotSet))
	counts := getAllCounts(a)
	assert.Len(t, counts, 1)
	assert.Contains(t, counts, "my.count")

	// Gauge: last value wins
	require.NoError(t, a.gauge("my.gauge", 1.0, nil, CardinalityNotSet))
	require.NoError(t, a.gauge("my.gauge", 2.0, nil, CardinalityNotSet))
	gauges := getAllGauges(a)
	assert.Len(t, gauges, 1)
	assert.Contains(t, gauges, "my.gauge")

	// Set: unique values tracked
	require.NoError(t, a.set("my.set", "a", nil, CardinalityNotSet))
	require.NoError(t, a.set("my.set", "a", nil, CardinalityNotSet))
	require.NoError(t, a.set("my.set", "b", nil, CardinalityNotSet))
	sets := getAllSets(a)
	assert.Len(t, sets, 1)
	assert.Contains(t, sets, "my.set")

	metrics := a.flushMetrics()

	var gotCount, gotGauge, gotSet *metric
	for i := range metrics {
		m := &metrics[i]
		switch m.name {
		case "my.count":
			gotCount = m
		case "my.gauge":
			gotGauge = m
		case "my.set":
			gotSet = m
		}
	}

	require.NotNil(t, gotCount)
	assert.Equal(t, int64(10), gotCount.ivalue)
	assert.Nil(t, gotCount.tags)
	assert.Equal(t, CardinalityNotSet, gotCount.cardinality)

	require.NotNil(t, gotGauge)
	assert.Equal(t, float64(2.0), gotGauge.fvalue)
	assert.Nil(t, gotGauge.tags)
	assert.Equal(t, CardinalityNotSet, gotGauge.cardinality)

	require.NotNil(t, gotSet)
	assert.Nil(t, gotSet.tags)
	assert.Equal(t, CardinalityNotSet, gotSet.cardinality)
}

// Checks that the no-tags fast path and the regular (tagged) path produce
// separate context keys and do not collide.
func TestAggregatorNoTagsFastPathIsolation(t *testing.T) {
	a := newAggregator(nil, 0, 8)
	tags := []string{"env:prod"}

	require.NoError(t, a.count("my.count", 1, nil, CardinalityNotSet))
	require.NoError(t, a.count("my.count", 2, tags, CardinalityNotSet))
	require.NoError(t, a.count("my.count", 4, tags, CardinalityLow))

	counts := getAllCounts(a)
	assert.Len(t, counts, 3)
	assert.Contains(t, counts, "my.count")
	assert.Contains(t, counts, "my.count:env:prod")
	assert.Contains(t, counts, "my.count:low|env:prod")
}

// Verifies the double-check locking in the StringContext fast path under
// concurrent writers.
func TestAggregatorNoTagsFastPathConcurrency(t *testing.T) {
	a := newAggregator(nil, 0, 8)

	const goroutines = 20
	const samplesEach = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < samplesEach; j++ {
				a.count("concurrent.count", 1, nil, CardinalityNotSet)
				a.gauge("concurrent.gauge", float64(j), nil, CardinalityNotSet)
				a.set("concurrent.set", "v", nil, CardinalityNotSet)
			}
		}()
	}
	wg.Wait()

	counts := getAllCounts(a)
	assert.Len(t, counts, 1)
	assert.Contains(t, counts, "concurrent.count")

	gauges := getAllGauges(a)
	assert.Len(t, gauges, 1)
	assert.Contains(t, gauges, "concurrent.gauge")

	sets := getAllSets(a)
	assert.Len(t, sets, 1)
	assert.Contains(t, sets, "concurrent.set")

	metrics := a.flushMetrics()
	var gotCount *metric
	for i := range metrics {
		if metrics[i].name == "concurrent.count" {
			gotCount = &metrics[i]
		}
	}
	require.NotNil(t, gotCount)
	assert.Equal(t, int64(goroutines*samplesEach), gotCount.ivalue)
}

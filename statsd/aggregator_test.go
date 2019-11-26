package statsd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func getAggregatedMetrics(a *aggregator) []*aggregatedMetric {
	var metrics []*aggregatedMetric
	for _, m := range a.metrics {
		metrics = append(metrics, m)
	}
	return metrics
}

func TestAggregator(t *testing.T) {
	a := newAggregator(nil, time.Hour*24)
	metric := metric{
		name:       "metric.test",
		tags:       []string{"test:1"},
		metricType: gauge,
		fvalue:     1,
	}
	a.addSample(metric)

	metrics := getAggregatedMetrics(a)
	assert.Len(t, metrics, 1)
	assert.Equal(t, metric.name, metrics[0].name)
	assert.ElementsMatch(t, metric.tags, metrics[0].tags)
	assert.Equal(t, metric.fvalue, metrics[0].fvalue)
}

func TestAggregatorUdpateGauge(t *testing.T) {
	a := newAggregator(nil, time.Hour*24)
	metric1 := metric{
		name:       "metric.test",
		tags:       []string{"test:1", "test:2"},
		metricType: gauge,
		fvalue:     1,
	}
	a.addSample(metric1)
	metric2 := metric{
		name:       "metric.test",
		tags:       []string{"test:2", "test:1"},
		metricType: gauge,
		fvalue:     250,
	}
	a.addSample(metric2)
	metric3 := metric{
		name:       "metric.test",
		tags:       []string{"test:2", "test:1"},
		metricType: gauge,
		fvalue:     1337,
	}
	a.addSample(metric3)

	metrics := getAggregatedMetrics(a)
	assert.Len(t, metrics, 1)
	assert.Equal(t, metric3.name, metrics[0].name)
	assert.ElementsMatch(t, metric3.tags, metrics[0].tags)
	assert.Equal(t, metric3.fvalue, metrics[0].fvalue)
}

func TestAggregatorUdpateCount(t *testing.T) {
	a := newAggregator(nil, time.Hour*24)
	metric1 := metric{
		name:       "metric.test.count",
		tags:       []string{"test:1", "test:2"},
		metricType: count,
		ivalue:     1,
	}
	a.addSample(metric1)
	metric2 := metric{
		name:       "metric.test.count",
		tags:       []string{"test:2", "test:1"},
		metricType: count,
		ivalue:     5,
	}
	a.addSample(metric2)
	metric3 := metric{
		name:       "metric.test.count",
		tags:       []string{"test:2", "test:1"},
		metricType: count,
		ivalue:     5,
	}
	a.addSample(metric3)

	metrics := getAggregatedMetrics(a)
	assert.Len(t, metrics, 1)
	assert.Equal(t, metric1.name, metrics[0].name)
	assert.ElementsMatch(t, metric1.tags, metrics[0].tags)
	assert.Equal(t, int64(11), metrics[0].ivalue)
}

func TestAggregatorDedupeTags(t *testing.T) {
	a := newAggregator(nil, time.Hour*24)
	metric1 := metric{
		name:       "metric.test",
		tags:       []string{"test:1", "test:2"},
		metricType: gauge,
		fvalue:     1,
	}
	a.addSample(metric1)
	metric2 := metric{
		name:       "metric.test",
		tags:       []string{"test:2", "test:1", "test:1"},
		metricType: gauge,
		fvalue:     2,
	}
	a.addSample(metric2)

	metrics := getAggregatedMetrics(a)
	assert.Len(t, metrics, 1)
	assert.Equal(t, metric2.name, metrics[0].name)
	assert.ElementsMatch(t, []string{"test:2", "test:1"}, metrics[0].tags)
	assert.Equal(t, metric2.fvalue, metrics[0].fvalue)
}

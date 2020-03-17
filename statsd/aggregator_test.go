package statsd

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAggregatorSample(t *testing.T) {
	a := newAggregator(nil)

	tags := []string{"tag1", "tag2"}

	a.gauge("gaugeTest", 21, tags, 1)
	assert.Len(t, a.gauges, 1)
	assert.Contains(t, a.gauges, "gaugeTest:tag1,tag2")

	a.count("countTest", 21, tags, 1)
	assert.Len(t, a.counts, 1)
	assert.Contains(t, a.counts, "countTest:tag1,tag2")

	a.set("setTest", "value1", tags, 1)
	assert.Len(t, a.sets, 1)
	assert.Contains(t, a.sets, "setTest:tag1,tag2")

	a.gauge("gaugeTest", 123, tags, 1)
	assert.Len(t, a.gauges, 1)
	assert.Contains(t, a.gauges, "gaugeTest:tag1,tag2")

	a.count("countTest", 10, tags, 1)
	assert.Len(t, a.counts, 1)
	assert.Contains(t, a.counts, "countTest:tag1,tag2")

	a.set("setTest", "value1", tags, 1)
	assert.Len(t, a.sets, 1)
	assert.Contains(t, a.sets, "setTest:tag1,tag2")
}

func TestAggregatorFlush(t *testing.T) {
	a := newAggregator(nil)

	tags := []string{"tag1", "tag2"}

	a.gauge("gaugeTest1", 21, tags, 1)
	a.gauge("gaugeTest1", 10, tags, 1)
	a.gauge("gaugeTest2", 15, tags, 1)

	a.count("countTest1", 21, tags, 1)
	a.count("countTest1", 10, tags, 1)
	a.count("countTest2", 1, tags, 1)

	a.set("setTest1", "value1", tags, 1)
	a.set("setTest1", "value1", tags, 1)
	a.set("setTest1", "value2", tags, 1)
	a.set("setTest2", "value1", tags, 1)

	metrics := a.flushMetrics()

	assert.Len(t, a.gauges, 0)
	assert.Len(t, a.counts, 0)
	assert.Len(t, a.sets, 0)

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

	assert.Equal(t, metrics, []metric{
		metric{
			metricType: gauge,
			name:       "gaugeTest1",
			tags:       tags,
			rate:       1,
			fvalue:     float64(10),
		},
		metric{
			metricType: gauge,
			name:       "gaugeTest2",
			tags:       tags,
			rate:       1,
			fvalue:     float64(15),
		},
		metric{
			metricType: count,
			name:       "countTest1",
			tags:       tags,
			rate:       1,
			ivalue:     int64(31),
		},
		metric{
			metricType: count,
			name:       "countTest2",
			tags:       tags,
			rate:       1,
			ivalue:     int64(1),
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
			metricType: set,
			name:       "setTest2",
			tags:       tags,
			rate:       1,
			svalue:     "value1",
		},
	})

}

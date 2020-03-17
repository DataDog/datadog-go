package statsd

import (
	"math"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCountMetric(t *testing.T) {
	c := newCountMetric("test", 21, []string{"tag1", "tag2"}, 1)
	assert.Equal(t, c.value, int64(21))
	assert.Equal(t, c.name, "test")
	assert.Equal(t, c.tags, []string{"tag1", "tag2"})
	assert.Equal(t, c.rate, 1.0)
}

func TestCountMetricSample(t *testing.T) {
	c := newCountMetric("test", 21, []string{"tag1", "tag2"}, 1)
	c.sample(12)
	assert.Equal(t, c.value, int64(33))
	assert.Equal(t, c.name, "test")
	assert.Equal(t, c.tags, []string{"tag1", "tag2"})
	assert.Equal(t, c.rate, 1.0)
}

func TestFlushUnsafeCountMetricSample(t *testing.T) {
	c := newCountMetric("test", 21, []string{"tag1", "tag2"}, 1)
	m := c.flushUnsafe()
	assert.Equal(t, m.metricType, count)
	assert.Equal(t, m.ivalue, int64(21))
	assert.Equal(t, m.name, "test")
	assert.Equal(t, m.tags, []string{"tag1", "tag2"})
	assert.Equal(t, m.rate, 1.0)

	c.sample(12)
	m = c.flushUnsafe()
	assert.Equal(t, m.metricType, count)
	assert.Equal(t, m.ivalue, int64(33))
	assert.Equal(t, m.name, "test")
	assert.Equal(t, m.tags, []string{"tag1", "tag2"})
	assert.Equal(t, m.rate, 1.0)
}

func TestNewGaugeMetric(t *testing.T) {
	g := newGaugeMetric("test", 21, []string{"tag1", "tag2"}, 1)
	assert.Equal(t, math.Float64frombits(g.value), float64(21))
	assert.Equal(t, g.name, "test")
	assert.Equal(t, g.tags, []string{"tag1", "tag2"})
	assert.Equal(t, g.rate, 1.0)
}

func TestGaugeMetricSample(t *testing.T) {
	g := newGaugeMetric("test", 21, []string{"tag1", "tag2"}, 1)
	g.sample(12)
	assert.Equal(t, math.Float64frombits(g.value), float64(12))
	assert.Equal(t, g.name, "test")
	assert.Equal(t, g.tags, []string{"tag1", "tag2"})
	assert.Equal(t, g.rate, 1.0)
}

func TestFlushUnsafeGaugeMetricSample(t *testing.T) {
	g := newGaugeMetric("test", 21, []string{"tag1", "tag2"}, 1)
	m := g.flushUnsafe()
	assert.Equal(t, m.metricType, gauge)
	assert.Equal(t, m.fvalue, float64(21))
	assert.Equal(t, m.name, "test")
	assert.Equal(t, m.tags, []string{"tag1", "tag2"})
	assert.Equal(t, m.rate, 1.0)

	g.sample(12)
	m = g.flushUnsafe()
	assert.Equal(t, m.metricType, gauge)
	assert.Equal(t, m.fvalue, float64(12))
	assert.Equal(t, m.name, "test")
	assert.Equal(t, m.tags, []string{"tag1", "tag2"})
	assert.Equal(t, m.rate, 1.0)
}

func TestNewSetMetric(t *testing.T) {
	s := newSetMetric("test", "value1", []string{"tag1", "tag2"}, 1)
	assert.Equal(t, s.data, map[string]struct{}{"value1": struct{}{}})
	assert.Equal(t, s.name, "test")
	assert.Equal(t, s.tags, []string{"tag1", "tag2"})
	assert.Equal(t, s.rate, 1.0)
}

func TestSetMetricSample(t *testing.T) {
	s := newSetMetric("test", "value1", []string{"tag1", "tag2"}, 1)
	s.sample("value2")
	assert.Equal(t, s.data, map[string]struct{}{"value1": struct{}{}, "value2": struct{}{}})
	assert.Equal(t, s.name, "test")
	assert.Equal(t, s.tags, []string{"tag1", "tag2"})
	assert.Equal(t, s.rate, 1.0)
}

func TestFlushUnsafeSetMetricSample(t *testing.T) {
	s := newSetMetric("test", "value1", []string{"tag1", "tag2"}, 1)
	m := s.flushUnsafe()

	require.Len(t, m, 1)

	assert.Equal(t, m[0].metricType, set)
	assert.Equal(t, m[0].svalue, "value1")
	assert.Equal(t, m[0].name, "test")
	assert.Equal(t, m[0].tags, []string{"tag1", "tag2"})
	assert.Equal(t, m[0].rate, 1.0)

	s.sample("value1")
	s.sample("value2")
	m = s.flushUnsafe()

	sort.Slice(m, func(i, j int) bool {
		return strings.Compare(m[i].svalue, m[j].svalue) != 1
	})

	require.Len(t, m, 2)
	assert.Equal(t, m[0].metricType, set)
	assert.Equal(t, m[0].svalue, "value1")
	assert.Equal(t, m[0].name, "test")
	assert.Equal(t, m[0].tags, []string{"tag1", "tag2"})
	assert.Equal(t, m[0].rate, 1.0)
	assert.Equal(t, m[1].metricType, set)
	assert.Equal(t, m[1].svalue, "value2")
	assert.Equal(t, m[1].name, "test")
	assert.Equal(t, m[1].tags, []string{"tag1", "tag2"})
	assert.Equal(t, m[1].rate, 1.0)
}

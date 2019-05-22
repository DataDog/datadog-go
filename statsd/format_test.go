package statsd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppendGauge(t *testing.T) {
	var buffer []byte
	buffer = appendGauge(buffer, "namespace.", []string{"global:tag"}, "gauge", 1., []string{"tag:tag"}, 1)
	assert.Equal(t, `namespace.gauge:1.000000|g|#global:tag,tag:tag`, string(buffer))
}

func TestAppendCount(t *testing.T) {
	var buffer []byte
	buffer = appendCount(buffer, "namespace.", []string{"global:tag"}, "count", 2, []string{"tag:tag"}, 1)
	assert.Equal(t, `namespace.count:2|c|#global:tag,tag:tag`, string(buffer))
}

func TestAppendHistogram(t *testing.T) {
	var buffer []byte
	buffer = appendHistogram(buffer, "namespace.", []string{"global:tag"}, "histogram", 3., []string{"tag:tag"}, 1)
	assert.Equal(t, `namespace.histogram:3.000000|h|#global:tag,tag:tag`, string(buffer))
}

func TestAppendDistribution(t *testing.T) {
	var buffer []byte
	buffer = appendDistribution(buffer, "namespace.", []string{"global:tag"}, "distribution", 4., []string{"tag:tag"}, 1)
	assert.Equal(t, `namespace.distribution:4.000000|d|#global:tag,tag:tag`, string(buffer))
}

func TestAppendDecrement(t *testing.T) {
	var buffer []byte
	buffer = appendDecrement(buffer, "namespace.", []string{"global:tag"}, "decrement", []string{"tag:tag"}, 1)
	assert.Equal(t, `namespace.decrement:-1|c|#global:tag,tag:tag`, string(buffer))
}

func TestAppendIncrement(t *testing.T) {
	var buffer []byte
	buffer = appendIncrement(buffer, "namespace.", []string{"global:tag"}, "increment", []string{"tag:tag"}, 1)
	assert.Equal(t, `namespace.increment:1|c|#global:tag,tag:tag`, string(buffer))
}

func TestAppendSet(t *testing.T) {
	var buffer []byte
	buffer = appendSet(buffer, "namespace.", []string{"global:tag"}, "set", "five", []string{"tag:tag"}, 1)
	assert.Equal(t, `namespace.set:five|s|#global:tag,tag:tag`, string(buffer))
}

func TestAppendTiming(t *testing.T) {
	var buffer []byte
	buffer = appendTiming(buffer, "namespace.", []string{"global:tag"}, "timing", 6., []string{"tag:tag"}, 1)
	assert.Equal(t, `namespace.timing:6.000000|ms|#global:tag,tag:tag`, string(buffer))
}

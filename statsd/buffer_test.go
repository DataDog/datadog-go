package statsd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBufferGauge(t *testing.T) {
	buffer := newStatsdBuffer(1024, 1)
	err := buffer.writeGauge("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1)
	assert.Nil(t, err)
	assert.Equal(t, `namespace.metric:1|g|#tag:tag`, string(buffer.bytes()))
}

func TestBufferCount(t *testing.T) {
	buffer := newStatsdBuffer(1024, 1)
	err := buffer.writeCount("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1)
	assert.Nil(t, err)
	assert.Equal(t, `namespace.metric:1|c|#tag:tag`, string(buffer.bytes()))
}

func TestBufferHistogram(t *testing.T) {
	buffer := newStatsdBuffer(1024, 1)
	err := buffer.writeHistogram("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1)
	assert.Nil(t, err)
	assert.Equal(t, `namespace.metric:1|h|#tag:tag`, string(buffer.bytes()))
}

func TestBufferDistribution(t *testing.T) {
	buffer := newStatsdBuffer(1024, 1)
	err := buffer.writeDistribution("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1)
	assert.Nil(t, err)
	assert.Equal(t, `namespace.metric:1|d|#tag:tag`, string(buffer.bytes()))
}
func TestBufferSet(t *testing.T) {
	buffer := newStatsdBuffer(1024, 1)
	err := buffer.writeSet("namespace.", []string{"tag:tag"}, "metric", "value", []string{}, 1)
	assert.Nil(t, err)
	assert.Equal(t, `namespace.metric:value|s|#tag:tag`, string(buffer.bytes()))
}

func TestBufferTiming(t *testing.T) {
	buffer := newStatsdBuffer(1024, 1)
	err := buffer.writeTiming("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1)
	assert.Nil(t, err)
	assert.Equal(t, `namespace.metric:1.000000|ms|#tag:tag`, string(buffer.bytes()))
}

func TestBufferEvent(t *testing.T) {
	buffer := newStatsdBuffer(1024, 1)
	err := buffer.writeEvent(Event{Title: "title", Text: "text"}, []string{"tag:tag"})
	assert.Nil(t, err)
	assert.Equal(t, `_e{5,4}:title|text|#tag:tag`, string(buffer.bytes()))
}

func TestBufferServiceCheck(t *testing.T) {
	buffer := newStatsdBuffer(1024, 1)
	err := buffer.writeServiceCheck(ServiceCheck{Name: "name", Status: Ok}, []string{"tag:tag"})
	assert.Nil(t, err)
	assert.Equal(t, `_sc|name|0|#tag:tag`, string(buffer.bytes()))
}

func TestBufferFullItems(t *testing.T) {
	buffer := newStatsdBuffer(1024, 1)
	err := buffer.writeGauge("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1)
	assert.Nil(t, err)
	err = buffer.writeGauge("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1)
	assert.Equal(t, errBufferFull, err)
}

func TestBufferFullSize(t *testing.T) {
	buffer := newStatsdBuffer(29, 10)
	err := buffer.writeGauge("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1)
	assert.Nil(t, err)
	assert.Len(t, buffer.bytes(), 29)
	err = buffer.writeGauge("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1)
	assert.Equal(t, errBufferFull, err)
}

func TestBufferSeparator(t *testing.T) {
	buffer := newStatsdBuffer(1024, 10)
	err := buffer.writeGauge("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1)
	assert.Nil(t, err)
	err = buffer.writeGauge("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1)
	assert.Equal(t, "namespace.metric:1|g|#tag:tag\nnamespace.metric:1|g|#tag:tag", string(buffer.bytes()))
}

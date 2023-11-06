package statsd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBufferGauge(t *testing.T) {
	buffer := newStatsdBuffer(1024, 1)
	err := buffer.writeGauge("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1, noTimestamp)
	assert.Nil(t, err)
	assert.Equal(t, "namespace.metric:1|g|#tag:tag\n", string(buffer.bytes()))

	// with a container ID field
	patchContainerID("container-id")
	defer resetContainerID()

	buffer = newStatsdBuffer(1024, 1)
	err = buffer.writeGauge("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1, noTimestamp)
	assert.Nil(t, err)
	assert.Equal(t, "namespace.metric:1|g|#tag:tag|c:container-id\n", string(buffer.bytes()))

	// with a timestamp
	buffer = newStatsdBuffer(1024, 1)
	err = buffer.writeGauge("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1, 1658934092)
	assert.Nil(t, err)
	assert.Equal(t, "namespace.metric:1|g|#tag:tag|c:container-id|T1658934092\n", string(buffer.bytes()))

}

func TestBufferCount(t *testing.T) {
	buffer := newStatsdBuffer(1024, 1)
	err := buffer.writeCount("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1, noTimestamp)
	assert.Nil(t, err)
	assert.Equal(t, "namespace.metric:1|c|#tag:tag\n", string(buffer.bytes()))

	// with a container ID field
	patchContainerID("container-id")
	defer resetContainerID()

	buffer = newStatsdBuffer(1024, 1)
	err = buffer.writeCount("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1, noTimestamp)
	assert.Nil(t, err)
	assert.Equal(t, "namespace.metric:1|c|#tag:tag|c:container-id\n", string(buffer.bytes()))

	// with a timestamp
	buffer = newStatsdBuffer(1024, 1)
	err = buffer.writeCount("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1, 1658934092)
	assert.Nil(t, err)
	assert.Equal(t, "namespace.metric:1|c|#tag:tag|c:container-id|T1658934092\n", string(buffer.bytes()))
}

func TestBufferHistogram(t *testing.T) {
	buffer := newStatsdBuffer(1024, 1)
	err := buffer.writeHistogram("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1)
	assert.Nil(t, err)
	assert.Equal(t, "namespace.metric:1|h|#tag:tag\n", string(buffer.bytes()))

	// with a container ID field
	patchContainerID("container-id")
	defer resetContainerID()

	buffer = newStatsdBuffer(1024, 1)
	err = buffer.writeHistogram("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1)
	assert.Nil(t, err)
	assert.Equal(t, "namespace.metric:1|h|#tag:tag|c:container-id\n", string(buffer.bytes()))
}

func TestBufferDistribution(t *testing.T) {
	buffer := newStatsdBuffer(1024, 1)
	err := buffer.writeDistribution("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1)
	assert.Nil(t, err)
	assert.Equal(t, "namespace.metric:1|d|#tag:tag\n", string(buffer.bytes()))

	// with a container ID field
	patchContainerID("container-id")
	defer resetContainerID()

	buffer = newStatsdBuffer(1024, 1)
	err = buffer.writeDistribution("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1)
	assert.Nil(t, err)
	assert.Equal(t, "namespace.metric:1|d|#tag:tag|c:container-id\n", string(buffer.bytes()))
}
func TestBufferSet(t *testing.T) {
	buffer := newStatsdBuffer(1024, 1)
	err := buffer.writeSet("namespace.", []string{"tag:tag"}, "metric", "value", []string{}, 1)
	assert.Nil(t, err)
	assert.Equal(t, "namespace.metric:value|s|#tag:tag\n", string(buffer.bytes()))

	// with a container ID field
	patchContainerID("container-id")
	defer resetContainerID()

	buffer = newStatsdBuffer(1024, 1)
	err = buffer.writeSet("namespace.", []string{"tag:tag"}, "metric", "value", []string{}, 1)
	assert.Nil(t, err)
	assert.Equal(t, "namespace.metric:value|s|#tag:tag|c:container-id\n", string(buffer.bytes()))
}

func TestBufferTiming(t *testing.T) {
	buffer := newStatsdBuffer(1024, 1)
	err := buffer.writeTiming("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1)
	assert.Nil(t, err)
	assert.Equal(t, "namespace.metric:1.000000|ms|#tag:tag\n", string(buffer.bytes()))

	// with a container ID field
	patchContainerID("container-id")
	defer resetContainerID()

	buffer = newStatsdBuffer(1024, 1)
	err = buffer.writeTiming("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1)
	assert.Nil(t, err)
	assert.Equal(t, "namespace.metric:1.000000|ms|#tag:tag|c:container-id\n", string(buffer.bytes()))
}

func TestBufferEvent(t *testing.T) {
	buffer := newStatsdBuffer(1024, 1)
	err := buffer.writeEvent(&Event{Title: "title", Text: "text"}, []string{"tag:tag"})
	assert.Nil(t, err)
	assert.Equal(t, "_e{5,4}:title|text|#tag:tag\n", string(buffer.bytes()))

	// with a container ID field
	patchContainerID("container-id")
	defer resetContainerID()

	buffer = newStatsdBuffer(1024, 1)
	err = buffer.writeEvent(&Event{Title: "title", Text: "text"}, []string{"tag:tag"})
	assert.Nil(t, err)
	assert.Equal(t, "_e{5,4}:title|text|#tag:tag|c:container-id\n", string(buffer.bytes()))
}

func TestBufferServiceCheck(t *testing.T) {
	buffer := newStatsdBuffer(1024, 1)
	err := buffer.writeServiceCheck(&ServiceCheck{Name: "name", Status: Ok}, []string{"tag:tag"})
	assert.Nil(t, err)
	assert.Equal(t, "_sc|name|0|#tag:tag\n", string(buffer.bytes()))

	// with a container ID field
	patchContainerID("container-id")
	defer resetContainerID()

	buffer = newStatsdBuffer(1024, 1)
	err = buffer.writeServiceCheck(&ServiceCheck{Name: "name", Status: Ok}, []string{"tag:tag"})
	assert.Nil(t, err)
	assert.Equal(t, "_sc|name|0|#tag:tag|c:container-id\n", string(buffer.bytes()))
}

func TestBufferFullSize(t *testing.T) {
	buffer := newStatsdBuffer(30, 10)
	err := buffer.writeGauge("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1, noTimestamp)
	assert.Nil(t, err)
	assert.Len(t, buffer.bytes(), 30)
	err = buffer.writeGauge("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1, noTimestamp)
	assert.Equal(t, errBufferFull, err)
}

func TestBufferSeparator(t *testing.T) {
	buffer := newStatsdBuffer(1024, 10)
	err := buffer.writeGauge("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1, noTimestamp)
	assert.Nil(t, err)
	err = buffer.writeGauge("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1, noTimestamp)
	assert.Nil(t, err)
	assert.Equal(t, "namespace.metric:1|g|#tag:tag\nnamespace.metric:1|g|#tag:tag\n", string(buffer.bytes()))
}

func TestBufferAggregated(t *testing.T) {
	buffer := newStatsdBuffer(1024, 1)
	pos, err := buffer.writeAggregated([]byte("h"), "namespace.", []string{"tag:tag"}, "metric", []float64{1}, "", 12, -1, 1)
	assert.Nil(t, err)
	assert.Equal(t, 1, pos)
	assert.Equal(t, "namespace.metric:1|h|#tag:tag\n", string(buffer.bytes()))

	buffer = newStatsdBuffer(1024, 1)
	pos, err = buffer.writeAggregated([]byte("h"), "namespace.", []string{"tag:tag"}, "metric", []float64{1, 2, 3, 4}, "", 12, -1, 1)
	assert.Nil(t, err)
	assert.Equal(t, 4, pos)
	assert.Equal(t, "namespace.metric:1:2:3:4|h|#tag:tag\n", string(buffer.bytes()))

	// With a sampling rate
	buffer = newStatsdBuffer(1024, 1)
	pos, err = buffer.writeAggregated([]byte("h"), "namespace.", []string{"tag:tag"}, "metric", []float64{1, 2, 3, 4}, "", 12, -1, 0.33)
	assert.Nil(t, err)
	assert.Equal(t, 4, pos)
	assert.Equal(t, "namespace.metric:1:2:3:4|h|@0.33|#tag:tag\n", string(buffer.bytes()))

	// max element already used
	buffer = newStatsdBuffer(1024, 1)
	buffer.elementCount = 1
	pos, err = buffer.writeAggregated([]byte("h"), "namespace.", []string{"tag:tag"}, "metric", []float64{1, 2, 3, 4}, "", 12, -1, 1)
	assert.Equal(t, errBufferFull, err)

	// not enough size to start serializing (tags and header too big)
	buffer = newStatsdBuffer(4, 1)
	pos, err = buffer.writeAggregated([]byte("h"), "namespace.", []string{"tag:tag"}, "metric", []float64{1, 2, 3, 4}, "", 12, -1, 1)
	assert.Equal(t, errBufferFull, err)

	// not enough size to serializing one message
	buffer = newStatsdBuffer(29, 1)
	pos, err = buffer.writeAggregated([]byte("h"), "namespace.", []string{"tag:tag"}, "metric", []float64{1, 2, 3, 4}, "", 12, -1, 1)
	assert.Equal(t, errBufferFull, err)

	// space for only 1 number
	buffer = newStatsdBuffer(30, 1)
	pos, err = buffer.writeAggregated([]byte("h"), "namespace.", []string{"tag:tag"}, "metric", []float64{1, 2, 3, 4}, "", 12, -1, 1)
	assert.Equal(t, errPartialWrite, err)
	assert.Equal(t, 1, pos)
	assert.Equal(t, "namespace.metric:1|h|#tag:tag\n", string(buffer.bytes()))

	// first value too big
	buffer = newStatsdBuffer(30, 1)
	pos, err = buffer.writeAggregated([]byte("h"), "namespace.", []string{"tag:tag"}, "metric", []float64{12, 2, 3, 4}, "", 12, -1, 1)
	assert.Equal(t, errBufferFull, err)
	assert.Equal(t, 0, pos)
	assert.Equal(t, "", string(buffer.bytes())) // checking that the buffer was reset

	// not enough space left
	buffer = newStatsdBuffer(40, 1)
	buffer.buffer = append(buffer.buffer, []byte("abcdefghij")...)
	pos, err = buffer.writeAggregated([]byte("h"), "namespace.", []string{"tag:tag"}, "metric", []float64{12, 2, 3, 4}, "", 12, -1, 1)
	assert.Equal(t, errBufferFull, err)
	assert.Equal(t, 0, pos)
	assert.Equal(t, "abcdefghij", string(buffer.bytes())) // checking that the buffer was reset

	// space for only 2 number
	buffer = newStatsdBuffer(32, 1)
	pos, err = buffer.writeAggregated([]byte("h"), "namespace.", []string{"tag:tag"}, "metric", []float64{1, 2, 3, 4}, "", 12, -1, 1)
	assert.Equal(t, errPartialWrite, err)
	assert.Equal(t, 2, pos)
	assert.Equal(t, "namespace.metric:1:2|h|#tag:tag\n", string(buffer.bytes()))

	// with a container ID field
	patchContainerID("container-id")
	defer resetContainerID()

	buffer = newStatsdBuffer(1024, 1)
	pos, err = buffer.writeAggregated([]byte("h"), "namespace.", []string{"tag:tag"}, "metric", []float64{1}, "", 12, -1, 1)
	assert.Nil(t, err)
	assert.Equal(t, 1, pos)
	assert.Equal(t, "namespace.metric:1|h|#tag:tag|c:container-id\n", string(buffer.bytes()))
}

func TestBufferMaxElement(t *testing.T) {
	buffer := newStatsdBuffer(1024, 1)

	err := buffer.writeGauge("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1, noTimestamp)
	assert.Nil(t, err)

	err = buffer.writeGauge("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1, noTimestamp)
	assert.Equal(t, errBufferFull, err)

	err = buffer.writeCount("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1, noTimestamp)
	assert.Equal(t, errBufferFull, err)

	err = buffer.writeHistogram("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1)
	assert.Equal(t, errBufferFull, err)

	err = buffer.writeDistribution("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1)
	assert.Equal(t, errBufferFull, err)

	err = buffer.writeSet("namespace.", []string{"tag:tag"}, "metric", "value", []string{}, 1)
	assert.Equal(t, errBufferFull, err)

	err = buffer.writeTiming("namespace.", []string{"tag:tag"}, "metric", 1, []string{}, 1)
	assert.Equal(t, errBufferFull, err)

	err = buffer.writeEvent(&Event{Title: "title", Text: "text"}, []string{"tag:tag"})
	assert.Equal(t, errBufferFull, err)

	err = buffer.writeServiceCheck(&ServiceCheck{Name: "name", Status: Ok}, []string{"tag:tag"})
	assert.Equal(t, errBufferFull, err)
}

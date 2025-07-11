package statsd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultOptions(t *testing.T) {
	options, err := resolveOptions([]Option{})

	assert.NoError(t, err)
	assert.Equal(t, options.namespace, defaultNamespace)
	assert.Equal(t, options.tags, defaultTags)
	assert.Equal(t, options.maxBytesPerPayload, defaultMaxBytesPerPayload)
	assert.Equal(t, options.maxMessagesPerPayload, defaultMaxMessagesPerPayload)
	assert.Equal(t, options.bufferPoolSize, defaultBufferPoolSize)
	assert.Equal(t, options.bufferFlushInterval, defaultBufferFlushInterval)
	assert.Equal(t, options.workersCount, defaultWorkerCount)
	assert.Equal(t, options.senderQueueSize, defaultSenderQueueSize)
	assert.Equal(t, options.writeTimeout, defaultWriteTimeout)
	assert.Equal(t, options.telemetry, defaultTelemetry)
	assert.Equal(t, options.receiveMode, defaultReceivingMode)
	assert.Equal(t, options.channelModeBufferSize, defaultChannelModeBufferSize)
	assert.Equal(t, options.aggregationFlushInterval, defaultAggregationFlushInterval)
	assert.Equal(t, options.aggregation, defaultAggregation)
	assert.Equal(t, options.extendedAggregation, defaultExtendedAggregation)
	assert.Zero(t, options.telemetryAddr)
	assert.Equal(t, options.tagCardinality, defaultTagCardinality)
}

func TestOptions(t *testing.T) {
	testNamespace := "datadog."
	testTags := []string{"rocks"}
	testMaxBytesPerPayload := 2048
	testMaxMessagePerPayload := 1024
	testBufferPoolSize := 32
	testBufferFlushInterval := 48 * time.Second
	testBufferShardCount := 28
	testSenderQueueSize := 64
	testWriteTimeout := 1 * time.Minute
	testChannelBufferSize := 500
	testAggregationWindow := 10 * time.Second
	testTelemetryAddr := "localhost:1234"
	testTagCardinality := "high"

	options, err := resolveOptions([]Option{
		WithNamespace(testNamespace),
		WithTags(testTags),
		WithMaxBytesPerPayload(testMaxBytesPerPayload),
		WithMaxMessagesPerPayload(testMaxMessagePerPayload),
		WithBufferPoolSize(testBufferPoolSize),
		WithBufferFlushInterval(testBufferFlushInterval),
		WithWorkersCount(testBufferShardCount),
		WithSenderQueueSize(testSenderQueueSize),
		WithWriteTimeout(testWriteTimeout),
		WithoutTelemetry(),
		WithChannelMode(),
		WithChannelModeBufferSize(testChannelBufferSize),
		WithAggregationInterval(testAggregationWindow),
		WithClientSideAggregation(),
		WithTelemetryAddr(testTelemetryAddr),
		WithCardinality(testTagCardinality),
	})

	assert.NoError(t, err)
	assert.Equal(t, options.namespace, testNamespace)
	assert.Equal(t, options.tags, testTags)
	assert.Equal(t, options.maxBytesPerPayload, testMaxBytesPerPayload)
	assert.Equal(t, options.maxMessagesPerPayload, testMaxMessagePerPayload)
	assert.Equal(t, options.bufferPoolSize, testBufferPoolSize)
	assert.Equal(t, options.bufferFlushInterval, testBufferFlushInterval)
	assert.Equal(t, options.workersCount, testBufferShardCount)
	assert.Equal(t, options.senderQueueSize, testSenderQueueSize)
	assert.Equal(t, options.writeTimeout, testWriteTimeout)
	assert.Equal(t, options.telemetry, false)
	assert.Equal(t, options.receiveMode, channelMode)
	assert.Equal(t, options.channelModeBufferSize, testChannelBufferSize)
	assert.Equal(t, options.aggregationFlushInterval, testAggregationWindow)
	assert.Equal(t, options.aggregation, true)
	assert.Equal(t, options.extendedAggregation, false)
	assert.Equal(t, options.telemetryAddr, testTelemetryAddr)
	assert.Equal(t, options.tagCardinality.card, testTagCardinality)
}

func TestExtendedAggregation(t *testing.T) {
	options, err := resolveOptions([]Option{
		WithoutClientSideAggregation(),
		WithExtendedClientSideAggregation(),
	})

	assert.NoError(t, err)
	assert.Equal(t, options.aggregation, true)
	assert.Equal(t, options.extendedAggregation, true)
}

func TestResetOptions(t *testing.T) {
	options, err := resolveOptions([]Option{
		WithChannelMode(),
		WithMutexMode(),
		WithoutClientSideAggregation(),
	})

	assert.NoError(t, err)
	assert.Equal(t, options.receiveMode, mutexMode)
	assert.Equal(t, options.aggregation, false)
	assert.Equal(t, options.extendedAggregation, false)
}
func TestOptionsNamespaceWithoutDot(t *testing.T) {
	testNamespace := "datadog"

	options, err := resolveOptions([]Option{
		WithNamespace(testNamespace),
	})

	assert.NoError(t, err)
	assert.Equal(t, options.namespace, testNamespace+".")
}

func TestOptionsInvalidTagCardinality(t *testing.T) {
	invalidTagCardinality := "invalid"
	options, err := resolveOptions([]Option{
		WithCardinality(invalidTagCardinality),
	})

	assert.NoError(t, err)
	assert.Equal(t, options.tagCardinality, defaultTagCardinality)
}

package statsd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultOptions(t *testing.T) {
	options, err := resolveOptions([]Option{})

	assert.NoError(t, err)
	assert.Equal(t, options.Namespace, DefaultNamespace)
	assert.Equal(t, options.Tags, DefaultTags)
	assert.Equal(t, options.Buffered, DefaultBuffered)
	assert.Equal(t, options.MaxBytePerPayload, DefaultMaxBytePerPayload)
	assert.Equal(t, options.MaxMessagesPerPayload, DefaultMaxMessagesPerPayload)
	assert.Equal(t, options.BufferPoolSize, DefaultBufferPoolSize)
	assert.Equal(t, options.BufferFlushInterval, DefaultBufferFlushInterval)
	assert.Equal(t, options.SenderQueueSize, DefaultSenderQueueSize)
	assert.Equal(t, options.AsyncUDS, DefaultAsyncUDS)
	assert.Equal(t, options.WriteTimeoutUDS, DefaultWriteTimeoutUDS)
}

func TestOptions(t *testing.T) {
	testNamespace := "datadog."
	testTags := []string{"rocks"}
	testBuffered := true
	testMaxBytePerPayload := 2048
	testMaxMessagePerPayload := 1024
	testBufferPoolSize := 32
	testBufferFlushInterval := 48 * time.Second
	testSenderQueueSize := 64
	testAsyncUDS := true
	testWriteTimeoutUDS := 1 * time.Minute

	options, err := resolveOptions([]Option{
		WithNamespace(testNamespace),
		WithTags(testTags),
		Buffered(),
		WithMaxBytePerPayload(testMaxBytePerPayload),
		WithMaxMessagesPerPayload(testMaxMessagePerPayload),
		WithBufferPoolSize(testBufferPoolSize),
		WithBufferFlushInterval(testBufferFlushInterval),
		WithSenderQueueSize(testSenderQueueSize),
		WithAsyncUDS(),
		WithWriteTimeoutUDS(testWriteTimeoutUDS),
	})

	assert.NoError(t, err)
	assert.Equal(t, options.Namespace, testNamespace)
	assert.Equal(t, options.Tags, testTags)
	assert.Equal(t, options.Buffered, testBuffered)
	assert.Equal(t, options.MaxBytePerPayload, testMaxBytePerPayload)
	assert.Equal(t, options.MaxMessagesPerPayload, testMaxMessagePerPayload)
	assert.Equal(t, options.BufferPoolSize, testBufferPoolSize)
	assert.Equal(t, options.BufferFlushInterval, testBufferFlushInterval)
	assert.Equal(t, options.SenderQueueSize, testSenderQueueSize)
	assert.Equal(t, options.AsyncUDS, testAsyncUDS)
	assert.Equal(t, options.WriteTimeoutUDS, testWriteTimeoutUDS)
}

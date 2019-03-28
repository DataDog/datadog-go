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
	assert.Equal(t, options.MaxMessagesPerPayload, DefaultMaxMessagesPerPayload)
	assert.Equal(t, options.AsyncUDS, DefaultAsyncUDS)
	assert.Equal(t, options.WriteTimeoutUDS, DefaultWriteTimeoutUDS)
}

func TestOptions(t *testing.T) {
	testNamespace := "datadog."
	testTags := []string{"rocks"}
	testBuffered := true
	testMaxMessagePerPayload := 1024
	testAsyncUDS := true
	testWriteTimeoutUDS := 1 * time.Minute

	options, err := resolveOptions([]Option{
		WithNamespace(testNamespace),
		WithTags(testTags),
		Buffered(),
		WithMaxMessagesPerPayload(testMaxMessagePerPayload),
		WithAsyncUDS(),
		WithWriteTimeoutUDS(testWriteTimeoutUDS),
	})

	assert.NoError(t, err)
	assert.Equal(t, options.Namespace, testNamespace)
	assert.Equal(t, options.Tags, testTags)
	assert.Equal(t, options.Buffered, testBuffered)
	assert.Equal(t, options.MaxMessagesPerPayload, testMaxMessagePerPayload)
	assert.Equal(t, options.AsyncUDS, testAsyncUDS)
	assert.Equal(t, options.WriteTimeoutUDS, testWriteTimeoutUDS)
}

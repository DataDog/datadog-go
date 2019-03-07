package statsd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultOptions(t *testing.T) {
	options := resolveOptions([]Option{})

	assert.Equal(t, options.Namespace, DefaultNamespace)
	assert.Equal(t, options.Tags, DefaultTags)
	assert.Equal(t, options.Buffered, DefaultBuffered)
	assert.Equal(t, options.MaxMessagesPerPayload, DefaultMaxMessagesPerPayload)
	assert.Equal(t, options.BlockingUDS, DefaultBlockingUDS)
	assert.Equal(t, options.WriteTimeoutUDS, DefaultWriteTimeoutUDS)
}

func TestOptions(t *testing.T) {
	testNamespace := "datadog."
	testTags := []string{"rocks"}
	testBuffered := true
	testMaxMessagePerPayload := 1024
	testBlockingUDS := true
	testWriteTimeoutUDS := 1 * time.Minute

	options := resolveOptions([]Option{
		Namespace(testNamespace),
		Tags(testTags),
		Buffered(testBuffered),
		MaxMessagesPerPayload(testMaxMessagePerPayload),
		BlockingUDS(testBlockingUDS),
		WriteTimeoutUDS(testWriteTimeoutUDS),
	})

	assert.Equal(t, options.Namespace, testNamespace)
	assert.Equal(t, options.Tags, testTags)
	assert.Equal(t, options.Buffered, testBuffered)
	assert.Equal(t, options.MaxMessagesPerPayload, testMaxMessagePerPayload)
	assert.Equal(t, options.BlockingUDS, testBlockingUDS)
	assert.Equal(t, options.WriteTimeoutUDS, testWriteTimeoutUDS)
}

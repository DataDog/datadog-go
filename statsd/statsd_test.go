package statsd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type statsdWriterWrapper struct{}

func (statsdWriterWrapper) SetWriteTimeout(time.Duration) error {
	return nil
}

func (statsdWriterWrapper) Close() error {
	return nil
}

func (statsdWriterWrapper) Write(p []byte) (n int, err error) {
	return 0, nil
}

func TestCustomWriterBufferConfiguration(t *testing.T) {
	client, err := NewWithWriter(statsdWriterWrapper{})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	assert.Equal(t, OptimalUDPPayloadSize, client.bufferPool.bufferMaxSize)
	assert.Equal(t, DefaultUDPBufferPoolSize, cap(client.bufferPool.pool))
	assert.Equal(t, DefaultUDPBufferPoolSize, cap(client.sender.queue))
}

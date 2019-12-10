package statsd

import (
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type statsdWriterWrapper struct {
	io.WriteCloser
}

func (statsdWriterWrapper) SetWriteTimeout(time.Duration) error {
	return nil
}

func TestCustomWriterBufferConfiguration(t *testing.T) {
	client, err := NewWithWriter(statsdWriterWrapper{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, OptimalUDPPayloadSize, client.bufferPool.bufferMaxSize)
	assert.Equal(t, DefaultUDPBufferPoolSize, cap(client.bufferPool.pool))
	assert.Equal(t, DefaultUDPBufferPoolSize, cap(client.sender.queue))
}

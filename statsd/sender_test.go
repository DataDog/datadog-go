package statsd

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockedWriter struct {
	mock.Mock
}

func (w *mockedWriter) Write(data []byte) (n int, err error) {
	args := w.Called(data)
	return args.Int(0), args.Error(1)
}

func (w *mockedWriter) Close() error {
	args := w.Called()
	return args.Error(0)
}

func TestSender(t *testing.T) {
	writer := new(mockedWriter)
	writer.On("Write", mock.Anything).Return(1, nil)
	writer.On("Close").Return(nil)
	pool := newBufferPool(10, 1024, 1)
	sender := newSender(writer, 10, pool, LoggingErrorHandler)
	buffer := pool.borrowBuffer()
	buffer.writeSeparator() // add some dummy data

	sender.send(buffer)

	err := sender.close()
	assert.Nil(t, err)
	writer.AssertCalled(t, "Write", []byte("\n"))
	assert.Equal(t, 10, len(pool.pool))

	assert.Equal(t, uint64(1), sender.telemetry.totalPayloadsSent)
	assert.Equal(t, uint64(0), sender.telemetry.totalPayloadsDroppedQueueFull)
	assert.Equal(t, uint64(0), sender.telemetry.totalPayloadsDroppedWriter)
	assert.Equal(t, uint64(1), sender.telemetry.totalBytesSent)
	assert.Equal(t, uint64(0), sender.telemetry.totalBytesDroppedQueueFull)
	assert.Equal(t, uint64(0), sender.telemetry.totalBytesDroppedWriter)

}

func TestSenderBufferFullTelemetry(t *testing.T) {
	writer := new(mockedWriter)
	writer.On("Write", mock.Anything).Return(0, nil)
	writer.On("Close").Return(nil)

	// a sender with a queue of 1 message
	pool := newBufferPool(10, 1024, 1)
	sender := newSender(writer, 0, pool, LoggingErrorHandler)

	// close the sender to prevent it from consuming the queue
	sender.close()

	// fill the queue to its max
	buffer := pool.borrowBuffer()
	buffer.writeSeparator() // add some dummy data
	sender.send(buffer)

	assert.Equal(t, uint64(0), sender.telemetry.totalPayloadsSent)
	assert.Equal(t, uint64(1), sender.telemetry.totalPayloadsDroppedQueueFull)
	assert.Equal(t, uint64(0), sender.telemetry.totalPayloadsDroppedWriter)

	assert.Equal(t, uint64(0), sender.telemetry.totalBytesSent)
	assert.Equal(t, uint64(1), sender.telemetry.totalBytesDroppedQueueFull)
	assert.Equal(t, uint64(0), sender.telemetry.totalBytesDroppedWriter)
}

func TestSenderWriteError(t *testing.T) {
	writer := new(mockedWriter)
	writer.On("Write", mock.Anything).Return(1, fmt.Errorf("some write error"))
	writer.On("Close").Return(nil)
	pool := newBufferPool(10, 1024, 1)
	sender := newSender(writer, 10, pool, LoggingErrorHandler)
	buffer := pool.borrowBuffer()
	buffer.writeSeparator() // add some dummy data

	sender.send(buffer)

	err := sender.close()
	assert.Nil(t, err)
	writer.AssertCalled(t, "Write", []byte("\n"))
	assert.Equal(t, 10, len(pool.pool))

	assert.Equal(t, uint64(0), sender.telemetry.totalPayloadsSent)
	assert.Equal(t, uint64(0), sender.telemetry.totalPayloadsDroppedQueueFull)
	assert.Equal(t, uint64(1), sender.telemetry.totalPayloadsDroppedWriter)

	assert.Equal(t, uint64(0), sender.telemetry.totalBytesSent)
	assert.Equal(t, uint64(0), sender.telemetry.totalBytesDroppedQueueFull)
	assert.Equal(t, uint64(1), sender.telemetry.totalBytesDroppedWriter)
}

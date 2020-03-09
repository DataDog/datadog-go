package statsd

import (
	"sync"
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

func TestCloseDeadlock(t *testing.T) {
	timeout := time.After(1 * time.Second)
	done := make(chan bool)
	go func() {
		c, err := New("localhost:8125")
		c.flushTime = time.Millisecond
		c.debugDeadlock = make(chan struct{})
		assert.NoError(t, err)

		var wg sync.WaitGroup

		// start the watcher and make sure the ticker has fired.
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.watch()
		}()
		time.Sleep(100 * time.Millisecond)

		// start the Close()
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Close()
		}()
		time.Sleep(100 * time.Millisecond)

		// now trigger the deadlock condition
		close(c.debugDeadlock)
		wg.Wait()

		done <- true
	}()

	select {
	case <-timeout:
		t.Fatal("Test didn't finish in time")
	case <-done:
	}
}

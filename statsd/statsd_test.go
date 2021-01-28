package statsd

import (
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.Nil(t, err)
	defer client.Close()

	assert.Equal(t, OptimalUDPPayloadSize, client.sender.pool.bufferMaxSize)
	assert.Equal(t, DefaultUDPBufferPoolSize, cap(client.sender.pool.pool))
	assert.Equal(t, DefaultUDPBufferPoolSize, cap(client.sender.queue))
}

// TestConcurrentSend sends various metric types in separate goroutines to
// trigger any possible data races. It is intended to be run with the data race
// detector enabled.
func TestConcurrentSend(t *testing.T) {
	tests := []struct {
		description   string
		clientOptions []Option
	}{
		{
			description:   "Client with default options",
			clientOptions: []Option{},
		},
		{
			description:   "Client with mutex mode enabled",
			clientOptions: []Option{WithMutexMode()},
		},
		{
			description:   "Client with channel mode enabled",
			clientOptions: []Option{WithChannelMode()},
		},
	}

	for _, test := range tests {
		test := test // Capture range variable.
		t.Run(test.description, func(t *testing.T) {
			t.Parallel()

			client, err := New("localhost:9876", test.clientOptions...)
			require.Nil(t, err, fmt.Sprintf("failed to create client: %s", err))

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				client.Gauge("name", 1, []string{"tag"}, 0.1)
				wg.Done()
			}()

			wg.Add(1)
			go func() {
				client.Count("name", 1, []string{"tag"}, 0.1)
				wg.Done()
			}()

			wg.Add(1)
			go func() {
				client.Timing("name", 1, []string{"tag"}, 0.1)
				wg.Done()
			}()

			wg.Wait()
			err = client.Close()
			require.Nil(t, err, fmt.Sprintf("failed to close client: %s", err))
		})
	}
}

func getTestServer(t *testing.T, addr string) *net.UDPConn {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	require.Nil(t, err, fmt.Sprintf("could not resolve udp '%s': %s", addr, err))

	server, err := net.ListenUDP("udp", udpAddr)
	require.Nil(t, err, fmt.Sprintf("Could not listen to UDP addr: %s", err))
	return server
}

func testStatsdPipeline(t *testing.T, client *Client, addr string) {
	server := getTestServer(t, addr)
	defer server.Close()

	client.Count("name", 1, []string{"tag"}, 1)

	err := client.Close()
	require.Nil(t, err, fmt.Sprintf("failed to close client: %s", err))

	readDone := make(chan struct{})
	buffer := make([]byte, 4096)
	n := 0
	go func() {
		n, _ = io.ReadAtLeast(server, buffer, 1)
		close(readDone)
	}()

	select {
	case <-readDone:
	case <-time.After(2 * time.Second):
		require.Fail(t, "No data was flush on Close")
	}

	result := string(buffer[:n])
	assert.Equal(t, "name:1|c|#tag", result)
}

func TestChannelMode(t *testing.T) {
	addr := "localhost:1201"

	client, err := New(addr, WithChannelMode())
	require.Nil(t, err, fmt.Sprintf("failed to create client: %s", err))
	assert.False(t, client.telemetry.devMode)

	testStatsdPipeline(t, client, addr)
}

func TestMutexMode(t *testing.T) {
	addr := "localhost:1201"

	client, err := New(addr)
	require.Nil(t, err, fmt.Sprintf("failed to create client: %s", err))
	assert.False(t, client.telemetry.devMode)

	testStatsdPipeline(t, client, addr)
}

func TestDevMode(t *testing.T) {
	addr := "localhost:1201"

	client, err := New(addr, WithDevMode())
	require.Nil(t, err, fmt.Sprintf("failed to create client: %s", err))
	assert.True(t, client.telemetry.devMode)

	testStatsdPipeline(t, client, addr)
}

func TestCloneWithExtraOptions(t *testing.T) {
	addr := "localhost:1201"

	client, err := New(addr, WithTags([]string{"tag1", "tag2"}))
	require.Nil(t, err, fmt.Sprintf("failed to create client: %s", err))

	assert.Equal(t, client.Tags, []string{"tag1", "tag2"})
	assert.Equal(t, client.Namespace, "")
	assert.Equal(t, client.receiveMode, MutexMode)
	assert.Equal(t, client.addrOption, addr)
	assert.Len(t, client.options, 1)

	cloneClient, err := CloneWithExtraOptions(client, WithNamespace("test"), WithChannelMode())
	require.Nil(t, err, fmt.Sprintf("failed to clone client: %s", err))

	assert.Equal(t, cloneClient.Tags, []string{"tag1", "tag2"})
	assert.Equal(t, cloneClient.Namespace, "test.")
	assert.Equal(t, cloneClient.receiveMode, ChannelMode)
	assert.Equal(t, cloneClient.addrOption, addr)
	assert.Len(t, cloneClient.options, 3)
}

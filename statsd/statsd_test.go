package statsd

import (
	"fmt"
	"io"
	"net"
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

func getTestServer(t *testing.T, addr string) *net.UDPConn {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	require.Nil(t, err, fmt.Sprintf("could not resolve udp '%s': %s", addr, err))

	server, err := net.ListenUDP("udp", udpAddr)
	require.Nil(t, err, fmt.Sprintf("Could not listen to UDP addr: %s", err))
	return server
}

func TestFlushOnClose(t *testing.T) {
	buffer := make([]byte, 4096)
	addr := "localhost:1201"
	server := getTestServer(t, addr)
	defer server.Close()

	client, err := New(addr)
	require.Nil(t, err, fmt.Sprintf("failed to create client: %s", err))

	client.Count("name", 1, []string{"tag"}, 1)

	err = client.Close()
	require.Nil(t, err, fmt.Sprintf("failed to close client: %s", err))

	readDone := make(chan struct{})
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

// +build !windows

package statsd

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func TestNewUDSWriter(t *testing.T) {
	w, err := newUDSWriter("/tmp/test.socket", 100*time.Millisecond)
	assert.NotNil(t, w)
	assert.NoError(t, err)
}

func TestUDSWrite(t *testing.T) {
	socketPath := fmt.Sprintf("/tmp/dsd_%d.socket", rand.Int())
	defer os.Remove(socketPath)

	address, err := net.ResolveUnixAddr("unixgram", socketPath)
	require.NoError(t, err)
	conn, err := net.ListenUnixgram("unixgram", address)
	require.NoError(t, err)
	err = os.Chmod(socketPath, 0722)
	require.NoError(t, err)

	w, err := newUDSWriter(socketPath, 100*time.Millisecond)
	require.Nil(t, err)
	require.NotNil(t, w)

	// test 2 Write: the first one should setup the connection
	for i := 0; i < 2; i++ {
		n, err := w.Write([]byte("some data"))
		require.NoError(t, err)
		assert.Equal(t, 9, n)

		buffer := make([]byte, 100)
		n, err = conn.Read(buffer)
		require.NoError(t, err)
		assert.Equal(t, "some data", string(buffer[:n]))
	}
}

func TestUDSWriteUnsetConnection(t *testing.T) {
	socketPath := fmt.Sprintf("/tmp/dsd_%d.socket", rand.Int())
	defer os.Remove(socketPath)

	address, err := net.ResolveUnixAddr("unixgram", socketPath)
	require.NoError(t, err)
	conn, err := net.ListenUnixgram("unixgram", address)
	require.NoError(t, err)
	err = os.Chmod(socketPath, 0722)
	require.NoError(t, err)

	w, err := newUDSWriter(socketPath, 100*time.Millisecond)
	require.Nil(t, err)
	require.NotNil(t, w)

	// test 2 Write: the first one should setup the connection
	for i := 0; i < 2; i++ {
		n, err := w.Write([]byte("some data"))
		require.NoError(t, err)
		assert.Equal(t, 9, n)

		buffer := make([]byte, 100)
		n, err = conn.Read(buffer)
		require.NoError(t, err)
		assert.Equal(t, "some data", string(buffer[:n]))

		// Unset connection for the next Read
		w.unsetConnection()
	}
}

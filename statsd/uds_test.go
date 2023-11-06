//go:build !windows
// +build !windows

package statsd

import (
	"encoding/binary"
	"golang.org/x/net/nettest"
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
	w, err := newUDSWriter("/tmp/test.socket", 100*time.Millisecond, "")
	assert.NotNil(t, w)
	assert.NoError(t, err)
	w, err = newUDSWriter("/tmp/test.socket", 100*time.Millisecond, "unix")
	assert.NotNil(t, w)
	assert.NoError(t, err)
	w, err = newUDSWriter("/tmp/test.socket", 100*time.Millisecond, "unixgram")
	assert.NotNil(t, w)
	assert.NoError(t, err)
}

func TestUDSDatagramWrite(t *testing.T) {
	socketPath, err := nettest.LocalPath()
	require.NoError(t, err)
	defer os.Remove(socketPath)

	address, err := net.ResolveUnixAddr("unixgram", socketPath)
	require.NoError(t, err)
	conn, err := net.ListenUnixgram("unixgram", address)
	require.NoError(t, err)
	err = os.Chmod(socketPath, 0722)
	require.NoError(t, err)

	w, err := newUDSWriter(socketPath, 100*time.Millisecond, "")
	require.Nil(t, err)
	require.NotNil(t, w)

	// test 2 Write: the first one should setup the connection
	for i := 0; i < 2; i++ {
		msg := []byte("some data")
		n, err := w.Write(msg)
		require.NoError(t, err)
		assert.Equal(t, len(msg), n)

		buffer := make([]byte, 100)
		n, err = conn.Read(buffer)
		require.NoError(t, err)
		assert.Equal(t, "some data", string(buffer[:n]))
	}
}

func TestUDSDatagramWriteUnsetConnection(t *testing.T) {
	socketPath, err := nettest.LocalPath()
	require.NoError(t, err)
	defer os.Remove(socketPath)

	address, err := net.ResolveUnixAddr("unixgram", socketPath)
	require.NoError(t, err)
	conn, err := net.ListenUnixgram("unixgram", address)
	require.NoError(t, err)
	err = os.Chmod(socketPath, 0722)
	require.NoError(t, err)

	w, err := newUDSWriter(socketPath, 100*time.Millisecond, "")
	require.Nil(t, err)
	require.NotNil(t, w)

	// test 2 Write: the first one should setup the connection
	for i := 0; i < 2; i++ {
		msg := []byte("some data")
		n, err := w.Write(msg)
		require.NoError(t, err)
		assert.Equal(t, len(msg), n)

		buffer := make([]byte, 100)
		n, err = conn.Read(buffer)
		require.NoError(t, err)
		assert.Equal(t, "some data", string(buffer[:n]))

		// Unset connection for the next Read
		w.unsetConnection()
	}
}

func TestUDSStreamWrite(t *testing.T) {
	socketPath, err := nettest.LocalPath()
	require.NoError(t, err)
	defer os.Remove(socketPath)

	address, err := net.ResolveUnixAddr("unix", socketPath)
	require.NoError(t, err)
	listener, err := net.ListenUnix("unix", address)
	require.NoError(t, err)
	err = os.Chmod(socketPath, 0722)
	require.NoError(t, err)

	w, err := newUDSWriter(socketPath, 100*time.Millisecond, "")
	require.Nil(t, err)
	require.NotNil(t, w)

	var conn net.Conn

	// test 2 Write: the first one should setup the connection
	for i := 0; i < 2; i++ {
		msg := []byte("some data")
		n, err := w.Write(msg)
		require.NoError(t, err)
		assert.Equal(t, len(msg), n)

		if conn == nil {
			conn, err = listener.Accept()
			require.NoError(t, err)
		}

		var l uint32
		binary.Read(conn, binary.LittleEndian, &l)
		assert.Equal(t, uint32(len(msg)), l)

		buffer := make([]byte, 100)
		n, err = conn.Read(buffer)
		require.NoError(t, err)
		assert.Equal(t, "some data", string(buffer[:n]))
	}
}

func TestUDSStreamWriteUnsetConnection(t *testing.T) {
	socketPath, err := nettest.LocalPath()
	require.NoError(t, err)
	defer os.Remove(socketPath)

	address, err := net.ResolveUnixAddr("unix", socketPath)
	require.NoError(t, err)
	listener, err := net.ListenUnix("unix", address)
	require.NoError(t, err)
	err = os.Chmod(socketPath, 0722)
	require.NoError(t, err)

	w, err := newUDSWriter(socketPath, 100*time.Millisecond, "")
	require.Nil(t, err)
	require.NotNil(t, w)

	var conn net.Conn

	// test 2 Write: the first one should setup the connection
	for i := 0; i < 2; i++ {
		msg := []byte("some data")
		n, err := w.Write(msg)
		require.NoError(t, err)
		assert.Equal(t, len(msg), n)

		if conn == nil {
			conn, err = listener.Accept()
			require.NoError(t, err)
		}

		var l uint32
		binary.Read(conn, binary.LittleEndian, &l)
		assert.Equal(t, uint32(len(msg)), l)

		buffer := make([]byte, 100)
		n, err = conn.Read(buffer)
		require.NoError(t, err)
		assert.Equal(t, "some data", string(buffer[:n]))

		// Unset connection for the next Read
		w.unsetConnection()
		conn = nil
	}
}

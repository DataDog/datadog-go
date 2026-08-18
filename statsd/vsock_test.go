//go:build linux
// +build linux

package statsd

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewVsockWriter(t *testing.T) {
	w, err := newVsockWriter("vsock://host:8125", 100*time.Millisecond, 1000*time.Millisecond)
	require.NoError(t, err)
	require.NotNil(t, w)
	assert.Equal(t, writerNameVsock, w.GetTransportName())

	// The address is validated upfront, even though the connection itself is deferred to the first
	// write.
	w, err = newVsockWriter("vsock://not-a-cid:8125", 100*time.Millisecond, 1000*time.Millisecond)
	assert.Error(t, err)
	assert.Nil(t, w)
}

// newVsockTestListener returns a listening vsock socket and the port it is bound to. The test is
// skipped when the machine can't talk to itself over vsock: AF_VSOCK needs the vsock module, and
// VMADDR_CID_LOCAL needs the vsock_loopback one (Linux 5.6+), neither of which is a given on a CI
// runner or inside a container.
func newVsockTestListener(t *testing.T) (int, uint32) {
	fd, err := unix.Socket(unix.AF_VSOCK, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Skipf("vsock is not available on this machine: %v", err)
	}

	// Bound so that a missing connection never hangs the test suite.
	if err := setSocketTimeout(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, 10*time.Second); err != nil {
		unix.Close(fd)
		require.NoError(t, err)
	}

	if err := unix.Bind(fd, &unix.SockaddrVM{CID: unix.VMADDR_CID_ANY, Port: unix.VMADDR_PORT_ANY}); err != nil {
		unix.Close(fd)
		t.Skipf("could not bind a vsock socket on this machine: %v", err)
	}
	if err := unix.Listen(fd, 2); err != nil {
		unix.Close(fd)
		t.Skipf("could not listen on a vsock socket on this machine: %v", err)
	}

	sa, err := unix.Getsockname(fd)
	if err != nil {
		unix.Close(fd)
		require.NoError(t, err)
	}
	port := sa.(*unix.SockaddrVM).Port

	// Check that loopback connections actually work before handing the listener over.
	probe, err := unix.Socket(unix.AF_VSOCK, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		unix.Close(fd)
		t.Skipf("vsock is not available on this machine: %v", err)
	}
	// Bound so that a machine on which loopback connections hang, rather than fail, doesn't hang the
	// test suite with them.
	if err := setSocketTimeout(probe, unix.SOL_SOCKET, unix.SO_SNDTIMEO, 10*time.Second); err != nil {
		unix.Close(probe)
		unix.Close(fd)
		require.NoError(t, err)
	}
	err = unix.Connect(probe, &unix.SockaddrVM{CID: unix.VMADDR_CID_LOCAL, Port: port})
	unix.Close(probe)
	if err != nil {
		unix.Close(fd)
		t.Skipf("vsock loopback is not available on this machine: %v", err)
	}
	if probeFd, _, err := unix.Accept(fd); err == nil {
		unix.Close(probeFd)
	}

	return fd, port
}

// readFully reads exactly len(buffer) bytes from fd.
func readFully(t *testing.T, fd int, buffer []byte) {
	for read := 0; read < len(buffer); {
		n, err := unix.Read(fd, buffer[read:])
		require.NoError(t, err)
		require.NotZero(t, n, "connection closed before the whole payload was read")
		read += n
	}
}

func TestVsockStreamWrite(t *testing.T) {
	listener, port := newVsockTestListener(t)
	defer unix.Close(listener)

	w, err := newVsockWriter(fmt.Sprintf("vsock://local:%d", port), 100*time.Millisecond, 1000*time.Millisecond)
	require.NoError(t, err)
	defer w.Close()

	conn := -1

	// test 2 Write: the first one should setup the connection
	for i := 0; i < 2; i++ {
		msg := []byte("some data")
		n, err := w.Write(msg)
		require.NoError(t, err)
		assert.Equal(t, len(msg), n)

		// This works because the kernel accepts sockets before the accept call
		if conn < 0 {
			conn, _, err = unix.Accept(listener)
			require.NoError(t, err)
			defer unix.Close(conn)
		}

		buffer := make([]byte, 4+len(msg))
		readFully(t, conn, buffer)
		assert.Equal(t, uint32(len(msg)), binary.LittleEndian.Uint32(buffer[:4]))
		assert.Equal(t, "some data", string(buffer[4:]))
	}
}

func TestVsockStreamWriteUnsetConnection(t *testing.T) {
	listener, port := newVsockTestListener(t)
	defer unix.Close(listener)

	w, err := newVsockWriter(fmt.Sprintf("vsock://local:%d", port), 100*time.Millisecond, 1000*time.Millisecond)
	require.NoError(t, err)
	defer w.Close()

	// Each iteration reconnects, as the Agent would force us to do when it restarts.
	for i := 0; i < 2; i++ {
		msg := []byte("some data")
		n, err := w.Write(msg)
		require.NoError(t, err)
		assert.Equal(t, len(msg), n)

		conn, _, err := unix.Accept(listener)
		require.NoError(t, err)

		buffer := make([]byte, 4+len(msg))
		readFully(t, conn, buffer)
		assert.Equal(t, uint32(len(msg)), binary.LittleEndian.Uint32(buffer[:4]))
		assert.Equal(t, "some data", string(buffer[4:]))

		unix.Close(conn)
		w.unsetConnection()
	}
}

// vsock ignores SO_SNDTIMEO while connecting and waits on its own connect timeout instead, which
// defaults to 2 seconds. Check that the timeout the client is configured with is the one that ends
// up on the socket, otherwise a peer that never answers blocks the sender for longer than asked.
func TestVsockConnectTimeoutIsAppliedToTheSocket(t *testing.T) {
	listener, port := newVsockTestListener(t)
	defer unix.Close(listener)

	connectTimeout := 250 * time.Millisecond
	w, err := newVsockWriter(fmt.Sprintf("vsock://local:%d", port), 100*time.Millisecond, connectTimeout)
	require.NoError(t, err)
	defer w.Close()

	conn, err := w.ensureConnection()
	require.NoError(t, err)

	tv, err := unix.GetsockoptTimeval(conn.(*vsockConn).fd, unix.AF_VSOCK, unix.SO_VM_SOCKETS_CONNECT_TIMEOUT)
	require.NoError(t, err)

	applied := time.Duration(tv.Sec)*time.Second + time.Duration(tv.Usec)*time.Microsecond
	// The kernel stores the timeout in jiffies, so it is rounded up to the resolution of the clock
	// tick this kernel was built with.
	assert.True(t, applied >= connectTimeout && applied < connectTimeout+50*time.Millisecond,
		"expected roughly %v, got %v", connectTimeout, applied)
}

// A payload we can't even start to write is reported as a timeout, and costs us the connection
// since the length delimiter announcing it has already been sent.
func TestVsockStreamPartialWrite(t *testing.T) {
	listener, port := newVsockTestListener(t)
	defer unix.Close(listener)

	w, err := newVsockWriter(fmt.Sprintf("vsock://local:%d", port), 100*time.Millisecond, 1000*time.Millisecond)
	require.NoError(t, err)
	defer w.Close()

	// Force a connection
	_, err = w.ensureConnection()
	require.NoError(t, err)
	conn, _, err := unix.Accept(listener)
	require.NoError(t, err)
	defer unix.Close(conn)

	// On linux we need to force a timeout this way
	w.connectTimeout = -1 * time.Millisecond

	msg := []byte("some data")
	n, err := w.Write(msg)
	require.Error(t, err)
	assert.True(t, n < len(msg), "n: %d, len(msg): %d", n, len(msg))

	// The writer relies on timeouts being reported as net.Error to tell them apart from a
	// disconnected Agent.
	netErr, ok := err.(net.Error)
	require.True(t, ok, "expected a net.Error, got %T: %v", err, err)
	assert.True(t, netErr.Timeout())

	// The connection should be dropped
	assert.Nil(t, w.conn)
}

// The client can be built from a vsock address and reports vsock as its transport.
func TestVsockClient(t *testing.T) {
	listener, port := newVsockTestListener(t)
	defer unix.Close(listener)

	client, err := NewEx(fmt.Sprintf("vsock://local:%d", port), WithoutOriginDetection())
	require.NoError(t, err)
	defer client.Close()

	assert.Equal(t, writerNameVsock, client.GetTransport())

	require.NoError(t, client.Gauge("my.gauge", 1, nil, 1))
	require.NoError(t, client.Flush())

	conn, _, err := unix.Accept(listener)
	require.NoError(t, err)
	defer unix.Close(conn)

	// The payload itself depends on the global tags the environment adds, so only its length
	// delimiter and the metric it starts with are checked.
	length := make([]byte, 4)
	readFully(t, conn, length)
	payload := make([]byte, binary.LittleEndian.Uint32(length))
	readFully(t, conn, payload)
	assert.True(t, strings.HasPrefix(string(payload), "my.gauge:1|g"), "unexpected payload: %q", payload)
}

// Connecting to a port nobody listens on is retried a few times, as the Agent may just be
// restarting. Note that vsock resets the connection instead of refusing it.
func TestVsockConnectionRetriedWhenNobodyListens(t *testing.T) {
	listener, port := newVsockTestListener(t)
	unix.Close(listener)

	connectTimeout := 400 * time.Millisecond
	w, err := newVsockWriter(fmt.Sprintf("vsock://local:%d", port), 100*time.Millisecond, connectTimeout)
	require.NoError(t, err)
	defer w.Close()

	start := time.Now()
	_, err = w.Write([]byte("some data"))
	require.Error(t, err)
	assert.True(t, isVsockConnectRetryable(err), "unexpected error: %v", err)

	// 3 attempts, so 2 backoffs of connectTimeout/4 between them.
	assert.True(t, time.Since(start) >= connectTimeout/2, "the connection should have been retried")
}

//go:build !windows
// +build !windows

package statsd

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAddr is a net.Addr reporting an arbitrary network, which is what tells the connWriter whether
// it needs to frame the payloads it writes.
type fakeAddr string

func (a fakeAddr) Network() string { return string(a) }
func (a fakeAddr) String() string  { return string(a) + ":fake" }

// fakeConn records everything written to it and fails the writes listed in writeErrors.
type fakeConn struct {
	network string
	written bytes.Buffer
	closed  bool

	// writeErrors holds one entry per Write call: a nil entry lets the write go through.
	writeErrors []error
	writeCount  int
}

func (c *fakeConn) Write(data []byte) (int, error) {
	var err error
	if c.writeCount < len(c.writeErrors) {
		err = c.writeErrors[c.writeCount]
	}
	c.writeCount++

	if err != nil {
		return 0, err
	}
	return c.written.Write(data)
}

func (c *fakeConn) Read(_ []byte) (int, error)         { return 0, errors.New("not implemented") }
func (c *fakeConn) Close() error                       { c.closed = true; return nil }
func (c *fakeConn) LocalAddr() net.Addr                { return fakeAddr(c.network) }
func (c *fakeConn) RemoteAddr() net.Addr               { return fakeAddr(c.network) }
func (c *fakeConn) SetDeadline(_ time.Time) error      { return nil }
func (c *fakeConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *fakeConn) SetWriteDeadline(_ time.Time) error { return nil }

// fakeDialer hands out the connections it was given, one per dial.
type fakeDialer struct {
	conns     []*fakeConn
	dialCount int
	name      string
}

func (d *fakeDialer) dial(_ time.Duration) (net.Conn, error) {
	if d.dialCount >= len(d.conns) {
		return nil, errors.New("no more connections")
	}
	conn := d.conns[d.dialCount]
	d.dialCount++
	return conn, nil
}

func (d *fakeDialer) transportName() string { return d.name }

// timeoutError is a net.Error reporting a timeout, like the error a connection returns when its
// write deadline is reached.
type timeoutError struct{}

func (e timeoutError) Error() string   { return "i/o timeout" }
func (e timeoutError) Timeout() bool   { return true }
func (e timeoutError) Temporary() bool { return true }

func newFakeWriter(network string, conns ...*fakeConn) (*connWriter, *fakeDialer) {
	for _, conn := range conns {
		conn.network = network
	}

	dialer := &fakeDialer{conns: conns, name: writerNameVsock}
	return newConnWriter(dialer, 100*time.Millisecond, 1000*time.Millisecond), dialer
}

// framed returns the length delimited encoding of the given payloads.
func framed(payloads ...string) []byte {
	var expected bytes.Buffer
	for _, payload := range payloads {
		length := []byte{0, 0, 0, 0}
		binary.LittleEndian.PutUint32(length, uint32(len(payload)))
		expected.Write(length)
		expected.WriteString(payload)
	}
	return expected.Bytes()
}

// Stream transports, vsock included, need every payload to be prefixed with its length.
func TestConnWriterStreamFraming(t *testing.T) {
	conn := &fakeConn{}
	w, dialer := newFakeWriter("vsock", conn)

	for _, payload := range []string{"some data", "some more data"} {
		n, err := w.Write([]byte(payload))
		require.NoError(t, err)
		assert.Equal(t, len(payload), n)
	}

	assert.Equal(t, framed("some data", "some more data"), conn.written.Bytes())
	assert.Equal(t, 1, dialer.dialCount, "the connection should have been reused")
	assert.Equal(t, writerNameVsock, w.GetTransportName())
}

// Datagram transports keep message boundaries on their own and must not be framed.
func TestConnWriterDatagramNoFraming(t *testing.T) {
	conn := &fakeConn{}
	w, _ := newFakeWriter("unixgram", conn)

	n, err := w.Write([]byte("some data"))
	require.NoError(t, err)
	assert.Equal(t, len("some data"), n)

	assert.Equal(t, "some data", conn.written.String())
}

// A payload that is only partially written leaves the stream out of sync with the length we
// announced, so the connection has to be dropped and re-established.
func TestConnWriterPartialWriteReconnects(t *testing.T) {
	// The second write of the first connection is the payload following its length.
	failing := &fakeConn{writeErrors: []error{nil, timeoutError{}}}
	healthy := &fakeConn{}
	w, dialer := newFakeWriter("vsock", failing, healthy)

	_, err := w.Write([]byte("some data"))
	require.Error(t, err)
	assert.True(t, failing.closed, "the connection should have been closed")

	n, err := w.Write([]byte("some data"))
	require.NoError(t, err)
	assert.Equal(t, len("some data"), n)

	assert.Equal(t, 2, dialer.dialCount, "a new connection should have been established")
	assert.Equal(t, framed("some data"), healthy.written.Bytes())
}

// Failing to write the length delimiter leaves the connection in the same unknown state: the Agent
// may have received part of it.
func TestConnWriterLengthWriteFailureReconnects(t *testing.T) {
	failing := &fakeConn{writeErrors: []error{errors.New("some write error")}}
	healthy := &fakeConn{}
	w, dialer := newFakeWriter("vsock", failing, healthy)

	_, err := w.Write([]byte("some data"))
	require.Error(t, err)
	assert.True(t, failing.closed, "the connection should have been closed")
	assert.Equal(t, 0, failing.written.Len())

	_, err = w.Write([]byte("some data"))
	require.NoError(t, err)
	assert.Equal(t, 2, dialer.dialCount, "a new connection should have been established")
}

func TestDialWithRetryOnConnectionRefused(t *testing.T) {
	attempts := 0
	conn := &fakeConn{network: "vsock"}

	// Connection refused is retried, as the Agent is likely restarting.
	newConn, err := dialWithRetry(10*time.Millisecond, isConnectionRefused, func(_ time.Duration) (net.Conn, error) {
		attempts++
		if attempts < 3 {
			return nil, errors.New("dial vsock 2:8125: connect: connection refused")
		}
		return conn, nil
	})

	require.NoError(t, err)
	assert.Equal(t, conn, newConn)
	assert.Equal(t, 3, attempts)
}

func TestDialWithRetryGivesUp(t *testing.T) {
	attempts := 0

	// Any other error means retrying is pointless.
	_, err := dialWithRetry(10*time.Millisecond, isConnectionRefused, func(_ time.Duration) (net.Conn, error) {
		attempts++
		return nil, errors.New("no such device")
	})

	require.Error(t, err)
	assert.Equal(t, 1, attempts)
}

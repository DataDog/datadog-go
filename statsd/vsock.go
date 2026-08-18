//go:build linux
// +build linux

package statsd

import (
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

// vsockNetwork is the network name reported by vsock addresses.
const vsockNetwork = "vsock"

// vsockDialer is an internal class connecting to the Agent over a vsock socket
type vsockDialer struct {
	cid  uint32
	port uint32
}

// newVsockWriter returns a pointer to a new writer given a vsock address as addr.
func newVsockWriter(addr string, writeTimeout time.Duration, connectTimeout time.Duration) (*connWriter, error) {
	cid, port, err := parseVsockAddr(addr)
	if err != nil {
		return nil, err
	}

	return newConnWriter(&vsockDialer{cid: cid, port: port}, writeTimeout, connectTimeout), nil
}

// transportName returns the transport used by the dialer
func (d *vsockDialer) transportName() string {
	return writerNameVsock
}

func (d *vsockDialer) dial(connectTimeout time.Duration) (net.Conn, error) {
	// Try to gracefully reconnect to the socket when nothing is listening, as it's likely that the
	// Agent is restarting.
	return dialWithRetry(connectTimeout, isVsockConnectRetryable, d.tryToDial)
}

// isVsockConnectRetryable reports whether a failure to connect is worth retrying. On top of the
// usual "connection refused", vsock transports reset the connection when no socket is bound to the
// port we are connecting to.
func isVsockConnectRetryable(err error) bool {
	return isConnectionRefused(err) || strings.HasSuffix(err.Error(), "connection reset by peer")
}

func (d *vsockDialer) tryToDial(connectTimeout time.Duration) (net.Conn, error) {
	fd, err := unix.Socket(unix.AF_VSOCK, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to create vsock socket: %v", err)
	}

	// vsock does not honor SO_SNDTIMEO while connecting: af_vsock waits on the socket's own
	// connect timeout instead, which defaults to 2 seconds. Set that one so that the connect
	// timeout the client was configured with is the one that actually applies.
	if err := setSocketTimeout(fd, unix.AF_VSOCK, unix.SO_VM_SOCKETS_CONNECT_TIMEOUT, connectTimeout); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("failed to set vsock connect timeout: %v", err)
	}

	if err := unix.Connect(fd, &unix.SockaddrVM{CID: d.cid, Port: d.port}); err != nil {
		unix.Close(fd)
		// The error is kept at the end of the message so that isVsockConnectRetryable can still
		// recognize it.
		return nil, fmt.Errorf("failed to connect to vsock CID %d port %d: %v", d.cid, d.port, err)
	}

	return newVsockConn(fd, &vsockAddr{cid: d.cid, port: d.port}), nil
}

// vsockAddr is the net.Addr of a vsock endpoint.
type vsockAddr struct {
	cid  uint32
	port uint32
}

func (a *vsockAddr) Network() string {
	return vsockNetwork
}

func (a *vsockAddr) String() string {
	return fmt.Sprintf("%d:%d", a.cid, a.port)
}

// vsockConn is a minimal net.Conn implementation for AF_VSOCK sockets, which the standard library
// does not support.
//
// The file descriptor is left in blocking mode and deadlines are implemented with SO_SNDTIMEO /
// SO_RCVTIMEO rather than with the runtime poller: os.NewFile only hands sockets over to the poller
// on recent versions of Go, and this client still supports much older ones.
type vsockConn struct {
	fd     int
	local  net.Addr
	remote net.Addr

	closeOnce sync.Once

	// Deadlines are applied to the socket before each read and write. They are only accessed from
	// the goroutine doing the I/O, like the deadlines of the connections returned by net.Dial.
	readDeadline  time.Time
	writeDeadline time.Time
}

// Verify that vsockConn implements net.Conn.
var _ net.Conn = &vsockConn{}

func newVsockConn(fd int, remote net.Addr) *vsockConn {
	c := &vsockConn{fd: fd, local: &vsockAddr{}, remote: remote}

	// The local address is only used to report the network of the connection, so a failure to
	// resolve it is not worth failing the connection over.
	if sa, err := unix.Getsockname(fd); err == nil {
		if vm, ok := sa.(*unix.SockaddrVM); ok {
			c.local = &vsockAddr{cid: vm.CID, port: vm.Port}
		}
	}

	return c
}

// Write writes data to the connection, honoring the deadline set by SetWriteDeadline. It returns
// the number of bytes written, which can be less than len(data) if the deadline is reached while
// writing.
func (c *vsockConn) Write(data []byte) (int, error) {
	written := 0
	for written < len(data) {
		// A blocking socket with SO_SNDTIMEO returns a short write, without an error, when the
		// timeout is reached mid-write, so the time left is recomputed on every iteration.
		if err := c.applyDeadline(unix.SO_SNDTIMEO, c.writeDeadline); err != nil {
			return written, err
		}

		n, err := unix.Write(c.fd, data[written:])
		if n > 0 {
			written += n
		}
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			return written, err
		}
		if n <= 0 {
			// Not expected for a non-empty buffer, but we'd rather return an error than spin.
			return written, unix.EAGAIN
		}
	}
	return written, nil
}

// Read reads from the connection, honoring the deadline set by SetReadDeadline.
func (c *vsockConn) Read(data []byte) (int, error) {
	for {
		if err := c.applyDeadline(unix.SO_RCVTIMEO, c.readDeadline); err != nil {
			return 0, err
		}

		n, err := unix.Read(c.fd, data)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return 0, err
		}
		if n == 0 && len(data) > 0 {
			return 0, io.EOF
		}
		return n, nil
	}
}

func (c *vsockConn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		err = unix.Close(c.fd)
	})
	return err
}

func (c *vsockConn) LocalAddr() net.Addr {
	return c.local
}

func (c *vsockConn) RemoteAddr() net.Addr {
	return c.remote
}

func (c *vsockConn) SetDeadline(t time.Time) error {
	c.readDeadline = t
	c.writeDeadline = t
	return nil
}

func (c *vsockConn) SetReadDeadline(t time.Time) error {
	c.readDeadline = t
	return nil
}

func (c *vsockConn) SetWriteDeadline(t time.Time) error {
	c.writeDeadline = t
	return nil
}

// applyDeadline sets the time left until deadline as a socket timeout. It returns EAGAIN, which
// reports itself as a timeout to net.Error users, when the deadline has already passed.
func (c *vsockConn) applyDeadline(opt int, deadline time.Time) error {
	if deadline.IsZero() {
		// No deadline: clear any timeout previously set on the socket.
		tv := unix.NsecToTimeval(0)
		return unix.SetsockoptTimeval(c.fd, unix.SOL_SOCKET, opt, &tv)
	}

	timeLeft := time.Until(deadline)
	if timeLeft <= 0 {
		return unix.EAGAIN
	}
	return setSocketTimeout(c.fd, unix.SOL_SOCKET, opt, timeLeft)
}

// setSocketTimeout sets a timeout option on fd. A zero timeval means "no timeout" to the kernel, so
// non-positive timeouts are clamped to the smallest value it understands to make the operation they
// bound fail fast instead of blocking forever.
func setSocketTimeout(fd int, level int, opt int, timeout time.Duration) error {
	if timeout < time.Microsecond {
		timeout = time.Microsecond
	}

	tv := unix.NsecToTimeval(timeout.Nanoseconds())
	return unix.SetsockoptTimeval(fd, level, opt, &tv)
}

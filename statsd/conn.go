//go:build !windows
// +build !windows

package statsd

import (
	"encoding/binary"
	"net"
	"strings"
	"sync"
	"time"
)

// connDialer establishes the connections used by a connWriter.
type connDialer interface {
	// dial connects to the Agent, giving up after connectTimeout.
	dial(connectTimeout time.Duration) (net.Conn, error)

	// transportName returns the name of the transport. It can depend on the connection that
	// dial ultimately established, as UDS guesses between datagram and stream sockets.
	transportName() string
}

// connWriter is an internal class wrapping around management of a connection to the Agent. The
// connection is established on the first write and re-established whenever the Agent disconnects.
type connWriter struct {
	// Dialer used to establish new connections
	dialer connDialer
	// Established connection object, or nil if not connected yet
	conn net.Conn
	// write timeout
	writeTimeout time.Duration
	// connect timeout
	connectTimeout time.Duration
	sync.RWMutex   // used to lock conn / writer can replace it
}

// newConnWriter returns a pointer to a new connWriter using the given dialer.
func newConnWriter(dialer connDialer, writeTimeout time.Duration, connectTimeout time.Duration) *connWriter {
	// Defer connection to first Write
	return &connWriter{dialer: dialer, conn: nil, writeTimeout: writeTimeout, connectTimeout: connectTimeout}
}

// GetTransportName returns the transport used by the writer
func (w *connWriter) GetTransportName() string {
	w.RLock()
	defer w.RUnlock()

	return w.dialer.transportName()
}

// isStreamConn reports whether conn needs length-delimited framing: datagram transports preserve
// message boundaries, stream transports do not.
func isStreamConn(conn net.Conn) bool {
	return conn.LocalAddr().Network() != "unixgram"
}

func (w *connWriter) shouldCloseConnection(err error, partialWrite bool) bool {
	if err != nil && partialWrite {
		// We can't recover from a partial write
		return true
	}
	if err, isNetworkErr := err.(net.Error); err != nil && (!isNetworkErr || !err.Timeout()) {
		// Statsd server disconnected, retry connecting at next packet
		return true
	}
	return false
}

// Write data to the connection with write timeout and minimal error handling:
// create the connection if nil, and destroy it if the statsd server has disconnected
func (w *connWriter) Write(data []byte) (int, error) {
	var n int
	partialWrite := false
	conn, err := w.ensureConnection()
	if err != nil {
		return 0, err
	}
	stream := isStreamConn(conn)

	// When using streams the deadline will only make us drop the packet if we can't write it at all,
	// once we've started writing we need to finish.
	conn.SetWriteDeadline(time.Now().Add(w.writeTimeout))

	// When using streams, we append the length of the packet to the data
	if stream {
		bs := []byte{0, 0, 0, 0}
		binary.LittleEndian.PutUint32(bs, uint32(len(data)))
		_, err = conn.Write(bs)

		partialWrite = true

		// W need to be able to finish to write partially written packets once we have started.
		// But we will reset the connection if we can't write anything at all for a long time.
		conn.SetWriteDeadline(time.Now().Add(w.connectTimeout))

		// Continue writing only if we've written the length of the packet
		if err == nil {
			n, err = conn.Write(data)
			if err == nil {
				partialWrite = false
			}
		}
	} else {
		n, err = conn.Write(data)
	}

	if w.shouldCloseConnection(err, partialWrite) {
		w.unsetConnection()
	}
	return n, err
}

func (w *connWriter) Close() error {
	if w.conn != nil {
		return w.conn.Close()
	}
	return nil
}

func (w *connWriter) ensureConnection() (net.Conn, error) {
	// Check if we've already got a socket we can use
	w.RLock()
	currentConn := w.conn
	w.RUnlock()

	if currentConn != nil {
		return currentConn, nil
	}

	// Looks like we might need to connect - try again with write locking.
	w.Lock()
	defer w.Unlock()
	if w.conn != nil {
		return w.conn, nil
	}

	newConn, err := w.dialer.dial(w.connectTimeout)
	if err != nil {
		return nil, err
	}
	w.conn = newConn
	return newConn, nil
}

func (w *connWriter) unsetConnection() {
	w.Lock()
	defer w.Unlock()
	_ = w.conn.Close()
	w.conn = nil
}

// isConnectionRefused reports whether err means that nothing is listening on the other end. The
// error message is matched, rather than the error itself, because errors.Is is not available in the
// oldest versions of Go this library supports.
func isConnectionRefused(err error) bool {
	return strings.HasSuffix(err.Error(), "connection refused")
}

// dialWithRetry calls dial until it succeeds, the connect timeout expires, or dial fails with an
// error that isRetryable rejects. Errors meaning that nothing is listening are worth retrying: it's
// likely that the Agent is restarting in that case, and that it will be back shortly.
func dialWithRetry(connectTimeout time.Duration, isRetryable func(error) bool, dial func(timeout time.Duration) (net.Conn, error)) (net.Conn, error) {
	connectAttemptsLeft := 3
	connectDeadline := time.Now().Add(connectTimeout)

	// Calculate the backoff time for connection refused errors, but don't exceed one second: this means we won't waste
	// longer than 1 seconds worth of time if the socket becomes available immediately after our last connect attempt
	connRefusedBackoff := connectTimeout / time.Duration(connectAttemptsLeft+1)
	if connRefusedBackoff > time.Second {
		connRefusedBackoff = time.Second
	}

	for {
		connectAttemptsLeft--

		perCallTimeout := time.Until(connectDeadline)
		newConn, err := dial(perCallTimeout)
		if err != nil {
			if isRetryable(err) && connectAttemptsLeft > 0 {
				// If we get a retryable error, we need to wait a bit before trying again.
				time.Sleep(connRefusedBackoff)
				continue
			}
			return nil, err
		}
		return newConn, nil
	}
}

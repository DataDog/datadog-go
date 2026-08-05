//go:build !windows
// +build !windows

package statsd

import (
	"net"
	"strings"
	"time"
)

// udsDialer is an internal class connecting to the Agent over a Unix Domain Socket
type udsDialer struct {
	// Address to send metrics to, needed to allow reconnection on error
	addr string
	// Transport used
	transport string
}

// newUDSWriter returns a pointer to a new writer given a socket file path as addr.
func newUDSWriter(addr string, writeTimeout time.Duration, connectTimeout time.Duration, transport string) (*connWriter, error) {
	return newConnWriter(&udsDialer{addr: addr, transport: transport}, writeTimeout, connectTimeout), nil
}

// transportName returns the transport used by the dialer
func (d *udsDialer) transportName() string {
	if d.transport == "unix" {
		return writerNameUDSStream
	} else {
		return writerNameUDS
	}
}

func (d *udsDialer) dial(connectTimeout time.Duration) (net.Conn, error) {
	var newConn net.Conn
	var err error

	// Try to guess the transport if not specified.
	if d.transport == "" {
		newConn, err = d.tryToDial("unixgram", connectTimeout)
		// try to connect with unixgram failed, try again with unix streams.
		if err != nil && strings.Contains(err.Error(), "protocol wrong type for socket") {
			newConn, err = d.tryToDial("unix", connectTimeout)
		}
	} else {
		newConn, err = d.tryToDial(d.transport, connectTimeout)
	}

	if err != nil {
		return nil, err
	}
	d.transport = newConn.RemoteAddr().Network()
	return newConn, nil
}

func (d *udsDialer) tryToDial(network string, connectTimeout time.Duration) (net.Conn, error) {
	udsAddr, err := net.ResolveUnixAddr(network, d.addr)
	if err != nil {
		return nil, err
	}

	// Try to gracefully reconnect to the socket when we encounter "connection refused", as it's likely that the Agent
	// is restarting and the socket is not yet available.
	return dialWithRetry(connectTimeout, isConnectionRefused, func(timeout time.Duration) (net.Conn, error) {
		return net.DialTimeout(udsAddr.Network(), udsAddr.String(), timeout)
	})
}

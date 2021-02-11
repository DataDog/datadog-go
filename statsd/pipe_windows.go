// +build windows

package statsd

import (
	"net"
	"sync"
	"time"

	"github.com/Microsoft/go-winio"
)

const defaultPipeTimeout = 1 * time.Millisecond

type pipeWriter struct {
	mu       sync.RWMutex
	conn     net.Conn
	timeout  time.Duration
	pipepath string
}

func (p *pipeWriter) SetWriteTimeout(d time.Duration) error {
	p.timeout = d
	return nil
}

func (p *pipeWriter) Write(data []byte) (n int, err error) {
	conn, err := p.ensureConnection()
	if err != nil {
		return 0, err
	}

	p.mu.RLock()
	conn.SetWriteDeadline(time.Now().Add(p.timeout))
	p.mu.RUnlock()

	n, err = conn.Write(data)
	if err != nil {
		if e, ok := err.(net.Error); !ok || !e.Temporary() {
			// disconnected; retry again on next attempt
			p.mu.Lock()
			p.conn = nil
			p.mu.Unlock()
		}
	}
	return n, err
}

func (p *pipeWriter) ensureConnection() (net.Conn, error) {
	p.mu.RLock()
	conn := p.conn
	p.mu.RUnlock()
	if conn != nil {
		return conn, nil
	}

	// looks like we might need to connect - try again with write locking.
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn != nil {
		return p.conn, nil
	}
	newconn, err := winio.DialPipe(p.pipepath, nil)
	if err != nil {
		return nil, err
	}
	p.conn = newconn
	return newconn, nil
}

func (p *pipeWriter) Close() error {
	return p.conn.Close()
}

func newWindowsPipeWriter(pipepath string) (*pipeWriter, error) {
	conn, err := winio.DialPipe(pipepath, nil)
	return &pipeWriter{
		conn:     conn,
		timeout:  defaultPipeTimeout,
		pipepath: pipepath,
	}, err
}

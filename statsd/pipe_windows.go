// +build windows

package statsd

import (
	"errors"
	"net"
	"time"

	"github.com/Microsoft/go-winio"
)

type pipeWriter struct{ net.Conn }

func (pipeWriter) SetWriteTimeout(_ time.Duration) error {
	return errors.New("SetWriteTimeout is not supported on Windows Named Pipes")
}

func newWindowsPipeWriter(pipepath string) (*pipeWriter, error) {
	conn, err := winio.DialPipe(pipepath, nil)
	return &pipeWriter{conn}, err
}

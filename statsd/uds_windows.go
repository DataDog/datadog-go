// +build windows

package statsd

import (
	"fmt"
	"io"
	"time"
)

// newUDSWriter is disable on windows as unix sockets are not available
func newUDSWriter(addr string, writeTimeout time.Duration) (io.WriteCloser, error) {
	return nil, fmt.Errorf("unix socket is not available on windows")
}

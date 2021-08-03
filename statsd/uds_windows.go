// +build windows

package statsd

import "fmt"

// newUDSWriter is disable on windows as unix sockets are not available
func newUDSWriter(addr string) (io.WriteCloser, error) {
	return nil, fmt.Errorf("unix socket is not available on windows")
}

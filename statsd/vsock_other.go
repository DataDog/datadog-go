//go:build !linux
// +build !linux

package statsd

import (
	"fmt"
	"time"
)

// newVsockWriter is disabled outside of Linux: AF_VSOCK is a Linux specific address family.
func newVsockWriter(_ string, _ time.Duration, _ time.Duration) (Transport, error) {
	return nil, fmt.Errorf("vsock is only supported on Linux")
}

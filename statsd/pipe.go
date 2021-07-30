// +build !windows

package statsd

import (
	"errors"
	"time"
)

func newWindowsPipeWriter(pipepath string, writeTimeout time.Duration) (statsdWriter, error) {
	return nil, errors.New("Windows Named Pipes are only supported on Windows")
}

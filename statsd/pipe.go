// +build !windows

package statsd

import "errors"

func newWindowsPipeWriter(pipepath string) (statsdWriter, error) {
	return nil, errors.New("Windows Named Pipes are not supported on other operating systems than Windows")
}

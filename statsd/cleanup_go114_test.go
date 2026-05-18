//go:build go1.14
// +build go1.14

package statsd

import "testing"

func registerCleanup(t *testing.T, f func()) {
	t.Cleanup(f)
}

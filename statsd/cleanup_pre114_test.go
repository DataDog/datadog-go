//go:build !go1.14
// +build !go1.14

package statsd

import "testing"

// t.Cleanup is not available before Go 1.14, so this is a no-op on Go
// 1.13. Tests that assert payloads without a container-ID suffix must
// therefore call withoutOriginGlobals at their start to reset the
// package globals explicitly.
//
// Tests in format_test.go are safe without that call: they run before
// any client-constructing test (alphabetical file order), and the only
// earlier file that touches containerID (container_test.go, Linux-only)
// restores it with a defer in each test body.
func registerCleanup(t *testing.T, f func()) {}

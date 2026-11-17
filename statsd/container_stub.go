//go:build !linux
// +build !linux

package statsd

func isHostCgroupNamespace() bool {
	return false
}

// detectContainerID is not supported on non-Linux platforms.
func detectContainerID() string {
	return ""
}

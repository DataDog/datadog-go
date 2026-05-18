package statsd

import (
	"sync"
	"sync/atomic"
)

var (
	containerID atomic.Value
	initOnce    sync.Once
)

// getContainerID returns the container ID configured at the client creation
// It can either be auto-discovered with origin detection or provided by the user.
// User-defined container ID is prioritized.
func getContainerID() string {
	if v := containerID.Load(); v != nil {
		return v.(string)
	}
	return ""
}

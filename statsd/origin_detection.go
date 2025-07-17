package statsd

import "sync"

var (
	originDetection      = true
	originDetectionMutex sync.RWMutex
)

func getOriginDetection() bool {
	originDetectionMutex.RLock()
	defer originDetectionMutex.RUnlock()
	return originDetection
}

func setOriginDetection(enabled bool) {
	originDetectionMutex.Lock()
	defer originDetectionMutex.Unlock()
	originDetection = enabled
}

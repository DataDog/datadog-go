//go:build go1.19
// +build go1.19

package fastrand

import (
	_ "unsafe"
)

const (
	rngMax  = 1 << 63
	rngMask = rngMax - 1
)

//go:linkname runtimeFastrand64 runtime.fastrand64
func runtimeFastrand64() uint64

// Float64 returns a pseudo-random float64 in [0, 1)
func Float64() float64 {
	return float64(int64(runtimeFastrand64()&rngMask)&(1<<53-1)) / (1 << 53)
}

//go:build !go1.19
// +build !go1.19

package fastrand

import (
	_ "unsafe"
)

const (
	rngMax  = 1 << 63
	rngMask = rngMax - 1
)

//go:linkname runtimeFastrand runtime.fastrand
func runtimeFastrand() uint32

// Float64 returns a pseudo-random float64 in [0, 1)
func Float64() float64 {
	// Use 2 pseudo-random uint32 to create 1 pseudo-random uint64
	r := (uint64(runtimeFastrand()) << 32) | uint64(runtimeFastrand())
	return float64(int64(r&rngMask)&(1<<53-1)) / (1 << 53)
}

//go:build !windows
// +build !windows

package statsd

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
)

// TODO: implement the same test for uds-stream
func TestFullPipelineUDS(t *testing.T) {
	for testName, c := range getTestMap() {
		socketPath := fmt.Sprintf("/tmp/dsd_%d.socket", rand.Int())
		t.Run(testName, func(t *testing.T) {
			ts, client := newClientAndTestServer(t,
				"uds",
				"unix://"+socketPath,
				nil,
				c.opt...,
			)
			c.testFunc(t, ts, client)
		})
		os.Remove(socketPath)
	}
}

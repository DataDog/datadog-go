package statsd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVsockAddr(t *testing.T) {
	for _, tc := range []struct {
		name         string
		addr         string
		expectedCID  uint32
		expectedPort uint32
		expectedErr  string
	}{
		{"numeric CID", "vsock://2:8125", 2, 8125, ""},
		{"CID zero", "vsock://0:8125", 0, 8125, ""},
		{"max CID and port", "vsock://4294967295:4294967295", 4294967295, 4294967295, ""},
		{"without the scheme", "3:8125", 3, 8125, ""},

		{"hypervisor shorthand", "vsock://hypervisor:8125", vsockCIDHypervisor, 8125, ""},
		{"local shorthand", "vsock://local:8125", vsockCIDLocal, 8125, ""},
		{"host shorthand", "vsock://host:8125", vsockCIDHost, 8125, ""},
		{"shorthand is case insensitive", "vsock://Host:8125", vsockCIDHost, 8125, ""},

		{"missing port", "vsock://host", 0, 0, "missing port in address"},
		{"empty port", "vsock://host:", 0, 0, "port must be a number"},
		{"port zero", "vsock://host:0", 0, 0, "port must be a number"},
		{"port too large", "vsock://host:4294967296", 0, 0, "port must be a number"},
		{"port is not a number", "vsock://host:statsd", 0, 0, "port must be a number"},

		{"empty CID", "vsock://:8125", 0, 0, "CID must be a number"},
		{"unknown shorthand", "vsock://guest:8125", 0, 0, "CID must be a number"},
		{"CID too large", "vsock://4294967296:8125", 0, 0, "CID must be a number"},
		{"negative CID", "vsock://-1:8125", 0, 0, "CID must be a number"},

		{"empty address", "vsock://", 0, 0, "missing port in address"},
		{"too many colons", "vsock://2:8125:9", 0, 0, "too many colons in address"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cid, port, err := parseVsockAddr(tc.addr)

			if tc.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr)
				// The address is always quoted in the error so that users can tell which one of
				// their addresses is invalid.
				assert.Contains(t, err.Error(), `"`+tc.addr+`"`)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expectedCID, cid)
			assert.Equal(t, tc.expectedPort, port)
		})
	}
}

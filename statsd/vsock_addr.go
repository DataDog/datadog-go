package statsd

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

/*
Well-known vsock context IDs, mirroring the VMADDR_CID_* constants from the kernel. They are
declared here, instead of being taken from golang.org/x/sys/unix, so that vsock addresses can be
parsed on the platforms where vsock itself is not supported.
*/
const (
	vsockCIDHypervisor uint32 = 0
	vsockCIDLocal      uint32 = 1
	vsockCIDHost       uint32 = 2
)

// vsockCIDNames maps the shorthands accepted in a vsock address to their context ID.
var vsockCIDNames = map[string]uint32{
	"hypervisor": vsockCIDHypervisor,
	"local":      vsockCIDLocal,
	"host":       vsockCIDHost,
}

// parseVsockAddr parses a "vsock://<CID>:<port>" address, with or without the scheme, and returns
// the context ID and port it points to. The CID is either a number or one of the well-known
// shorthands: hypervisor, local or host.
func parseVsockAddr(addr string) (uint32, uint32, error) {
	cidStr, portStr, err := net.SplitHostPort(strings.TrimPrefix(addr, VsockAddressPrefix))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid vsock address %q: %v", addr, err)
	}

	cid, err := parseVsockCID(cidStr)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid vsock address %q: %v", addr, err)
	}

	port, err := strconv.ParseUint(portStr, 10, 32)
	if err != nil || port == 0 {
		return 0, 0, fmt.Errorf("invalid vsock address %q: port must be a number between 1 and 4294967295", addr)
	}

	return cid, uint32(port), nil
}

// parseVsockCID parses a context ID, given either as a number or as one of the well-known
// shorthands.
func parseVsockCID(cidStr string) (uint32, error) {
	if cid, found := vsockCIDNames[strings.ToLower(cidStr)]; found {
		return cid, nil
	}

	cid, err := strconv.ParseUint(cidStr, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("CID must be a number between 0 and 4294967295, or one of: hypervisor, local, host")
	}
	return uint32(cid), nil
}

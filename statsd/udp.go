package statsd

import (
	"net"
	"time"
)

// udpWriter is an internal class wrapping around management of UDP connection
type udpWriter struct {
	conn net.PacketConn
	addr string
}

// New returns a pointer to a new udpWriter given an addr in the format "hostname:port".
func newUDPWriter(addr string, _ time.Duration) (*udpWriter, error) {
	conn, err := net.ListenPacket("udp", ":0")
	if err != nil {
		return nil, err
	}
	writer := &udpWriter{conn: conn, addr: addr}
	return writer, nil
}

// Write data to the UDP connection with no error handling
func (w *udpWriter) Write(data []byte) (int, error) {
	dst, err := net.ResolveUDPAddr("udp", w.addr)
	if err != nil {
		return 0, err
	}
	return w.conn.WriteTo(data, dst)
}

func (w *udpWriter) Close() error {
	return w.conn.Close()
}

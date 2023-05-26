package statsd

import (
	"net"
	"sync"
	"time"
)

// udpWriter is an internal class wrapping around management of UDP connection
type udpWriter struct {
	conn   net.PacketConn
	addr   string
	dst    *dstValue
	closed chan struct{}
}

type dstValue struct {
	mutex sync.RWMutex
	dst   *net.UDPAddr
}

func (d *dstValue) set(dst *net.UDPAddr) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.dst = dst
}

func (d *dstValue) get() *net.UDPAddr {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return d.dst
}

// New returns a pointer to a new udpWriter given an addr in the format "hostname:port".
func newUDPWriter(addr string, _ time.Duration, refreshRate time.Duration) (*udpWriter, error) {
	conn, err := net.ListenPacket("udp", ":0")
	if err != nil {
		return nil, err
	}
	currentDst, err := getCurrentDst(addr)
	if err != nil {
		return nil, err
	}
	dst := &dstValue{dst: currentDst}
	writer := &udpWriter{conn: conn, addr: addr, dst: dst, closed: make(chan struct{})}
	if refreshRate > 0 {
		go writer.refreshDstLoop(refreshRate)
	}
	return writer, nil
}

func (w *udpWriter) refreshDstLoop(refreshRate time.Duration) {
	ticker := time.NewTicker(refreshRate)
	defer ticker.Stop()
	for {
		select {
		case <-w.closed:
			return
		case <-ticker.C:
			dst, err := getCurrentDst(w.addr)
			if err != nil {
				continue
			}
			w.dst.set(dst)
		}
	}
}

// Write data to the UDP connection with no error handling
func (w *udpWriter) Write(data []byte) (int, error) {
	return w.conn.WriteTo(data, w.dst.get())
}

func (w *udpWriter) Close() error {
	close(w.closed)
	return w.conn.Close()
}

func getCurrentDst(addr string) (*net.UDPAddr, error) {
	return net.ResolveUDPAddr("udp", addr)
}

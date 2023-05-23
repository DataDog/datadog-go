package statsd

import (
	"math/rand"
	"net"
	"sync"
	"time"
)

// udpWriter is an internal class wrapping around management of UDP connection
type udpWriter struct {
	conns      []net.Conn
	random     *rand.Rand
	randomLock sync.Mutex
}

// New returns a pointer to a new udpWriter given an addr in the format "hostname:port".
func newUDPWriter(addr string, _ time.Duration, udpSocketCount int) (*udpWriter, error) {
	conns := make([]net.Conn, udpSocketCount)
	for i := range conns {
		udpAddr, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			return nil, err
		}
		conn, err := net.DialUDP("udp", nil, udpAddr)
		if err != nil {
			return nil, err
		}
		conns[i] = conn
	}
	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	writer := &udpWriter{conns: conns, random: random}
	return writer, nil
}

// Write data to the UDP connection with no error handling
func (w *udpWriter) Write(data []byte) (int, error) {
	if len(w.conns) == 1 {
		return w.conns[0].Write(data)
	}
	w.randomLock.Lock()
	c := w.random.Intn(len(w.conns))
	w.randomLock.Unlock()
	return w.conns[c].Write(data)
}

func (w *udpWriter) Close() error {
	var err error
	for i := range w.conns {
		e := w.conns[i].Close()
		if e != nil {
			err = e
		}
	}
	return err
}

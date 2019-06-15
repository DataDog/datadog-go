package statsd

import (
	"sync/atomic"
	"time"
)

// A statsdWriter offers a standard interface regardless of the underlying
// protocol. For now UDS and UPD writers are available.
// Attention: the underlying buffer of `data` is reused after a `statsdWriter.Write` call.
// `statsdWriter.Write` must be synchronous.
type statsdWriter interface {
	Write(data []byte) (n int, err error)
	SetWriteTimeout(time.Duration) error
	Close() error
}

// SenderMetrics contains metrics about the health of the sender
type SenderMetrics struct {
	TotalSentBytes                uint64
	TotalSentPayloads             uint64
	TotalDroppedPayloads          uint64
	TotalDroppedBytes             uint64
	TotalDroppedPayloadsQueueFull uint64
	TotalDroppedBytesQueueFull    uint64
	TotalDroppedPayloadsWriter    uint64
	TotalDroppedBytesWriter       uint64
}

type sender struct {
	transport statsdWriter
	pool      *bufferPool
	queue     chan *statsdBuffer
	metrics   SenderMetrics
	stop      chan struct{}
}

func newSender(transport statsdWriter, queueSize int, pool *bufferPool) *sender {
	sender := &sender{
		transport: transport,
		pool:      pool,
		queue:     make(chan *statsdBuffer, queueSize),
		stop:      make(chan struct{}),
	}

	go sender.sendLoop()
	return sender
}

func (s *sender) send(buffer *statsdBuffer) {
	select {
	case s.queue <- buffer:
	default:
		atomic.AddUint64(&s.metrics.TotalDroppedPayloads, 1)
		atomic.AddUint64(&s.metrics.TotalDroppedBytes, uint64(len(buffer.bytes())))
		atomic.AddUint64(&s.metrics.TotalDroppedPayloadsQueueFull, 1)
		atomic.AddUint64(&s.metrics.TotalDroppedBytesQueueFull, uint64(len(buffer.bytes())))
		s.pool.returnBuffer(buffer)
	}
}

func (s *sender) write(buffer *statsdBuffer) {
	_, err := s.transport.Write(buffer.bytes())
	if err != nil {
		atomic.AddUint64(&s.metrics.TotalDroppedPayloads, 1)
		atomic.AddUint64(&s.metrics.TotalDroppedBytes, uint64(len(buffer.bytes())))
		atomic.AddUint64(&s.metrics.TotalDroppedPayloadsWriter, 1)
		atomic.AddUint64(&s.metrics.TotalDroppedBytesWriter, uint64(len(buffer.bytes())))
	} else {
		atomic.AddUint64(&s.metrics.TotalSentPayloads, 1)
		atomic.AddUint64(&s.metrics.TotalSentBytes, uint64(len(buffer.bytes())))
	}
	s.pool.returnBuffer(buffer)
}

func (s *sender) getMetrics() SenderMetrics {
	return SenderMetrics{
		TotalSentBytes:                atomic.LoadUint64(&s.metrics.TotalSentBytes),
		TotalSentPayloads:             atomic.LoadUint64(&s.metrics.TotalSentPayloads),
		TotalDroppedPayloads:          atomic.LoadUint64(&s.metrics.TotalDroppedPayloads),
		TotalDroppedBytes:             atomic.LoadUint64(&s.metrics.TotalDroppedBytes),
		TotalDroppedPayloadsQueueFull: atomic.LoadUint64(&s.metrics.TotalDroppedPayloadsQueueFull),
		TotalDroppedBytesQueueFull:    atomic.LoadUint64(&s.metrics.TotalDroppedBytesQueueFull),
		TotalDroppedPayloadsWriter:    atomic.LoadUint64(&s.metrics.TotalDroppedPayloadsWriter),
		TotalDroppedBytesWriter:       atomic.LoadUint64(&s.metrics.TotalDroppedBytesWriter),
	}
}

func (s *sender) sendLoop() {
	for {
		select {
		case buffer := <-s.queue:
			s.write(buffer)
		case <-s.stop:
			return
		}
	}
}

func (s *sender) flush() {
	for {
		select {
		case buffer := <-s.queue:
			s.write(buffer)
		default:
			return
		}
	}
}

func (s *sender) close() error {
	s.flush()
	err := s.transport.Close()
	close(s.stop)
	return err
}

package statsd

import "time"

// A statsdWriter offers a standard interface regardless of the underlying
// protocol. For now UDS and UPD writers are available.
// Attention: the underlying buffer of `data` is reused after a `statsdWriter.Write` call.
// `statsdWriter.Write` must be synchronous.
type statsdWriter interface {
	Write(data []byte) (n int, err error)
	SetWriteTimeout(time.Duration) error
	Close() error
}

type sender struct {
	transport statsdWriter
	pool      *bufferPool
	queue     chan *statsdBuffer
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
		s.pool.returnBuffer(buffer)
	}
}

func (s *sender) sendLoop() {
	for {
		select {
		case buffer := <-s.queue:
			s.transport.Write(buffer.bytes())
			s.pool.returnBuffer(buffer)
		case <-s.stop:
			return
		}
	}
}

func (s *sender) flush() {
	for {
		select {
		case buffer := <-s.queue:
			s.transport.Write(buffer.bytes())
			s.pool.returnBuffer(buffer)
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

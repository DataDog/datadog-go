package statsd

import (
	"io"

	"go.uber.org/atomic"
)

// senderTelemetry contains telemetry about the health of the sender
type senderTelemetry struct {
	totalPayloadsSent             *atomic.Uint64
	totalPayloadsDroppedQueueFull *atomic.Uint64
	totalPayloadsDroppedWriter    *atomic.Uint64
	totalBytesSent                *atomic.Uint64
	totalBytesDroppedQueueFull    *atomic.Uint64
	totalBytesDroppedWriter       *atomic.Uint64
}

type sender struct {
	transport   io.WriteCloser
	pool        *bufferPool
	queue       chan *statsdBuffer
	telemetry   *senderTelemetry
	stop        chan struct{}
	flushSignal chan struct{}
}

func newSender(transport io.WriteCloser, queueSize int, pool *bufferPool) *sender {
	sender := &sender{
		transport: transport,
		pool:      pool,
		queue:     make(chan *statsdBuffer, queueSize),
		telemetry: &senderTelemetry{
			totalPayloadsSent:             atomic.NewUint64(0),
			totalPayloadsDroppedQueueFull: atomic.NewUint64(0),
			totalPayloadsDroppedWriter:    atomic.NewUint64(0),
			totalBytesSent:                atomic.NewUint64(0),
			totalBytesDroppedQueueFull:    atomic.NewUint64(0),
			totalBytesDroppedWriter:       atomic.NewUint64(0),
		},
		stop:        make(chan struct{}),
		flushSignal: make(chan struct{}),
	}

	go sender.sendLoop()
	return sender
}

func (s *sender) send(buffer *statsdBuffer) {
	select {
	case s.queue <- buffer:
	default:
		s.telemetry.totalPayloadsDroppedQueueFull.Add(1)
		s.telemetry.totalBytesDroppedQueueFull.Add(uint64(len(buffer.bytes())))
		s.pool.returnBuffer(buffer)
	}
}

func (s *sender) write(buffer *statsdBuffer) {
	_, err := s.transport.Write(buffer.bytes())
	if err != nil {
		s.telemetry.totalPayloadsDroppedWriter.Add(1)
		s.telemetry.totalBytesDroppedWriter.Add(uint64(len(buffer.bytes())))
	} else {
		s.telemetry.totalPayloadsSent.Add(1)
		s.telemetry.totalBytesSent.Add(uint64(len(buffer.bytes())))
	}
	s.pool.returnBuffer(buffer)
}

func (s *sender) flushTelemetryMetrics(t *Telemetry) {
	t.TotalPayloadsSent = s.telemetry.totalPayloadsSent.Load()
	t.TotalPayloadsDroppedQueueFull = s.telemetry.totalPayloadsDroppedQueueFull.Load()
	t.TotalPayloadsDroppedWriter = s.telemetry.totalPayloadsDroppedWriter.Load()

	t.TotalBytesSent = s.telemetry.totalBytesSent.Load()
	t.TotalBytesDroppedQueueFull = s.telemetry.totalBytesDroppedQueueFull.Load()
	t.TotalBytesDroppedWriter = s.telemetry.totalBytesDroppedWriter.Load()
}

func (s *sender) sendLoop() {
	defer close(s.stop)
	for {
		select {
		case buffer := <-s.queue:
			s.write(buffer)
		case <-s.stop:
			return
		case <-s.flushSignal:
			// At that point we know that the workers are paused (the statsd client
			// will pause them before calling sender.flush()).
			// So we can fully flush the input queue
			s.flushInputQueue()
			s.flushSignal <- struct{}{}
		}
	}
}

func (s *sender) flushInputQueue() {
	for {
		select {
		case buffer := <-s.queue:
			s.write(buffer)
		default:
			return
		}
	}
}
func (s *sender) flush() {
	s.flushSignal <- struct{}{}
	<-s.flushSignal
}

func (s *sender) close() error {
	s.stop <- struct{}{}
	<-s.stop
	s.flushInputQueue()
	return s.transport.Close()
}

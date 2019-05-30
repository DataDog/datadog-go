package statsd

type bufferFullError string

func (e bufferFullError) Error() string { return string(e) }

const errBufferFull = bufferFullError("statsd buffer is full")

type statsdBuffer struct {
	buffer       []byte
	maxSize      int
	maxElements  int
	elementCount int
}

func newStatsdBuffer(maxSize, maxElements int) *statsdBuffer {
	return &statsdBuffer{
		buffer:      make([]byte, 0, maxSize*2),
		maxSize:     maxSize,
		maxElements: maxElements,
	}
}

func (b *statsdBuffer) writeGauge(namespace string, globalTags []string, name string, value float64, tags []string, rate float64) error {
	if b.elementCount >= b.maxElements {
		return errBufferFull
	}
	b.writeSeparator()
	originalBuffer := b.buffer
	b.buffer = appendGauge(b.buffer, namespace, globalTags, name, value, tags, rate)
	return b.validateNewElement(originalBuffer)
}

func (b *statsdBuffer) writeCount(namespace string, globalTags []string, name string, value int64, tags []string, rate float64) error {
	if b.elementCount >= b.maxElements {
		return errBufferFull
	}
	b.writeSeparator()
	originalBuffer := b.buffer
	b.buffer = appendCount(b.buffer, namespace, globalTags, name, value, tags, rate)
	return b.validateNewElement(originalBuffer)
}

func (b *statsdBuffer) writeHistogram(namespace string, globalTags []string, name string, value float64, tags []string, rate float64) error {
	if b.elementCount >= b.maxElements {
		return errBufferFull
	}
	b.writeSeparator()
	originalBuffer := b.buffer
	b.buffer = appendHistogram(b.buffer, namespace, globalTags, name, value, tags, rate)
	return b.validateNewElement(originalBuffer)
}

func (b *statsdBuffer) writeDistribution(namespace string, globalTags []string, name string, value float64, tags []string, rate float64) error {
	if b.elementCount >= b.maxElements {
		return errBufferFull
	}
	b.writeSeparator()
	originalBuffer := b.buffer
	b.buffer = appendDistribution(b.buffer, namespace, globalTags, name, value, tags, rate)
	return b.validateNewElement(originalBuffer)
}

func (b *statsdBuffer) writeDecrement(namespace string, globalTags []string, name string, tags []string, rate float64) error {
	if b.elementCount >= b.maxElements {
		return errBufferFull
	}
	b.writeSeparator()
	originalBuffer := b.buffer
	b.buffer = appendDecrement(b.buffer, namespace, globalTags, name, tags, rate)
	return b.validateNewElement(originalBuffer)
}

func (b *statsdBuffer) writeIncrement(namespace string, globalTags []string, name string, tags []string, rate float64) error {
	if b.elementCount >= b.maxElements {
		return errBufferFull
	}
	b.writeSeparator()
	originalBuffer := b.buffer
	b.buffer = appendIncrement(b.buffer, namespace, globalTags, name, tags, rate)
	return b.validateNewElement(originalBuffer)
}

func (b *statsdBuffer) writeSet(namespace string, globalTags []string, name string, value string, tags []string, rate float64) error {
	if b.elementCount >= b.maxElements {
		return errBufferFull
	}
	b.writeSeparator()
	originalBuffer := b.buffer
	b.buffer = appendSet(b.buffer, namespace, globalTags, name, value, tags, rate)
	return b.validateNewElement(originalBuffer)
}

func (b *statsdBuffer) writeTiming(namespace string, globalTags []string, name string, value float64, tags []string, rate float64) error {
	if b.elementCount >= b.maxElements {
		return errBufferFull
	}
	b.writeSeparator()
	originalBuffer := b.buffer
	b.buffer = appendTiming(b.buffer, namespace, globalTags, name, value, tags, rate)
	return b.validateNewElement(originalBuffer)
}

func (b *statsdBuffer) writeEvent(event Event, globalTags []string) error {
	if b.elementCount >= b.maxElements {
		return errBufferFull
	}
	b.writeSeparator()
	originalBuffer := b.buffer
	b.buffer = appendEvent(b.buffer, event, globalTags)
	return b.validateNewElement(originalBuffer)
}

func (b *statsdBuffer) writeServiceCheck(serviceCheck ServiceCheck, globalTags []string) error {
	if b.elementCount >= b.maxElements {
		return errBufferFull
	}
	b.writeSeparator()
	originalBuffer := b.buffer
	b.buffer = appendServiceCheck(b.buffer, serviceCheck, globalTags)
	return b.validateNewElement(originalBuffer)
}

func (b *statsdBuffer) validateNewElement(originalBuffer []byte) error {
	if len(b.buffer) > b.maxSize {
		b.buffer = originalBuffer
		return errBufferFull
	}
	b.elementCount++
	return nil
}

func (b *statsdBuffer) writeSeparator() {
	if b.elementCount != 0 {
		b.buffer = appendSeparator(b.buffer)
	}
}

func (b *statsdBuffer) bytes() []byte {
	return b.buffer
}
